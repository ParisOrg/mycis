package httpui

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"mycis/internal/config"
	"mycis/internal/db"
	"mycis/internal/service"
)

const sessionName = "mycis_session"

type Server struct {
	cfg       config.Config
	services  *service.Services
	store     *sessions.CookieStore
	templates map[string]*template.Template
}

type pageConfig struct {
	Layout string
	File   string
}

type templateRenderer struct {
	templates map[string]*template.Template
}

func (r templateRenderer) Render(c *echo.Context, w io.Writer, page string, data any) error {
	tmpl, ok := r.templates[page]
	if !ok {
		return echo.NewHTTPError(http.StatusInternalServerError, "template missing")
	}
	return tmpl.ExecuteTemplate(w, "layout", data)
}

func NewServer(cfg config.Config, services *service.Services) (*Server, error) {
	store := sessions.NewCookieStore([]byte(cfg.SessionKey))
	store.Options = &sessions.Options{
		Path:     "/",
		HttpOnly: true,
		Secure:   cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   60 * 60 * 12,
	}

	templates, err := buildTemplateCache()
	if err != nil {
		return nil, err
	}

	return &Server{
		cfg:       cfg,
		services:  services,
		store:     store,
		templates: templates,
	}, nil
}

func (s *Server) Router() http.Handler {
	e := echo.New()
	e.Renderer = templateRenderer{templates: s.templates}

	e.Use(middleware.RequestID())
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	e.Use(s.withCurrentUser)

	e.Static("/assets", "public/assets")
	e.GET("/", s.handleRoot)
	e.GET("/login", s.loginPage)
	e.POST("/login", s.loginPost, s.protectCSRF)

	auth := e.Group("")
	auth.Use(s.requireAuth)
	auth.Use(s.protectCSRF)
	auth.POST("/logout", s.logoutPost)
	auth.GET("/change-password", s.changePasswordPage)
	auth.POST("/change-password", s.changePasswordPost)

	protected := auth.Group("")
	protected.Use(s.enforcePasswordChange)
	protected.GET("/dashboard", s.dashboardPage)
	protected.GET("/frameworks", s.frameworksPage)
	protected.GET("/assessments", s.assessmentsPage)
	protected.GET("/assessments/new", s.assessmentNewPage)
	protected.POST("/assessments/new", s.assessmentCreatePost)
	protected.GET("/assessments/:assessmentID", s.assessmentDetailPage)
	protected.POST("/assessments/:assessmentID/bulk", s.assessmentBulkPost)
	protected.GET("/assessments/:assessmentID/cycle", s.assessmentCyclePage)
	protected.POST("/assessments/:assessmentID/cycle", s.assessmentCyclePost)
	protected.GET("/items/:itemID", s.itemDetailPage)
	protected.POST("/items/:itemID", s.itemUpdatePost)
	protected.POST("/items/:itemID/comments", s.itemCommentPost)
	protected.POST("/items/:itemID/evidence", s.itemEvidencePost)
	protected.GET("/admin/users", s.usersPage)
	protected.POST("/admin/users", s.userCreatePost)
	protected.POST("/admin/users/edit", s.userUpdatePost)

	return e
}

