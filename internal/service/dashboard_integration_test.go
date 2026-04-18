package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mycis/internal/db"
)

func TestDashboardGetPreservesFrameworkSortOrder(t *testing.T) {
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

	slug := fmt.Sprintf("dashboard-order-%d", time.Now().UnixNano())
	seedPath := filepath.Join(rootDir, "seed", "frameworks", slug+".yaml")
	t.Cleanup(func() {
		_ = os.Remove(seedPath)
	})

	if err := os.WriteFile(seedPath, []byte(testOrderedFrameworkSeedYAML(slug)), 0o644); err != nil {
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

	assessment, err := services.Assessments.CreateAssessment(ctx, admin, CreateAssessmentInput{
		FrameworkID: frameworks[0].ID,
		Name:        "Ordered Dashboard Assessment",
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
	if got, want := assessmentItemCodes(items), []string{"ZZ-two", "AA-one", "MM-three", "AA-nine"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected seeded assessment item order: got %v want %v", got, want)
	}

	overdueDate := time.Now().UTC().AddDate(0, 0, -7)
	lowScore := int32(2)
	for _, item := range items {
		if err := services.Items.Update(ctx, admin, UpdateItemInput{
			ID:       item.ID,
			Status:   db.AssessmentItemStatusInProgress,
			Score:    &lowScore,
			Priority: item.Priority,
			DueDate:  overdueDate,
		}); err != nil {
			t.Fatal(err)
		}
	}

	dashboard, err := services.Dashboard.Get(ctx, assessment.ID)
	if err != nil {
		t.Fatal(err)
	}

	groupCodes := make([]string, 0, len(dashboard.ByGroup))
	for _, group := range dashboard.ByGroup {
		groupCodes = append(groupCodes, group.GroupCode)
	}
	if want := []string{"ZZ", "AA", "MM"}; !reflect.DeepEqual(groupCodes, want) {
		t.Fatalf("unexpected dashboard group order: got %v want %v", groupCodes, want)
	}

	overdueCodes := make([]string, 0, len(dashboard.Overdue))
	for _, item := range dashboard.Overdue {
		overdueCodes = append(overdueCodes, item.ItemCode)
	}
	if want := []string{"ZZ-two", "AA-one", "AA-nine", "MM-three"}; !reflect.DeepEqual(overdueCodes, want) {
		t.Fatalf("unexpected overdue item order: got %v want %v", overdueCodes, want)
	}

	lowScoreCodes := make([]string, 0, len(dashboard.LowScore))
	for _, item := range dashboard.LowScore {
		lowScoreCodes = append(lowScoreCodes, item.ItemCode)
	}
	if want := []string{"ZZ-two", "AA-one", "AA-nine", "MM-three"}; !reflect.DeepEqual(lowScoreCodes, want) {
		t.Fatalf("unexpected low score item order: got %v want %v", lowScoreCodes, want)
	}
}
