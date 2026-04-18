package httpui

import (
	"net/http"
	"net/url"

	"github.com/labstack/echo/v5"
)

func (s *Server) handleRoot(c *echo.Context) error {
	if s.currentUser(c) != nil {
		return c.Redirect(http.StatusSeeOther, "/dashboard")
	}
	return c.Redirect(http.StatusSeeOther, "/login")
}

func (s *Server) loginPage(c *echo.Context) error {
	if s.currentUser(c) != nil {
		return c.Redirect(http.StatusSeeOther, "/dashboard")
	}
	return s.render(c, "login", LoginPageData{
		BaseData: s.baseData(c, "Sign In", ""),
		Email:    c.QueryParam("email"),
	})
}

func (s *Server) loginPost(c *echo.Context) error {
	form, err := c.FormValues()
	if err != nil {
		return s.redirectWithFlash(c, "/login", "error", "Could not read the sign-in form.")
	}

	email := form.Get("email")
	password := form.Get("password")
	user, err := s.services.Auth.Authenticate(c.Request().Context(), email, password)
	if err != nil {
		return s.redirectWithFlash(c, "/login?email="+url.QueryEscape(email), "error", "Email or password is wrong.")
	}

	session, err := s.session(c)
	if err == nil {
		session.Values["user_id"] = user.ID.String()
		_ = session.Save(c.Request(), c.Response())
	}

	if user.MustChangePassword {
		return c.Redirect(http.StatusSeeOther, "/change-password")
	}

	return c.Redirect(http.StatusSeeOther, "/dashboard")
}

func (s *Server) logoutPost(c *echo.Context) error {
	session, err := s.session(c)
	if err == nil {
		session.Options.MaxAge = -1
		_ = session.Save(c.Request(), c.Response())
	}
	return c.Redirect(http.StatusSeeOther, "/login")
}

func (s *Server) changePasswordPage(c *echo.Context) error {
	return s.render(c, "change_password", ChangePasswordPageData{
		BaseData: s.baseData(c, "Change Password", ""),
	})
}

func (s *Server) changePasswordPost(c *echo.Context) error {
	user := s.currentUser(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	form, err := c.FormValues()
	if err != nil {
		return s.redirectWithFlash(c, "/change-password", "error", "Could not read the password form.")
	}

	password := form.Get("password")
	confirm := form.Get("confirm_password")
	if password != confirm {
		return s.redirectWithFlash(c, "/change-password", "error", "The two passwords do not match.")
	}
	if err := s.services.Auth.ChangePassword(c.Request().Context(), user.ID.String(), password); err != nil {
		return s.redirectWithFlash(c, "/change-password", "error", err.Error())
	}
	return s.redirectWithFlash(c, "/dashboard", "success", "Password updated.")
}
