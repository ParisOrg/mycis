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
	Title       string
	AppName     string
	ActiveNav   string
	CurrentUser *db.User
	CSRFToken   string
	Flashes     []Flash
	Query       url.Values
}

type LoginPageData struct {
	BaseData
	Email string
}

type ChangePasswordPageData struct {
	BaseData
}

type DashboardPageData struct {
	BaseData
	Assessments        []db.ListAssessmentsRow
	SelectedAssessment *db.ListAssessmentsRow
	Dashboard          *service.DashboardData
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
	Assessment db.GetAssessmentByIDRow
	Items      []db.ListAssessmentItemsRow
	Users      []db.User
	Groups     []GroupOption
	Tags       []string
	Filters    service.AssessmentItemFilters
}

type ItemDetailPageData struct {
	BaseData
	Detail service.ItemDetail
	Users  []db.User
}

type AssessmentCyclePageData struct {
	BaseData
	Assessment db.GetAssessmentByIDRow
}

type UsersPageData struct {
	BaseData
	Users []db.User
}
