package httpui

import (
	"net/url"

	"mycis/internal/db"
	"mycis/internal/service"
)

type Flash struct {
	Kind    string
	Message string
}

type BaseData struct {
	Title           string
	AppName         string
	ActiveNav       string
	CurrentUser     *db.User
	CSRFToken       string
	Flashes         []Flash
	Query           url.Values
	HideHeaderTitle bool
}

type LoginPageData struct {
	BaseData
	Email        string
	ErrorMessage string
}

type ChangePasswordPageData struct {
	BaseData
}

type DashboardPageData struct {
	BaseData
	Assessments        []db.ListAssessmentsRow
	SelectedAssessment *db.ListAssessmentsRow
	Dashboard          *service.DashboardData
	Focus              DashboardFocus
	UnassignedItems    int
}

type DashboardFocus struct {
	Kicker      string
	Title       string
	Body        string
	ActionLabel string
	ActionHref  string
}

type FrameworksPageData struct {
	BaseData
	Frameworks        []db.ListFrameworksWithCountsRow
	SelectedFramework *db.ListFrameworksWithCountsRow
	Groups            []db.ListFrameworkGroupsByFrameworkRow
}

type AssessmentsPageData struct {
	BaseData
	Assessments []db.ListAssessmentsRow
}

type AssessmentNewPageData struct {
	BaseData
	Frameworks []db.ListFrameworksWithCountsRow
}

type GroupOption struct {
	Code  string
	Title string
}

type AssessmentDetailPageData struct {
	BaseData
	Assessment          db.GetAssessmentByIDRow
	Items               []db.ListAssessmentItemsRow
	Users               []db.User
	Groups              []GroupOption
	Tags                []string
	Filters             service.AssessmentItemFilters
	QueueRows           []AssessmentQueueRow
	WorkspaceStats      AssessmentWorkspaceStats
	WorkspaceFocus      AssessmentWorkspaceFocus
	FilterChips         []string
	HasActiveFilters    bool
	HasSecondaryFilters bool
}

type AssessmentQueueRow struct {
	Item      db.ListAssessmentItemsRow
	ShowGroup bool
}

type AssessmentWorkspaceStats struct {
	TotalItems          int
	VisibleItems        int
	ReadyForReviewItems int
	OverdueItems        int
	BlockedItems        int
	UnassignedItems     int
	HighPriorityItems   int
	ValidatedItems      int
}

type AssessmentWorkspaceFocus struct {
	Kicker string
	Title  string
	Body   string
}

type ItemDetailPageData struct {
	BaseData
	Detail               service.ItemDetail
	Users                []db.User
	CanEditItem          bool
	CanManageAssessments bool
}

type AssessmentCyclePageData struct {
	BaseData
	Assessment db.GetAssessmentByIDRow
}

type UsersPageData struct {
	BaseData
	Users                []db.User
	Roles                []db.UserRole
	CreateUserDialogOpen bool
	CreateUserForm       UserCreateFormData
	EditUserDialogOpen   bool
	EditUserForm         UserEditFormData
}

type UserCreateFormData struct {
	Name  string
	Email string
	Role  string
}

type UserEditFormData struct {
	ID    string
	Name  string
	Email string
	Role  string
}
