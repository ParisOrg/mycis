package httpui

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
)

const loginEmailSessionKey = "login_email"

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
	email := strings.TrimSpace(c.QueryParam("email"))
	if email == "" {
		email = s.popSessionString(c, loginEmailSessionKey)
	}
	base := s.baseData(c, "Sign In", "")
	return s.render(c, "login", LoginPageData{
		BaseData:     base,
		Email:        email,
		ErrorMessage: firstFlashMessage(base.Flashes, "error"),
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
		return s.redirectLoginFailure(c, email, "Email or password is wrong.")
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

func firstFlashMessage(flashes []Flash, kind string) string {
	for _, flash := range flashes {
		if flash.Kind == kind {
			return flash.Message
		}
	}
	return ""
}

func (s *Server) redirectLoginFailure(c *echo.Context, email, message string) error {
	session, err := s.session(c)
	if err == nil {
		email = strings.TrimSpace(email)
		if email == "" {
			delete(session.Values, loginEmailSessionKey)
		} else {
			session.Values[loginEmailSessionKey] = email
		}
		session.AddFlash("error|" + message)
		_ = session.Save(c.Request(), c.Response())
	}
	return c.Redirect(http.StatusSeeOther, "/login")
}

func (s *Server) popSessionString(c *echo.Context, key string) string {
	session, err := s.session(c)
	if err != nil {
		return ""
	}

	raw, ok := session.Values[key]
	if !ok {
		return ""
	}

	delete(session.Values, key)
	_ = session.Save(c.Request(), c.Response())

	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}
