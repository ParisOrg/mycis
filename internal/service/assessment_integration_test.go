package service

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"mycis/internal/db"
)

func TestAssessmentControlRecordsRemainIsolatedAcrossAssessmentsAndCycles(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	rootDir := repoRoot(t)
	withWorkingDirectory(t, rootDir)

	testDatabaseURL := createIntegrationDatabase(t, ctx, databaseURL)
	runMigrationsForTest(t, testDatabaseURL, rootDir)

	pool, err := pgxpool.New(ctx, testDatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	services := New(pool)

	if err := services.Frameworks.SeedFramework(ctx, "cis-v8-1", false); err != nil {
		t.Fatal(err)
	}

	admin, err := services.Auth.CreateUserWithPassword(ctx, "Admin", "admin@example.com", "password-12345", true, false)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := services.Auth.CreateUserWithPassword(ctx, "Owner", "owner@example.com", "password-12345", false, false)
	if err != nil {
		t.Fatal(err)
	}
	reviewer, err := services.Auth.CreateUserWithPassword(ctx, "Reviewer", "reviewer@example.com", "password-12345", false, false)
	if err != nil {
		t.Fatal(err)
	}

	frameworks, err := services.Frameworks.ListFrameworks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(frameworks) != 1 {
		t.Fatalf("expected one framework, got %d", len(frameworks))
	}

	startDate := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	dueDate := time.Date(2026, time.April, 30, 0, 0, 0, 0, time.UTC)

	assessmentOne, err := services.Assessments.CreateAssessment(ctx, admin, CreateAssessmentInput{
		FrameworkID: frameworks[0].ID,
		Name:        "Assessment One",
		Scope:       "Production",
		StartDate:   startDate,
		DueDate:     dueDate,
	})
	if err != nil {
		t.Fatal(err)
	}

	itemsOne, err := services.Assessments.ListAssessmentItems(ctx, assessmentOne.ID.String(), AssessmentItemFilters{})
	if err != nil {
		t.Fatal(err)
	}
	if len(itemsOne) == 0 {
		t.Fatal("expected seeded assessment items")
	}

	sourceItem := itemsOne[0]
	notes := "Documented as not applicable in the original assessment"
	if err := services.Items.Update(ctx, admin, UpdateItemInput{
		ID:             sourceItem.ID,
		OwnerUserID:    &owner.ID,
		ReviewerUserID: &reviewer.ID,
		Status:         db.AssessmentItemStatusNotApplicable,
		Priority:       sourceItem.Priority,
		DueDate:        sourceItem.DueDate,
		Notes:          &notes,
	}); err != nil {
		t.Fatal(err)
	}

	if err := services.Items.AddComment(ctx, owner, sourceItem.ID.String(), "Owner note"); err != nil {
		t.Fatal(err)
	}
	if err := services.Items.AddEvidenceLink(ctx, owner, sourceItem.ID.String(), "Evidence", "https://example.com/evidence"); err != nil {
		t.Fatal(err)
	}

	assessmentTwo, err := services.Assessments.CreateAssessment(ctx, admin, CreateAssessmentInput{
		FrameworkID: frameworks[0].ID,
		Name:        "Assessment Two",
		Scope:       "Production",
		StartDate:   startDate.AddDate(0, 1, 0),
		DueDate:     dueDate.AddDate(0, 1, 0),
	})
	if err != nil {
		t.Fatal(err)
	}

	itemsTwo, err := services.Assessments.ListAssessmentItems(ctx, assessmentTwo.ID.String(), AssessmentItemFilters{})
	if err != nil {
		t.Fatal(err)
	}
	secondItem := findAssessmentItem(t, itemsTwo, sourceItem.FrameworkItemID)
	if secondItem.OwnerUserID.Valid {
		t.Fatalf("expected owner to be reset, got %v", secondItem.OwnerUserID)
	}
	if secondItem.ReviewerUserID.Valid {
		t.Fatalf("expected reviewer to be reset, got %v", secondItem.ReviewerUserID)
	}
	if secondItem.Status != db.AssessmentItemStatusNotStarted {
		t.Fatalf("expected status reset to not_started, got %s", secondItem.Status)
	}
	if secondItem.Notes != "" {
		t.Fatalf("expected notes to reset, got %q", secondItem.Notes)
	}

	secondDetail, err := services.Items.GetDetail(ctx, secondItem.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(secondDetail.Comments) != 0 {
		t.Fatalf("expected no comments on new assessment, got %d", len(secondDetail.Comments))
	}
	if len(secondDetail.Evidence) != 0 {
		t.Fatalf("expected no evidence on new assessment, got %d", len(secondDetail.Evidence))
	}

	cycle, err := services.Assessments.CreateCycleFromPrevious(ctx, admin, CreateCycleInput{
		PreviousAssessmentID: assessmentOne.ID,
		Name:                 "Assessment One Cycle 2",
		Scope:                "Production",
		StartDate:            startDate.AddDate(0, 2, 0),
		DueDate:              dueDate.AddDate(0, 2, 0),
	})
	if err != nil {
		t.Fatal(err)
	}

	cycleItems, err := services.Assessments.ListAssessmentItems(ctx, cycle.ID.String(), AssessmentItemFilters{})
	if err != nil {
		t.Fatal(err)
	}
	cycleItem := findAssessmentItem(t, cycleItems, sourceItem.FrameworkItemID)
	if !cycleItem.OwnerUserID.Valid || cycleItem.OwnerUserID.Bytes != owner.ID {
		t.Fatalf("expected owner to carry forward, got %v", cycleItem.OwnerUserID)
	}
	if !cycleItem.ReviewerUserID.Valid || cycleItem.ReviewerUserID.Bytes != reviewer.ID {
		t.Fatalf("expected reviewer to carry forward, got %v", cycleItem.ReviewerUserID)
	}
	if cycleItem.Status != db.AssessmentItemStatusNotStarted {
		t.Fatalf("expected cycle status reset to not_started, got %s", cycleItem.Status)
	}
	if cycleItem.Notes != "" {
		t.Fatalf("expected cycle notes to reset, got %q", cycleItem.Notes)
	}

	cycleDetail, err := services.Items.GetDetail(ctx, cycleItem.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if cycleDetail.Item.IsNotApplicable {
		t.Fatal("expected cycle not_applicable flag to reset")
	}
	if len(cycleDetail.Comments) != 0 {
		t.Fatalf("expected no cycle comments, got %d", len(cycleDetail.Comments))
	}
	if len(cycleDetail.Evidence) != 0 {
		t.Fatalf("expected no cycle evidence, got %d", len(cycleDetail.Evidence))
	}

	sourceDetail, err := services.Items.GetDetail(ctx, sourceItem.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if !sourceDetail.Item.IsNotApplicable {
		t.Fatal("expected original assessment to remain not applicable")
	}
	if len(sourceDetail.Comments) != 1 {
		t.Fatalf("expected original assessment comments to remain, got %d", len(sourceDetail.Comments))
	}
	if len(sourceDetail.Evidence) != 1 {
		t.Fatalf("expected original assessment evidence to remain, got %d", len(sourceDetail.Evidence))
	}
}

func TestForceReseedUsesOnlyActiveFrameworkRowsForNewAssessmentsAndCycles(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	rootDir := repoRoot(t)
	withWorkingDirectory(t, rootDir)

	testDatabaseURL := createIntegrationDatabase(t, ctx, databaseURL)
	runMigrationsForTest(t, testDatabaseURL, rootDir)

	pool, err := pgxpool.New(ctx, testDatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	services := New(pool)

	slug := fmt.Sprintf("force-reseed-%d", time.Now().UnixNano())
	seedPath := filepath.Join(rootDir, "seed", "frameworks", slug+".yaml")
	t.Cleanup(func() {
		_ = os.Remove(seedPath)
	})

	if err := os.WriteFile(seedPath, []byte(testFrameworkSeedYAML(slug, "1.0.0", "1.1")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := services.Frameworks.SeedFramework(ctx, slug, false); err != nil {
		t.Fatal(err)
	}

	admin, err := services.Auth.CreateUserWithPassword(ctx, "Admin", "admin@example.com", "password-12345", true, false)
	if err != nil {
		t.Fatal(err)
	}

	frameworks, err := services.Frameworks.ListFrameworks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(frameworks) != 1 {
		t.Fatalf("expected one framework, got %d", len(frameworks))
	}

	startDate := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	dueDate := time.Date(2026, time.April, 30, 0, 0, 0, 0, time.UTC)

	originalAssessment, err := services.Assessments.CreateAssessment(ctx, admin, CreateAssessmentInput{
		FrameworkID: frameworks[0].ID,
		Name:        "Original Assessment",
		Scope:       "Production",
		StartDate:   startDate,
		DueDate:     dueDate,
	})
	if err != nil {
		t.Fatal(err)
	}

	originalItems, err := services.Assessments.ListAssessmentItems(ctx, originalAssessment.ID.String(), AssessmentItemFilters{})
	if err != nil {
		t.Fatal(err)
	}
	if codes := assessmentItemCodes(originalItems); len(codes) != 1 || codes[0] != "1.1" {
		t.Fatalf("expected original assessment to contain [1.1], got %v", codes)
	}

	if err := os.WriteFile(seedPath, []byte(testFrameworkSeedYAML(slug, "1.0.0", "1.2")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := services.Frameworks.SeedFramework(ctx, slug, true); err != nil {
		t.Fatal(err)
	}

	reseededAssessment, err := services.Assessments.CreateAssessment(ctx, admin, CreateAssessmentInput{
		FrameworkID: frameworks[0].ID,
		Name:        "Reseeded Assessment",
		Scope:       "Production",
		StartDate:   startDate.AddDate(0, 1, 0),
		DueDate:     dueDate.AddDate(0, 1, 0),
	})
	if err != nil {
		t.Fatal(err)
	}

	reseededItems, err := services.Assessments.ListAssessmentItems(ctx, reseededAssessment.ID.String(), AssessmentItemFilters{})
	if err != nil {
		t.Fatal(err)
	}
	if codes := assessmentItemCodes(reseededItems); len(codes) != 1 || codes[0] != "1.2" {
		t.Fatalf("expected new assessment to contain only active item [1.2], got %v", codes)
	}

	cycle, err := services.Assessments.CreateCycleFromPrevious(ctx, admin, CreateCycleInput{
		PreviousAssessmentID: originalAssessment.ID,
		Name:                 "Cycle After Reseed",
		Scope:                "Production",
		StartDate:            startDate.AddDate(0, 2, 0),
		DueDate:              dueDate.AddDate(0, 2, 0),
	})
	if err != nil {
		t.Fatal(err)
	}

	cycleItems, err := services.Assessments.ListAssessmentItems(ctx, cycle.ID.String(), AssessmentItemFilters{})
	if err != nil {
		t.Fatal(err)
	}
	if codes := assessmentItemCodes(cycleItems); len(codes) != 1 || codes[0] != "1.2" {
		t.Fatalf("expected cycle to contain only active item [1.2], got %v", codes)
	}

	originalItemsAfterReseed, err := services.Assessments.ListAssessmentItems(ctx, originalAssessment.ID.String(), AssessmentItemFilters{})
	if err != nil {
		t.Fatal(err)
	}
	if codes := assessmentItemCodes(originalItemsAfterReseed); len(codes) != 1 || codes[0] != "1.1" {
		t.Fatalf("expected historical assessment to keep [1.1], got %v", codes)
	}
}

func TestUpdateClearsNotesAndNotApplicableFlag(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	rootDir := repoRoot(t)
	withWorkingDirectory(t, rootDir)

	testDatabaseURL := createIntegrationDatabase(t, ctx, databaseURL)
	runMigrationsForTest(t, testDatabaseURL, rootDir)

	pool, err := pgxpool.New(ctx, testDatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	services := New(pool)

	if err := services.Frameworks.SeedFramework(ctx, "cis-v8-1", false); err != nil {
		t.Fatal(err)
	}

	admin, err := services.Auth.CreateUserWithPassword(ctx, "Admin", "admin@example.com", "password-12345", true, false)
	if err != nil {
		t.Fatal(err)
	}

	frameworks, err := services.Frameworks.ListFrameworks(ctx)
	if err != nil {
		t.Fatal(err)
	}

	assessment, err := services.Assessments.CreateAssessment(ctx, admin, CreateAssessmentInput{
		FrameworkID: frameworks[0].ID,
		Name:        "Assessment",
		Scope:       "Production",
		StartDate:   time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		DueDate:     time.Date(2026, time.April, 30, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}

	items, err := services.Assessments.ListAssessmentItems(ctx, assessment.ID.String(), AssessmentItemFilters{})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 {
		t.Fatal("expected seeded assessment items")
	}

	item := items[0]
	notes := "Legacy note"
	if err := services.Items.Update(ctx, admin, UpdateItemInput{
		ID:       item.ID,
		Status:   db.AssessmentItemStatusNotApplicable,
		Priority: item.Priority,
		DueDate:  item.DueDate,
		Notes:    &notes,
	}); err != nil {
		t.Fatal(err)
	}

	if err := services.Items.Update(ctx, admin, UpdateItemInput{
		ID:       item.ID,
		Status:   db.AssessmentItemStatusInProgress,
		Priority: item.Priority,
		DueDate:  item.DueDate,
	}); err != nil {
		t.Fatal(err)
	}

	detail, err := services.Items.GetDetail(ctx, item.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if detail.Item.IsNotApplicable {
		t.Fatal("expected not-applicable flag to be cleared")
	}
	if detail.Item.Notes != "" {
		t.Fatalf("expected notes to be cleared, got %q", detail.Item.Notes)
	}
}

func findAssessmentItem(t *testing.T, items []db.ListAssessmentItemsRow, frameworkItemID uuid.UUID) db.ListAssessmentItemsRow {
	t.Helper()

	for _, item := range items {
		if item.FrameworkItemID == frameworkItemID {
			return item
		}
	}

	t.Fatalf("assessment item for framework item %s not found", frameworkItemID)
	return db.ListAssessmentItemsRow{}
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine caller path")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func withWorkingDirectory(t *testing.T, dir string) {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Errorf("restore cwd: %v", err)
		}
	})
}

func createIntegrationDatabase(t *testing.T, ctx context.Context, baseDatabaseURL string) string {
	t.Helper()

	adminPool, err := pgxpool.New(ctx, baseDatabaseURL)
	if err != nil {
		t.Fatal(err)
	}

	databaseName := fmt.Sprintf("mycis_smoke_%d", time.Now().UnixNano())
	if _, err := adminPool.Exec(ctx, `CREATE DATABASE "`+databaseName+`"`); err != nil {
		adminPool.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := adminPool.Exec(context.Background(), `DROP DATABASE IF EXISTS "`+databaseName+`" WITH (FORCE)`); err != nil {
			t.Errorf("drop smoke database: %v", err)
		}
		adminPool.Close()
	})

	parsedURL, err := url.Parse(baseDatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	parsedURL.Path = "/" + databaseName
	return parsedURL.String()
}

func runMigrationsForTest(t *testing.T, databaseURL, rootDir string) {
	t.Helper()

	sourceURL := "file://" + filepath.Join(rootDir, "db", "migrations")
	m, err := migrate.New(sourceURL, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = m.Close()
	}()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatal(err)
	}
}

func assessmentItemCodes(items []db.ListAssessmentItemsRow) []string {
	codes := make([]string, 0, len(items))
	for _, item := range items {
		codes = append(codes, item.ItemCode)
	}
	return codes
}

func testFrameworkSeedYAML(slug, version string, itemCodes ...string) string {
	var builder strings.Builder
	builder.WriteString("framework:\n")
	fmt.Fprintf(&builder, "  slug: %s\n", slug)
	builder.WriteString("  label: Test Framework\n")
	fmt.Fprintf(&builder, "  version: %s\n", version)
	builder.WriteString("groups:\n")
	builder.WriteString("  - code: \"1\"\n")
	builder.WriteString("    title: Group 1\n")
	builder.WriteString("    summary: Test summary\n")
	builder.WriteString("    description: Test description\n")
	builder.WriteString("items:\n")
	for _, code := range itemCodes {
		builder.WriteString("  - group_code: \"1\"\n")
		fmt.Fprintf(&builder, "    code: \"%s\"\n", code)
		fmt.Fprintf(&builder, "    title: Control %s\n", code)
		fmt.Fprintf(&builder, "    summary: Summary %s\n", code)
		fmt.Fprintf(&builder, "    description: Description %s\n", code)
		builder.WriteString("    asset_class: Devices\n")
		builder.WriteString("    security_function: Identify\n")
		builder.WriteString("    tags:\n")
		builder.WriteString("      - ig1\n")
	}
	return builder.String()
}