func buildTemplateCache() (map[string]*template.Template, error) {
	pages := map[string]pageConfig{
		"login":            {Layout: "auth", File: "login.gohtml"},
		"change_password":  {Layout: "auth", File: "change_password.gohtml"},
		"dashboard":        {Layout: "app", File: "dashboard.gohtml"},
		"frameworks":       {Layout: "app", File: "frameworks.gohtml"},
		"assessments":      {Layout: "app", File: "assessments.gohtml"},
		"assessment_new":   {Layout: "app", File: "assessment_new.gohtml"},
		"assessment_show":  {Layout: "app", File: "assessment_detail.gohtml"},
		"assessment_cycle": {Layout: "app", File: "assessment_cycle.gohtml"},
		"item_show":        {Layout: "app", File: "item_detail.gohtml"},
		"users":            {Layout: "app", File: "users.gohtml"},
	}

	partialFiles, err := filepath.Glob(filepath.Join("internal", "http", "templates", "partials", "*.gohtml"))
	if err != nil {
		return nil, fmt.Errorf("glob partials: %w", err)
	}

	cache := make(map[string]*template.Template, len(pages))
	for name, cfg := range pages {
		files := append([]string{filepath.Join("internal", "http", "templates", "layouts", cfg.Layout+".gohtml")}, partialFiles...)
		files = append(files, filepath.Join("internal", "http", "templates", "pages", cfg.File))

		tmpl, err := template.New(name).Funcs(template.FuncMap{
			"formatDate":       formatDate,
			"formatDateTime":   formatDateTime,
			"percentage":       percentage,
			"statusClass":      statusClass,
			"statusLabel":      statusLabel,
			"scoreValue":       scoreValue,
			"stringValue":      stringValue,
			"uuidText":         uuidText,
			"containsTag":      containsTag,
			"tagLabel":         tagLabel,
			"queryWith":        queryWith,
			"add":              add,
			"boolValue":        boolValue,
			"ptrEquals":        ptrEquals,
			"intPtrEquals":     intPtrEquals,
			"trimPointerText":  trimPointerText,
			"assessmentStatus": assessmentStatus,
			"itemPriority":     itemPriority,
			"numericFloat":     numericFloat,
			"daysOverdue":      daysOverdue,
		}).ParseFiles(files...)
		if err != nil {
			return nil, fmt.Errorf("parse template %s: %w", name, err)
		}
		cache[name] = tmpl
	}

	return cache, nil
}

func (s *Server) render(c *echo.Context, page string, data any) error {
	return c.Render(http.StatusOK, page, data)
}

func formatDate(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Format("2006-01-02")
}

func formatDateTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Local().Format("2006-01-02 15:04")
}

func percentage(done, total int32) int {
	if total == 0 {
		return 0
	}
	return int(float64(done) / float64(total) * 100)
}

func statusClass(status any) string {
	text := strings.ReplaceAll(fmt.Sprint(status), "_", "-")
	return "status-pill status-" + text
}

func statusLabel(status any) string {
	text := strings.ReplaceAll(fmt.Sprint(status), "_", " ")
	return cases.Title(language.Und).String(text)
}

func scoreValue(value *int32) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%d/5", *value)
}

func stringValue(value *string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return "-"
	}
	return *value
}

func trimPointerText(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func uuidText(value pgtype.UUID) string {
	if !value.Valid {
		return ""
	}
	return uuid.UUID(value.Bytes).String()
}

func containsTag(tags []string, target string) bool {
	for _, tag := range tags {
		if tag == target {
			return true
		}
	}
	return false
}

func tagLabel(tag string) string {
	return strings.ToUpper(tag)
}

func queryWith(values url.Values, key, value string) string {
	copyValues := url.Values{}
	for existingKey, list := range values {
		for _, item := range list {
			copyValues.Add(existingKey, item)
		}
	}
	if value == "" {
		copyValues.Del(key)
	} else {
		copyValues.Set(key, value)
	}
	return copyValues.Encode()
}

func add(left, right int) int {
	return left + right
}

func boolValue(value *bool) bool {
	return value != nil && *value
}

func ptrEquals(value *string, expected string) bool {
	return value != nil && *value == expected
}

func intPtrEquals(value *int32, expected int) bool {
	return value != nil && int(*value) == expected
}

func assessmentStatus(status db.AssessmentStatus) string {
	text := strings.ReplaceAll(string(status), "_", " ")
	return cases.Title(language.Und).String(text)
}

func itemPriority(priority db.ItemPriority) string {
	text := strings.ReplaceAll(string(priority), "_", " ")
	return cases.Title(language.Und).String(text)
}

func numericFloat(n pgtype.Numeric) float64 {
	f, _ := n.Float64Value()
	return f.Float64
}

// daysOverdue returns how many whole days the given due date is past today.
// Returns 0 if the date is today or in the future. Used for inline status
// copy like "12 days late" in overdue lists.
func daysOverdue(due time.Time) int {
	if due.IsZero() {
		return 0
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	dueDay := due.UTC().Truncate(24 * time.Hour)
	if !dueDay.Before(today) {
		return 0
	}
	return int(today.Sub(dueDay).Hours() / 24)
}
