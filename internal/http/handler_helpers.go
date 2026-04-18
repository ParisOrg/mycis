package httpui

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"

	"mycis/internal/db"
	"mycis/internal/service"
	"mycis/internal/textutil"
)

func (s *Server) baseData(c *echo.Context, title, nav string) BaseData {
	return BaseData{
		Title:       title,
		AppName:     s.cfg.AppName,
		ActiveNav:   nav,
		CurrentUser: s.currentUser(c),
		CSRFToken:   s.csrfToken(c),
		Flashes:     s.readFlashes(c),
		Query:       c.QueryParams(),
	}
}

func (s *Server) redirectWithFlash(c *echo.Context, destination, kind, message string) error {
	session, err := s.session(c)
	if err == nil {
		session.AddFlash(kind + "|" + message)
		_ = session.Save(c.Request(), c.Response())
	}
	return c.Redirect(http.StatusSeeOther, destination)
}

func (s *Server) readFlashes(c *echo.Context) []Flash {
	session, err := s.session(c)
	if err != nil {
		return nil
	}

	raw := session.Flashes()
	_ = session.Save(c.Request(), c.Response())

	flashes := make([]Flash, 0, len(raw))
	for _, item := range raw {
		text, ok := item.(string)
		if !ok {
			continue
		}
		parts := strings.SplitN(text, "|", 2)
		if len(parts) != 2 {
			continue
		}
		flashes = append(flashes, Flash{
			Kind:    parts[0],
			Message: parts[1],
		})
	}
	return flashes
}

func readItemFilters(c *echo.Context) service.AssessmentItemFilters {
	query := c.QueryParams()
	filters := service.AssessmentItemFilters{
		GroupCode:      textutil.TrimPtr(query.Get("group")),
		Tag:            textutil.TrimPtr(query.Get("tag")),
		Status:         textutil.TrimPtr(query.Get("status")),
		OwnerUserID:    textutil.TrimPtr(query.Get("owner")),
		ReviewerUserID: textutil.TrimPtr(query.Get("reviewer")),
	}
	if query.Get("overdue") == "1" {
		value := true
		filters.Overdue = &value
	}
	return filters
}

func hasFilters(filters service.AssessmentItemFilters) bool {
	return filters.GroupCode != nil ||
		filters.Tag != nil ||
		filters.Status != nil ||
		filters.OwnerUserID != nil ||
		filters.ReviewerUserID != nil ||
		filters.Overdue != nil
}

func selectedByID[T any](items []T, selectedID string, getID func(T) string) *T {
	if len(items) == 0 {
		return nil
	}

	selectedIndex := 0
	if selectedID != "" {
		for i := range items {
			if getID(items[i]) == selectedID {
				selectedIndex = i
				break
			}
		}
	}
	return &items[selectedIndex]
}

func selectedAssessment(assessments []db.ListAssessmentsRow, selectedID string) *db.ListAssessmentsRow {
	return selectedByID(assessments, selectedID, func(a db.ListAssessmentsRow) string {
		return a.ID.String()
	})
}

func selectedFramework(frameworks []db.ListFrameworksWithCountsRow, selectedID string) *db.ListFrameworksWithCountsRow {
	return selectedByID(frameworks, selectedID, func(f db.ListFrameworksWithCountsRow) string {
		return f.ID.String()
	})
}

func (s *Server) readFormOrRedirect(c *echo.Context, destination, message string) (url.Values, error) {
	form, err := c.FormValues()
	if err != nil {
		return nil, s.redirectWithFlash(c, destination, "error", message)
	}
	return form, nil
}

func (s *Server) requiredUUID(c *echo.Context, raw, destination, message string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, s.redirectWithFlash(c, destination, "error", message)
	}
	return parsed, nil
}

func (s *Server) optionalUUID(raw string) *uuid.UUID {
	if raw == "" {
		return nil
	}
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return nil
	}
	return &parsed
}

func (s *Server) requiredDate(c *echo.Context, raw, destination, message string) (time.Time, error) {
	parsed, err := textutil.ParseDateOnly(raw)
	if err != nil {
		return time.Time{}, s.redirectWithFlash(c, destination, "error", message)
	}
	return parsed, nil
}

func (s *Server) optionalDate(raw string) *time.Time {
	if raw == "" {
		return nil
	}
	parsed, err := textutil.ParseDateOnly(raw)
	if err != nil {
		return nil
	}
	return &parsed
}

func (s *Server) optionalInt32(raw string) *int32 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parsed, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return nil
	}
	value := int32(parsed)
	return &value
}

func renderServiceError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		return c.String(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	case errors.Is(err, service.ErrNotFound):
		return c.String(http.StatusNotFound, http.StatusText(http.StatusNotFound))
	case errors.Is(err, service.ErrForbidden):
		return c.String(http.StatusForbidden, http.StatusText(http.StatusForbidden))
	default:
		return c.String(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}
}
