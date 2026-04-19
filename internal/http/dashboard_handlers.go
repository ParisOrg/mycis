package httpui

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/labstack/echo/v5"

	"mycis/internal/service"
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

	sort.SliceStable(dashboard.ByOwner, func(i, j int) bool {
		a, b := dashboard.ByOwner[i], dashboard.ByOwner[j]
		if a.OverdueItems != b.OverdueItems {
			return a.OverdueItems > b.OverdueItems
		}
		if a.TotalItems != b.TotalItems {
			return a.TotalItems > b.TotalItems
		}
		return a.Name < b.Name
	})

	assessmentPath := "/assessments/" + selected.ID.String()
	unassignedItems := int(dashboard.Overview.UnassignedItems)

	data.SelectedAssessment = selected
	data.Dashboard = &dashboard
	data.UnassignedItems = unassignedItems
	data.Focus = buildDashboardFocus(dashboard, unassignedItems, assessmentPath)
	return s.render(c, "dashboard", data)
}

func buildDashboardFocus(dashboard service.DashboardData, unassignedItems int, assessmentPath string) DashboardFocus {
	overview := dashboard.Overview

	switch {
	case overview.OverdueItems > 0:
		return DashboardFocus{
			Kicker:      "Contain drift",
			Title:       fmt.Sprintf("%d overdue controls need attention.", overview.OverdueItems),
			Body:        "Start with the overdue queue, clear anything stalled, and reset ownership or dates only where it sharpens the plan.",
			ActionLabel: "Open overdue work",
			ActionHref:  assessmentPath + "?overdue=1",
		}
	case overview.BlockedItems > 0:
		return DashboardFocus{
			Kicker:      "Remove blockers",
			Title:       fmt.Sprintf("%d controls are blocked right now.", overview.BlockedItems),
			Body:        "Work the blocked controls first so the cycle can move again. Tighten the reason, then reopen the path for owners and reviewers.",
			ActionLabel: "Open blocked work",
			ActionHref:  assessmentPath + "?status=blocked",
		}
	case unassignedItems > 0:
		return DashboardFocus{
			Kicker:      "Create ownership",
			Title:       fmt.Sprintf("%d controls still need an owner.", unassignedItems),
			Body:        "Assignment is the fastest way to turn this cycle into a working queue. Claim a slice, assign owners, and then let dates follow the real plan.",
			ActionLabel: "Open unassigned work",
			ActionHref:  assessmentPath + "?unassigned=1",
		}
	case overview.ReadyForReviewItems > 0:
		return DashboardFocus{
			Kicker:      "Pull review forward",
			Title:       fmt.Sprintf("%d controls are ready for review.", overview.ReadyForReviewItems),
			Body:        "Use the review queue to close finished work while the evidence is still fresh and the owners still have context.",
			ActionLabel: "Open review queue",
			ActionHref:  assessmentPath + "?status=ready_for_review",
		}
	case len(dashboard.ByGroup) > 0:
		group := dashboard.ByGroup[0]
		return DashboardFocus{
			Kicker:      "Advance the cycle",
			Title:       fmt.Sprintf("Group %s is the thinnest part of the assessment.", group.GroupCode),
			Body:        "Nothing urgent is stalling the queue. Move the lowest-completion group next so progress stays visible and uneven coverage does not spread.",
			ActionLabel: "Open assessment",
			ActionHref:  assessmentPath,
		}
	default:
		return DashboardFocus{
			Kicker:      "Open the cycle",
			Title:       "No urgent queue pressure right now.",
			Body:        "Use the dashboard to choose the next slice, then move into the assessment workspace to assign, review, and close controls.",
			ActionLabel: "Open assessment",
			ActionHref:  assessmentPath,
		}
	}
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
