package httpui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestLoginFailureRendersInlineErrorState(t *testing.T) {
	s := newTestServer(t)

	e := echo.New()
	e.Renderer = templateRenderer{templates: s.templates}

	redirectReq := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(""))
	redirectRec := httptest.NewRecorder()
	redirectCtx := e.NewContext(redirectReq, redirectRec)

	if err := s.redirectLoginFailure(redirectCtx, "admin1@example.com", "Email or password is wrong."); err != nil {
		t.Fatal(err)
	}
	if redirectRec.Code != http.StatusSeeOther {
		t.Fatalf("unexpected redirect status: got %d want %d", redirectRec.Code, http.StatusSeeOther)
	}
	if location := redirectRec.Header().Get(echo.HeaderLocation); location != "/login" {
		t.Fatalf("unexpected redirect location: got %q want %q", location, "/login")
	}

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	req.AddCookie(findSessionCookie(t, redirectRec.Result().Cookies()))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.loginPage(c); err != nil {
		t.Fatal(err)
	}

	body := rec.Body.String()
	for _, want := range []string{
		`id="login-error"`,
		`Sign-in failed`,
		`Email or password is wrong.`,
		`value="admin1@example.com"`,
		`aria-invalid="true"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response body to contain %q", want)
		}
	}
}

func TestLoginPageConsumesFailureStateAfterRender(t *testing.T) {
	s := newTestServer(t)

	e := echo.New()
	e.Renderer = templateRenderer{templates: s.templates}

	redirectReq := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(""))
	redirectRec := httptest.NewRecorder()
	redirectCtx := e.NewContext(redirectReq, redirectRec)

	if err := s.redirectLoginFailure(redirectCtx, "admin1@example.com", "Email or password is wrong."); err != nil {
		t.Fatal(err)
	}

	firstReq := httptest.NewRequest(http.MethodGet, "/login", nil)
	firstReq.AddCookie(findSessionCookie(t, redirectRec.Result().Cookies()))
	firstRec := httptest.NewRecorder()
	firstCtx := e.NewContext(firstReq, firstRec)

	if err := s.loginPage(firstCtx); err != nil {
		t.Fatal(err)
	}

	secondReq := httptest.NewRequest(http.MethodGet, "/login", nil)
	secondReq.AddCookie(findSessionCookie(t, firstRec.Result().Cookies()))
	secondRec := httptest.NewRecorder()
	secondCtx := e.NewContext(secondReq, secondRec)

	if err := s.loginPage(secondCtx); err != nil {
		t.Fatal(err)
	}

	body := secondRec.Body.String()
	for _, unwanted := range []string{
		`id="login-error"`,
		`Email or password is wrong.`,
		`value="admin1@example.com"`,
	} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("did not expect response body to contain %q", unwanted)
		}
	}
}
