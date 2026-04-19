package httpui

import (
	"fmt"
	"net/http"

	"mycis/internal/db"

	"github.com/labstack/echo/v5"
)

func (s *Server) usersPage(c *echo.Context) error {
	if !s.requireUserManager(c) {
		return nil
	}

	users, err := s.services.Auth.ListUsers(c.Request().Context())
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	return s.render(c, "users", UsersPageData{
		BaseData: s.baseData(c, "Users", "users"),
		Users:    users,
		Roles:    db.AllUserRoles(),
	})
}

func (s *Server) userCreatePost(c *echo.Context) error {
	if !s.requireUserManager(c) {
		return nil
	}

	form, err := s.readFormOrRedirect(c, "/admin/users", "Could not read the user form.")
	if err != nil {
		return err
	}

	role, err := db.ParseUserRole(form.Get("role"))
	if err != nil {
		return s.redirectWithFlash(c, "/admin/users", "error", err.Error())
	}

	user, password, err := s.services.Auth.CreateUser(c.Request().Context(), form.Get("name"), form.Get("email"), role)
	if err != nil {
		return s.redirectWithFlash(c, "/admin/users", "error", err.Error())
	}
	return s.redirectWithFlash(c, "/admin/users", "success", fmt.Sprintf("Created %s. Temporary password: %s", user.Email, password))
}
