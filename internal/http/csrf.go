package httpui

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
)

const (
	csrfTokenSessionKey = "csrf_token"
	csrfTokenFieldName  = "csrf_token"
)

func (s *Server) protectCSRF(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		switch c.Request().Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
			return next(c)
		}

		session, err := s.session(c)
		if err != nil {
			return c.String(http.StatusForbidden, http.StatusText(http.StatusForbidden))
		}

		expected, _ := session.Values[csrfTokenSessionKey].(string)
		if expected == "" {
			return c.String(http.StatusForbidden, http.StatusText(http.StatusForbidden))
		}

		provided := strings.TrimSpace(c.Request().Header.Get("X-CSRF-Token"))
		if provided == "" {
			if err := c.Request().ParseForm(); err != nil {
				return c.String(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
			}
			provided = strings.TrimSpace(c.Request().PostForm.Get(csrfTokenFieldName))
		}

		if !validCSRFToken(expected, provided) {
			return c.String(http.StatusForbidden, http.StatusText(http.StatusForbidden))
		}

		return next(c)
	}
}

func (s *Server) csrfToken(c *echo.Context) string {
	session, err := s.session(c)
	if err != nil {
		return ""
	}

	if token, _ := session.Values[csrfTokenSessionKey].(string); token != "" {
		return token
	}

	token, err := newCSRFToken()
	if err != nil {
		return ""
	}

	session.Values[csrfTokenSessionKey] = token
	_ = session.Save(c.Request(), c.Response())
	return token
}

func newCSRFToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func validCSRFToken(expected, provided string) bool {
	if expected == "" || provided == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) == 1
}
