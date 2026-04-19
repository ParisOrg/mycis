package httpui

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

func (s *Server) dashboardPage(c *echo.Context) error {
	assessments, err := s.services.Assessments.ListAssessments(c.Request().Context())
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	data := DashboardPageData{
		BaseData:    s.baseData(c, "Dashboard", "dashboard"),
		Assessments: assessments,
	}

	if len(assessments) == 0 {
		return s.render(c, "dashboard", data)
	}

	selected := selectedAssessment(assessments, c.QueryParam("assessment_id"))

	dashboard, err := s.services.Dashboard.Get(c.Request().Context(), selected.ID)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	data.SelectedAssessment = selected
	data.Dashboard = &dashboard
	return s.render(c, "dashboard", data)
}

func (s *Server) frameworksPage(c *echo.Context) error {
	frameworks, err := s.services.Frameworks.ListFrameworks(c.Request().Context())
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	data := FrameworksPageData{
		BaseData:   s.baseData(c, "Framework Library", "frameworks"),
		Frameworks: frameworks,
	}

	if len(frameworks) == 0 {
		return s.render(c, "frameworks", data)
	}

	selected := selectedFramework(frameworks, c.QueryParam("framework_id"))

	groups, err := s.services.Frameworks.ListGroupsByFramework(c.Request().Context(), selected.ID.String())
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	data.SelectedFramework = selected
	data.Groups = groups
	return s.render(c, "frameworks", data)
}

func (s *Server) assessmentsPage(c *echo.Context) error {
	assessments, err := s.services.Assessments.ListAssessments(c.Request().Context())
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	return s.render(c, "assessments", AssessmentsPageData{
		BaseData:    s.baseData(c, "Assessments", "assessments"),
		Assessments: assessments,
	})
}
