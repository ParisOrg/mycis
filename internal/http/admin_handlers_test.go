package httpui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"

	"mycis/internal/db"
)

func TestUsersTemplateShowsEditButtonForAdmins(t *testing.T) {
	s := newTestServer(t)

	e := echo.New()
	e.Renderer = templateRenderer{templates: s.templates}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	c := e.NewContext(req, rec)

	data := UsersPageData{
		BaseData: BaseData{
			Title:       "Users",
			AppName:     "Controls Tracker",
			CurrentUser: &db.User{Name: "Admin User", Role: db.UserRoleAdmin},
		},
		Users: []db.User{{ID: uuid.New(), Name: "Admin User", Email: "admin@example.com", Role: db.UserRoleAdmin}},
		Roles: db.AllUserRoles(),
	}

	if err := s.render(c, "users", data); err != nil {
		t.Fatal(err)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "data-edit-user-trigger") {
		t.Fatal("expected admin users page to render edit button")
	}
}

func TestUsersTemplateHidesEditButtonForNonAdmins(t *testing.T) {
	s := newTestServer(t)

	e := echo.New()
	e.Renderer = templateRenderer{templates: s.templates}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	c := e.NewContext(req, rec)

	data := UsersPageData{
		BaseData: BaseData{
			Title:       "Users",
			AppName:     "Controls Tracker",
			CurrentUser: &db.User{Name: "Editor User", Role: db.UserRoleEditor},
		},
		Users: []db.User{{ID: uuid.New(), Name: "Editor User", Email: "editor@example.com", Role: db.UserRoleEditor}},
		Roles: db.AllUserRoles(),
	}

	if err := s.render(c, "users", data); err != nil {
		t.Fatal(err)
	}

	body := rec.Body.String()
	if strings.Contains(body, "data-edit-user-trigger") {
		t.Fatal("did not expect non-admin users page to render edit button")
	}
}

func TestPopEditUserFormStateSavesSessionOnce(t *testing.T) {
	s := newTestServer(t)

	e := echo.New()

	seedReq := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	seedRec := httptest.NewRecorder()
	seedCtx := e.NewContext(seedReq, seedRec)

	session, err := s.session(seedCtx)
	if err != nil {
		t.Fatal(err)
	}
	session.Values[editUserOpenSessionKey] = "1"
	session.Values[editUserIDSessionKey] = "user-123"
	session.Values[editUserNameSessionKey] = "Edited User"
	session.Values[editUserEmailSessionKey] = "edited@example.com"
	session.Values[editUserRoleSessionKey] = string(db.UserRoleAssessmentManager)
	if err := session.Save(seedReq, seedRec); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.AddCookie(findSessionCookie(t, seedRec.Result().Cookies()))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	form, open := s.popEditUserFormState(c)
	if !open {
		t.Fatal("expected edit dialog state to open")
	}
	if form != (UserEditFormData{
		ID:    "user-123",
		Name:  "Edited User",
		Email: "edited@example.com",
		Role:  string(db.UserRoleAssessmentManager),
	}) {
		t.Fatalf("unexpected form state: %+v", form)
	}

	if got := len(rec.Header().Values("Set-Cookie")); got != 1 {
		t.Fatalf("expected one session save when popping edit form state, got %d", got)
	}
}
