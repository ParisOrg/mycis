package httpui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v5"

	"mycis/internal/config"
	"mycis/internal/db"
	"mycis/internal/service"
)

type httpIntegrationHarness struct {
	ctx      context.Context
	services *service.Services
}

func TestUserUpdatePostInvalidPasswordKeepsEditDialogState(t *testing.T) {
	h := newHTTPIntegrationHarness(t)
	s := h.newServer(t)

	admin := h.createUser(t, "Admin User", "admin@example.com", db.UserRoleAdmin)
	user := h.createUser(t, "Original User", "original@example.com", db.UserRoleEditor)

	form := url.Values{
		"user_id":  []string{user.ID.String()},
		"name":     []string{"Updated User"},
		"email":    []string{user.Email},
		"role":     []string{string(db.UserRoleAssessmentManager)},
		"password": []string{"short"},
	}
	redirectRec := submitUserUpdate(t, s, admin, form)
	if redirectRec.Code != http.StatusSeeOther {
		t.Fatalf("unexpected redirect status: got %d want %d", redirectRec.Code, http.StatusSeeOther)
	}
	if location := redirectRec.Header().Get(echo.HeaderLocation); location != "/admin/users" {
		t.Fatalf("unexpected redirect location: got %q want %q", location, "/admin/users")
	}

	body := renderUsersPage(t, s, admin, findSessionCookie(t, redirectRec.Result().Cookies()))
	for _, want := range []string{
		`id="edit-user-dialog" data-dialog-auto-open="true"`,
		`value="Updated User"`,
		`value="original@example.com"`,
		`password must be at least 12 characters`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response body to contain %q", want)
		}
	}
}

func TestUserUpdatePostPasswordResetShowsFlashAndUpdatesPassword(t *testing.T) {
	h := newHTTPIntegrationHarness(t)
	s := h.newServer(t)

	admin := h.createUser(t, "Admin User", "admin@example.com", db.UserRoleAdmin)
	user := h.createUser(t, "Original User", "original@example.com", db.UserRoleEditor)

	form := url.Values{
		"user_id":  []string{user.ID.String()},
		"name":     []string{"Updated User"},
		"email":    []string{user.Email},
		"role":     []string{string(db.UserRoleAssessmentManager)},
		"password": []string{"new-password-12345"},
	}
	redirectRec := submitUserUpdate(t, s, admin, form)
	if redirectRec.Code != http.StatusSeeOther {
		t.Fatalf("unexpected redirect status: got %d want %d", redirectRec.Code, http.StatusSeeOther)
	}
	if location := redirectRec.Header().Get(echo.HeaderLocation); location != "/admin/users" {
		t.Fatalf("unexpected redirect location: got %q want %q", location, "/admin/users")
	}

	body := renderUsersPage(t, s, admin, findSessionCookie(t, redirectRec.Result().Cookies()))
	if !strings.Contains(body, `Updated original@example.com and reset the password.`) {
		t.Fatal("expected success flash describing the password reset")
	}

	reloaded, err := h.services.Auth.GetUserByID(h.ctx, user.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Name != "Updated User" {
		t.Fatalf("unexpected updated name: got %q want %q", reloaded.Name, "Updated User")
	}
	if reloaded.Role != db.UserRoleAssessmentManager {
		t.Fatalf("unexpected updated role: got %q want %q", reloaded.Role, db.UserRoleAssessmentManager)
	}
	if !reloaded.MustChangePassword {
		t.Fatal("expected password reset to require a password change")
	}

	authenticated, err := h.services.Auth.Authenticate(h.ctx, user.Email, "new-password-12345")
	if err != nil {
		t.Fatal(err)
	}
	if authenticated.ID != user.ID {
		t.Fatalf("unexpected authenticated user: got %s want %s", authenticated.ID, user.ID)
	}
}

func newHTTPIntegrationHarness(t *testing.T) *httpIntegrationHarness {
	t.Helper()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	rootDir := httpRepoRoot(t)
	withHTTPWorkingDirectory(t, rootDir)

	testDatabaseURL := createHTTPIntegrationDatabase(t, ctx, databaseURL)
	runHTTPMigrationsForTest(t, testDatabaseURL, rootDir)

	pool, err := pgxpool.New(ctx, testDatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	return &httpIntegrationHarness{
		ctx:      ctx,
		services: service.New(pool),
	}
}

func (h *httpIntegrationHarness) newServer(t *testing.T) *Server {
	t.Helper()

	server, err := NewServer(config.Config{
		AppName:    "Controls Tracker",
		SessionKey: "0123456789abcdef0123456789abcdef",
	}, h.services)
	if err != nil {
		t.Fatal(err)
	}
	return server
}

func (h *httpIntegrationHarness) createUser(t *testing.T, name, email string, role db.UserRole) db.User {
	t.Helper()

	user, err := h.services.Auth.CreateUserWithPassword(h.ctx, name, email, "password-12345", role, false)
	if err != nil {
		t.Fatal(err)
	}
	return user
}

func submitUserUpdate(t *testing.T, s *Server, currentUser db.User, form url.Values) *httptest.ResponseRecorder {
	t.Helper()

	e := echo.New()
	body := strings.NewReader(form.Encode())
	req := httptest.NewRequest(http.MethodPost, "/admin/users/edit", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(userKey, &currentUser)

	if err := s.userUpdatePost(c); err != nil {
		t.Fatal(err)
	}

	return rec
}

func renderUsersPage(t *testing.T, s *Server, currentUser db.User, sessionCookie *http.Cookie) string {
	t.Helper()

	e := echo.New()
	e.Renderer = templateRenderer{templates: s.templates}

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(userKey, &currentUser)

	if err := s.usersPage(c); err != nil {
		t.Fatal(err)
	}

	return rec.Body.String()
}

func httpRepoRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine caller path")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func withHTTPWorkingDirectory(t *testing.T, dir string) {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Errorf("restore cwd: %v", err)
		}
	})
}

func createHTTPIntegrationDatabase(t *testing.T, ctx context.Context, baseDatabaseURL string) string {
	t.Helper()

	adminPool, err := pgxpool.New(ctx, baseDatabaseURL)
	if err != nil {
		t.Fatal(err)
	}

	databaseName := "mycis_http_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
	if _, err := adminPool.Exec(ctx, `CREATE DATABASE "`+databaseName+`"`); err != nil {
		adminPool.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := adminPool.Exec(context.Background(), `DROP DATABASE IF EXISTS "`+databaseName+`" WITH (FORCE)`); err != nil {
			t.Errorf("drop http test database: %v", err)
		}
		adminPool.Close()
	})

	parsedURL, err := url.Parse(baseDatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	parsedURL.Path = "/" + databaseName
	return parsedURL.String()
}

func runHTTPMigrationsForTest(t *testing.T, databaseURL, rootDir string) {
	t.Helper()

	sourceURL := "file://" + filepath.Join(rootDir, "db", "migrations")
	m, err := migrate.New(sourceURL, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = m.Close()
	}()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatal(err)
	}
}
