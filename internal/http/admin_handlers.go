package httpui

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"
)

func (s *Server) usersPage(c *echo.Context) error {
	if !s.requireAdmin(c) {
		return nil
	}

	users, err := s.services.Auth.ListUsers(c.Request().Context())
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	return s.render(c, "users", UsersPageData{
		BaseData: s.baseData(c, "Users", "users"),
		Users:    users,
	})
}

func (s *Server) userCreatePost(c *echo.Context) error {
	if !s.requireAdmin(c) {
		return nil
	}

	form, err := s.readFormOrRedirect(c, "/admin/users", "Could not read the user form.")
	if err != nil {
		return err
	}

	isAdmin := form.Get("is_admin") == "on"
	user, password, err := s.services.Auth.CreateUser(c.Request().Context(), form.Get("name"), form.Get("email"), isAdmin)
	if err != nil {
		return s.redirectWithFlash(c, "/admin/users", "error", err.Error())
	}
	return s.redirectWithFlash(c, "/admin/users", "success", fmt.Sprintf("Created %s. Temporary password: %s", user.Email, password))
}
