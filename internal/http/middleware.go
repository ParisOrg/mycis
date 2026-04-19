package httpui

import (
	"net/http"

	"mycis/internal/db"

	"github.com/labstack/echo/v5"
)

const userKey = "current_user"

func (s *Server) withCurrentUser(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		session, err := s.session(c)
		if err == nil {
			if raw, ok := session.Values["user_id"].(string); ok && raw != "" {
				user, err := s.services.Auth.GetUserByID(c.Request().Context(), raw)
				if err == nil {
					c.Set(userKey, &user)
				}
			}
		}
		return next(c)
	}
}

func (s *Server) requireAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if s.currentUser(c) == nil {
			return s.redirectWithFlash(c, "/login", "error", "Sign in to continue.")
		}
		return next(c)
	}
}

func (s *Server) requireAssessmentManager(c *echo.Context) bool {
	user := s.currentUser(c)
	if user == nil || !user.CanManageAssessments() {
		_ = s.redirectWithFlash(c, "/dashboard", "error", "Assessment manager access is required.")
		return false
	}
	return true
}

func (s *Server) requireUserManager(c *echo.Context) bool {
	user := s.currentUser(c)
	if user == nil || !user.CanManageUsers() {
		_ = s.redirectWithFlash(c, "/dashboard", "error", "Admin access is required.")
		return false
	}
	return true
}

func (s *Server) enforcePasswordChange(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		user := s.currentUser(c)
		if user != nil && user.MustChangePassword && c.Request().URL.Path != "/change-password" && c.Request().URL.Path != "/logout" {
			return c.Redirect(http.StatusSeeOther, "/change-password")
		}
		return next(c)
	}
}

func (s *Server) currentUser(c *echo.Context) *db.User {
	user, _ := c.Get(userKey).(*db.User)
	return user
}
