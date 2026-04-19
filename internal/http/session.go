package httpui

import (
	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v5"
)

const sessionContextKey = "session"

func (s *Server) session(c *echo.Context) (*sessions.Session, error) {
	if cached, ok := c.Get(sessionContextKey).(*sessions.Session); ok && cached != nil {
		return cached, nil
	}

	session, err := s.store.Get(c.Request(), sessionName)
	if session != nil {
		c.Set(sessionContextKey, session)
		return session, nil
	}
	return nil, err
}
