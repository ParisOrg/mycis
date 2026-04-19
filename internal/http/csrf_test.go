package httpui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"mycis/internal/config"
)

var csrfFieldPattern = regexp.MustCompile(`name="csrf_token" value="([^"]+)"`)

func TestLoginPageIncludesCSRFToken(t *testing.T) {
	s := newTestServer(t)

	e := echo.New()
	e.Renderer = templateRenderer{templates: s.templates}

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.loginPage(c); err != nil {
		t.Fatal(err)
	}

	token := extractCSRFToken(t, rec.Body.String())
	if token == "" {
		t.Fatal("expected csrf token")
	}
	if cookie := findSessionCookie(t, rec.Result().Cookies()); cookie == nil {
		t.Fatal("expected session cookie")
	}
}

func TestLoginPageRecoversFromUndecodableSessionCookie(t *testing.T) {
	s := newTestServer(t)

	e := echo.New()
	e.Renderer = templateRenderer{templates: s.templates}

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	req.AddCookie(&http.Cookie{Name: sessionName, Value: "not-a-valid-session"})
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.loginPage(c); err != nil {
		t.Fatal(err)
	}

	if token := extractCSRFToken(t, rec.Body.String()); token == "" {
		t.Fatal("expected csrf token")
	}
	if cookie := findSessionCookie(t, rec.Result().Cookies()); cookie == nil {
		t.Fatal("expected replacement session cookie")
	}
}

func TestProtectCSRFRejectsMissingToken(t *testing.T) {
	s := newTestServer(t)
	cookie, _ := issueLoginPage(t, s)

	form := url.Values{
		"email":    {"demo@example.com"},
		"password": {"not-secret"},
	}

	rec := submitProtectedForm(t, s, cookie, form, func(c *echo.Context) error {
		t.Fatal("expected middleware to stop request")
		return nil
	})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("unexpected status: got %d want %d", rec.Code, http.StatusForbidden)
	}
}

func TestProtectCSRFRejectsInvalidToken(t *testing.T) {
	s := newTestServer(t)
	cookie, _ := issueLoginPage(t, s)

	form := url.Values{
		"email":      {"demo@example.com"},
		"password":   {"not-secret"},
		"csrf_token": {"wrong-token"},
	}

	rec := submitProtectedForm(t, s, cookie, form, func(c *echo.Context) error {
		t.Fatal("expected middleware to stop request")
		return nil
	})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("unexpected status: got %d want %d", rec.Code, http.StatusForbidden)
	}
}

func TestProtectCSRFAcceptsValidToken(t *testing.T) {
	s := newTestServer(t)
	cookie, token := issueLoginPage(t, s)

	form := url.Values{
		"email":      {"demo@example.com"},
		"password":   {"not-secret"},
		"csrf_token": {token},
	}

	rec := submitProtectedForm(t, s, cookie, form, func(c *echo.Context) error {
		values, err := c.FormValues()
		if err != nil {
			t.Fatal(err)
		}
		if values.Get("email") != "demo@example.com" {
			t.Fatalf("unexpected email: %q", values.Get("email"))
		}
		return c.NoContent(http.StatusNoContent)
	})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: got %d want %d", rec.Code, http.StatusNoContent)
	}
}

func TestRouterLoginPostRejectsMissingCSRFToken(t *testing.T) {
	s := newTestServer(t)

	form := url.Values{
		"email":    {"demo@example.com"},
		"password": {"not-secret"},
	}

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	rec := httptest.NewRecorder()

	s.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("unexpected status: got %d want %d", rec.Code, http.StatusForbidden)
	}
}

func TestProtectedPostWithInvalidSessionRedirectsToLogin(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(&http.Cookie{Name: sessionName, Value: "not-a-valid-session"})
	rec := httptest.NewRecorder()

	s.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("unexpected status: got %d want %d", rec.Code, http.StatusSeeOther)
	}
	if location := rec.Header().Get(echo.HeaderLocation); location != "/login" {
		t.Fatalf("unexpected redirect location: %q", location)
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	t.Chdir(filepath.Join("..", ".."))

	server, err := NewServer(config.Config{
		AppName:    "Controls Tracker",
		SessionKey: "0123456789abcdef0123456789abcdef",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	return server
}

func issueLoginPage(t *testing.T, s *Server) (*http.Cookie, string) {
	t.Helper()

	e := echo.New()
	e.Renderer = templateRenderer{templates: s.templates}

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.loginPage(c); err != nil {
		t.Fatal(err)
	}

	return findSessionCookie(t, rec.Result().Cookies()), extractCSRFToken(t, rec.Body.String())
}

func submitProtectedForm(t *testing.T, s *Server, cookie *http.Cookie, form url.Values, next echo.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()

	e := echo.New()
	body := strings.NewReader(form.Encode())
	req := httptest.NewRequest(http.MethodPost, "/login", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.protectCSRF(next)(c); err != nil {
		t.Fatal(err)
	}

	return rec
}

func extractCSRFToken(t *testing.T, body string) string {
	t.Helper()

	match := csrfFieldPattern.FindStringSubmatch(body)
	if len(match) != 2 {
		t.Fatalf("csrf token field not found in body: %s", body)
	}
	return match[1]
}

func findSessionCookie(t *testing.T, cookies []*http.Cookie) *http.Cookie {
	t.Helper()

	for i := len(cookies) - 1; i >= 0; i-- {
		if cookies[i].Name == sessionName {
			return cookies[i]
		}
	}
	t.Fatal("session cookie not found")
	return nil
}
