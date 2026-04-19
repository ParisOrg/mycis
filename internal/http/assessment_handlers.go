package httpui

import (
	"net/http"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"

	"mycis/internal/db"
	"mycis/internal/service"
)

func (s *Server) assessmentNewPage(c *echo.Context) error {
	if !s.requireAssessmentManager(c) {
		return nil
	}

	frameworks, err := s.services.Frameworks.ListFrameworks(c.Request().Context())
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	return s.render(c, "assessment_new", AssessmentNewPageData{
		BaseData:   s.baseData(c, "New Assessment", "assessments"),
		Frameworks: frameworks,
	})
}

func (s *Server) assessmentCreatePost(c *echo.Context) error {
	if !s.requireAssessmentManager(c) {
		return nil
	}

	form, err := s.readFormOrRedirect(c, "/assessments/new", "Could not read the assessment form.")
	if err != nil {
		return err
	}

	frameworkID, err := s.requiredUUID(c, form.Get("framework_id"), "/assessments/new", "Select a framework.")
	if err != nil {
		return err
	}
	startDate, err := s.requiredDate(c, form.Get("start_date"), "/assessments/new", "Enter a valid start date.")
	if err != nil {
		return err
	}
	dueDate, err := s.requiredDate(c, form.Get("due_date"), "/assessments/new", "Enter a valid due date.")
	if err != nil {
		return err
	}

	assessment, err := s.services.Assessments.CreateAssessment(c.Request().Context(), *s.currentUser(c), service.CreateAssessmentInput{
		FrameworkID: frameworkID,
		Name:        strings.TrimSpace(form.Get("name")),
		Scope:       strings.TrimSpace(form.Get("scope")),
		StartDate:   startDate,
		DueDate:     dueDate,
	})
	if err != nil {
		return s.redirectWithFlash(c, "/assessments/new", "error", err.Error())
	}

	return s.redirectWithFlash(c, "/assessments/"+assessment.ID.String(), "success", "Assessment created.")
}

func (s *Server) assessmentDetailPage(c *echo.Context) error {
	assessmentID := c.Param("assessmentID")
	assessment, err := s.services.Assessments.GetAssessment(c.Request().Context(), assessmentID)
	if err != nil {
		return renderServiceError(c, err)
	}

	allItems, err := s.services.Assessments.ListAssessmentItems(c.Request().Context(), assessmentID, service.AssessmentItemFilters{})
	if err != nil {
		return renderServiceError(c, err)
	}

	filters := readItemFilters(c)
	items := allItems
	if hasFilters(filters) {
		items, err = s.services.Assessments.ListAssessmentItems(c.Request().Context(), assessmentID, filters)
		if err != nil {
			return renderServiceError(c, err)
		}
	}

	users, err := s.services.Auth.ListUsers(c.Request().Context())
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	groups := buildGroupOptions(allItems)
	tags := buildTagOptions(allItems)

	return s.render(c, "assessment_show", AssessmentDetailPageData{
		BaseData:   s.baseData(c, assessment.Name, "assessments"),
		Assessment: assessment,
		Items:      items,
		Users:      users,
		Groups:     groups,
		Tags:       tags,
		Filters:    filters,
	})
}

func buildGroupOptions(items []db.ListAssessmentItemsRow) []GroupOption {
	type groupOptionWithSort struct {
		GroupOption
		SortOrder int32
	}

	seen := make(map[string]struct{}, len(items))
	groups := make([]groupOptionWithSort, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item.GroupCode]; ok {
			continue
		}
		seen[item.GroupCode] = struct{}{}
		groups = append(groups, groupOptionWithSort{
			GroupOption: GroupOption{
				Code:  item.GroupCode,
				Title: item.GroupTitle,
			},
			SortOrder: item.GroupSortOrder,
		})
	}

	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].SortOrder == groups[j].SortOrder {
			return groups[i].Code < groups[j].Code
		}
		return groups[i].SortOrder < groups[j].SortOrder
	})

	options := make([]GroupOption, 0, len(groups))
	for _, group := range groups {
		options = append(options, group.GroupOption)
	}
	return options
}

