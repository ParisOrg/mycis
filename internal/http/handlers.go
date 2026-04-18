package httpui

import (
	"errors"
	"fmt"
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

	session, _ := s.store.Get(c.Request(), sessionName)
	session.Values["user_id"] = user.ID.String()
	_ = session.Save(c.Request(), c.Response())

	if user.MustChangePassword {
		return c.Redirect(http.StatusSeeOther, "/change-password")
	}

	return c.Redirect(http.StatusSeeOther, "/dashboard")
}

func (s *Server) logoutPost(c *echo.Context) error {
	session, _ := s.store.Get(c.Request(), sessionName)
	session.Options.MaxAge = -1
	_ = session.Save(c.Request(), c.Response())
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

func (s *Server) assessmentNewPage(c *echo.Context) error {
	if !s.requireAdmin(c) {
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
	if !s.requireAdmin(c) {
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
	seen := make(map[string]struct{}, len(items))
	groups := make([]GroupOption, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item.GroupCode]; ok {
			continue
		}
		seen[item.GroupCode] = struct{}{}
		groups = append(groups, GroupOption{
			Code:  item.GroupCode,
			Title: item.GroupTitle,
		})
	}
	return groups
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
	if !s.requireAdmin(c) {
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
	if !s.requireAdmin(c) {
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
	if !s.requireAdmin(c) {
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

func (s *Server) itemDetailPage(c *echo.Context) error {
	itemID := c.Param("itemID")
	detail, err := s.services.Items.GetDetail(c.Request().Context(), itemID)
	if err != nil {
		return renderServiceError(c, err)
	}
	users, err := s.services.Auth.ListUsers(c.Request().Context())
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	return s.render(c, "item_show", ItemDetailPageData{
		BaseData: s.baseData(c, detail.Item.ItemCode+" · "+detail.Item.ItemTitle, "assessments"),
		Detail:   detail,
		Users:    users,
	})
}

func (s *Server) itemUpdatePost(c *echo.Context) error {
	itemID := c.Param("itemID")
	form, err := s.readFormOrRedirect(c, "/items/"+itemID, "Could not read the item form.")
	if err != nil {
		return err
	}

	id, err := uuid.Parse(itemID)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}

	user := s.currentUser(c)
	input, err := readItemUpdateInput(form, id, user != nil && user.IsAdmin)
	if err != nil {
		return s.redirectWithFlash(c, "/items/"+itemID, "error", err.Error())
	}

	if err := s.services.Items.Update(c.Request().Context(), *user, input); err != nil {
		return s.redirectWithFlash(c, "/items/"+itemID, "error", err.Error())
	}

	return s.redirectWithFlash(c, "/items/"+itemID, "success", "Item updated.")
}

func (s *Server) itemCommentPost(c *echo.Context) error {
	itemID := c.Param("itemID")
	form, err := s.readFormOrRedirect(c, "/items/"+itemID, "Could not read the comment form.")
	if err != nil {
		return err
	}
	if err := s.services.Items.AddComment(c.Request().Context(), *s.currentUser(c), itemID, form.Get("body")); err != nil {
		return s.redirectWithFlash(c, "/items/"+itemID, "error", err.Error())
	}
	return s.redirectWithFlash(c, "/items/"+itemID, "success", "Comment added.")
}

func (s *Server) itemEvidencePost(c *echo.Context) error {
	itemID := c.Param("itemID")
	form, err := s.readFormOrRedirect(c, "/items/"+itemID, "Could not read the evidence form.")
	if err != nil {
		return err
	}
	if err := s.services.Items.AddEvidenceLink(c.Request().Context(), *s.currentUser(c), itemID, form.Get("label"), form.Get("url")); err != nil {
		return s.redirectWithFlash(c, "/items/"+itemID, "error", err.Error())
	}
	return s.redirectWithFlash(c, "/items/"+itemID, "success", "Evidence link added.")
}

func (s *Server) usersPage(c *echo.Context) error {
	if !s.requireAdmin(c) {
		return nil
	}

	users, err := s.services.Auth.ListUsers(c.Request().Context())
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	return s.render(c, "users", UsersPageData{
		BaseData: s.baseData(c, "Users", "users"),
		Users:    users,
	})
}

func (s *Server) userCreatePost(c *echo.Context) error {
	if !s.requireAdmin(c) {
		return nil
	}

	form, err := s.readFormOrRedirect(c, "/admin/users", "Could not read the user form.")
	if err != nil {
		return err
	}

	isAdmin := form.Get("is_admin") == "on"
	user, password, err := s.services.Auth.CreateUser(c.Request().Context(), form.Get("name"), form.Get("email"), isAdmin)
	if err != nil {
		return s.redirectWithFlash(c, "/admin/users", "error", err.Error())
	}
	return s.redirectWithFlash(c, "/admin/users", "success", fmt.Sprintf("Created %s. Temporary password: %s", user.Email, password))
}

func (s *Server) baseData(c *echo.Context, title, nav string) BaseData {
	return BaseData{
		Title:       title,
		AppName:     s.cfg.AppName,
		ActiveNav:   nav,
		CurrentUser: s.currentUser(c),
		Flashes:     s.readFlashes(c),
		Query:       c.QueryParams(),
	}
}

func (s *Server) redirectWithFlash(c *echo.Context, destination, kind, message string) error {
	session, _ := s.store.Get(c.Request(), sessionName)
	session.AddFlash(kind + "|" + message)
	_ = session.Save(c.Request(), c.Response())
	return c.Redirect(http.StatusSeeOther, destination)
}

func (s *Server) readFlashes(c *echo.Context) []Flash {
	session, _ := s.store.Get(c.Request(), sessionName)
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

func readItemUpdateInput(form url.Values, itemID uuid.UUID, isAdmin bool) (service.UpdateItemInput, error) {
	status, err := service.ParseAssessmentItemStatus(form.Get("status"))
	if err != nil {
		return service.UpdateItemInput{}, err
	}

	priority, err := service.ParseItemPriority(form.Get("priority"))
	if err != nil {
		return service.UpdateItemInput{}, err
	}

	input := service.UpdateItemInput{
		ID:             itemID,
		Status:         status,
		Priority:       priority,
		OwnerUserID:    parseOptionalUUID(form.Get("owner_user_id")),
		ReviewerUserID: parseOptionalUUID(form.Get("reviewer_user_id")),
		Score:          parseOptionalInt32(form.Get("score")),
		Notes:          textutil.TrimPtr(form.Get("notes")),
		BlockedReason:  textutil.TrimPtr(form.Get("blocked_reason")),
	}

	if !isAdmin {
		return input, nil
	}

	dueDate, err := textutil.ParseDateOnly(strings.TrimSpace(form.Get("due_date")))
	if err != nil {
		return service.UpdateItemInput{}, fmt.Errorf("Enter a valid due date.")
	}
	input.DueDate = dueDate
	return input, nil
}

func parseOptionalUUID(raw string) *uuid.UUID {
	if raw == "" {
		return nil
	}
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return nil
	}
	return &parsed
}

func parseOptionalInt32(raw string) *int32 {
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
