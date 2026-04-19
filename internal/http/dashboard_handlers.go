package httpui

import (
	"net/http"
	"sort"

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
	// The dashboard renders its own title block with assessment identity;
	// the layout's duplicate header title would just compete with it.
	data.BaseData.HideHeaderTitle = true

	if len(assessments) == 0 {
		return s.render(c, "dashboard", data)
	}

	selected := selectedAssessment(assessments, c.QueryParam("assessment_id"))

	dashboard, err := s.services.Dashboard.Get(c.Request().Context(), selected.ID)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	// Sort groups worst-first so the dashboard answers "where is this assessment bleeding"
	// at a glance. Groups with equal completion fall back to total items descending, then code.
	sort.SliceStable(dashboard.ByGroup, func(i, j int) bool {
		a, b := dashboard.ByGroup[i], dashboard.ByGroup[j]
		ap := percentage(a.CompletedItems, a.TotalItems)
		bp := percentage(b.CompletedItems, b.TotalItems)
		if ap != bp {
			return ap < bp
		}
		if a.TotalItems != b.TotalItems {
			return a.TotalItems > b.TotalItems
		}
		return a.GroupCode < b.GroupCode
	})

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