func buildTagOptions(items []db.ListAssessmentItemsRow) []string {
	seen := make(map[string]struct{})
	tags := make([]string, 0)
	for _, item := range items {
		for _, tag := range item.Tags {
			if _, ok := seen[tag]; ok {
				continue
			}
			seen[tag] = struct{}{}
			tags = append(tags, tag)
		}
	}
	return tags
}

func (s *Server) assessmentBulkPost(c *echo.Context) error {
	if !s.requireAssessmentManager(c) {
		return nil
	}

	assessmentID := c.Param("assessmentID")
	id, err := uuid.Parse(assessmentID)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}

	form, err := s.readFormOrRedirect(c, "/assessments/"+assessmentID, "Could not read the bulk update form.")
	if err != nil {
		return err
	}

	itemIDs := make([]uuid.UUID, 0, len(form["item_ids"]))
	for _, raw := range form["item_ids"] {
		if parsed := s.optionalUUID(raw); parsed != nil {
			itemIDs = append(itemIDs, *parsed)
		}
	}

	input := service.BulkUpdateInput{
		AssessmentID: id,
		ItemIDs:      itemIDs,
		Action:       form.Get("action"),
	}

	switch input.Action {
	case "assign_owner", "assign_reviewer":
		input.UserID = s.optionalUUID(form.Get("user_id"))
	case "set_due_date":
		input.DueDate = s.optionalDate(form.Get("due_date"))
	case "set_priority":
		priority, err := service.ParseItemPriority(form.Get("priority"))
		if err != nil {
			return s.redirectWithFlash(c, "/assessments/"+assessmentID, "error", err.Error())
		}
		input.Priority = &priority
	}

	if err := s.services.Assessments.BulkUpdateItems(c.Request().Context(), *s.currentUser(c), input); err != nil {
		return s.redirectWithFlash(c, "/assessments/"+assessmentID, "error", err.Error())
	}

	return s.redirectWithFlash(c, "/assessments/"+assessmentID, "success", "Bulk update applied.")
}

func (s *Server) assessmentCyclePage(c *echo.Context) error {
	if !s.requireAssessmentManager(c) {
		return nil
	}

	assessmentID := c.Param("assessmentID")
	assessment, err := s.services.Assessments.GetAssessment(c.Request().Context(), assessmentID)
	if err != nil {
		return renderServiceError(c, err)
	}
	return s.render(c, "assessment_cycle", AssessmentCyclePageData{
		BaseData:   s.baseData(c, "New Cycle from "+assessment.Name, "assessments"),
		Assessment: assessment,
	})
}

func (s *Server) assessmentCyclePost(c *echo.Context) error {
	if !s.requireAssessmentManager(c) {
		return nil
	}

	assessmentID := c.Param("assessmentID")
	prevID, err := uuid.Parse(assessmentID)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}

	form, err := s.readFormOrRedirect(c, "/assessments/"+assessmentID+"/cycle", "Could not read the cycle form.")
	if err != nil {
		return err
	}
	startDate, err := s.requiredDate(c, form.Get("start_date"), "/assessments/"+assessmentID+"/cycle", "Enter a valid start date.")
	if err != nil {
		return err
	}
	dueDate, err := s.requiredDate(c, form.Get("due_date"), "/assessments/"+assessmentID+"/cycle", "Enter a valid due date.")
	if err != nil {
		return err
	}

	assessment, err := s.services.Assessments.CreateCycleFromPrevious(c.Request().Context(), *s.currentUser(c), service.CreateCycleInput{
		PreviousAssessmentID: prevID,
		Name:                 strings.TrimSpace(form.Get("name")),
		Scope:                strings.TrimSpace(form.Get("scope")),
		StartDate:            startDate,
		DueDate:              dueDate,
	})
	if err != nil {
		return s.redirectWithFlash(c, "/assessments/"+assessmentID+"/cycle", "error", err.Error())
	}
	return s.redirectWithFlash(c, "/assessments/"+assessment.ID.String(), "success", "New cycle created.")
}
