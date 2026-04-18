package service

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"mycis/internal/db"
)

func TestDashboardGetPreservesFrameworkSortOrder(t *testing.T) {
	h := newIntegrationHarness(t)

	slug := fmt.Sprintf("dashboard-order-%d", time.Now().UnixNano())
	h.writeFrameworkSeed(t, slug, testOrderedFrameworkSeedYAML(slug))
	if err := h.services.Frameworks.SeedFramework(h.ctx, slug, false); err != nil {
		t.Fatal(err)
	}

	admin := h.createAdmin(t)
	framework := h.onlyFramework(t)

	assessment, err := h.services.Assessments.CreateAssessment(h.ctx, admin, CreateAssessmentInput{
		FrameworkID: framework.ID,
		Name:        "Ordered Dashboard Assessment",
		Scope:       "Production",
		StartDate:   time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		DueDate:     time.Date(2026, time.April, 30, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}

	items, err := h.services.Assessments.ListAssessmentItems(h.ctx, assessment.ID.String(), AssessmentItemFilters{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := assessmentItemCodes(items), []string{"ZZ-two", "AA-one", "MM-three", "AA-nine"}; !slices.Equal(got, want) {
		t.Fatalf("unexpected seeded assessment item order: got %v want %v", got, want)
	}

	overdueDate := time.Now().UTC().AddDate(0, 0, -7)
	lowScore := int32(2)
	for _, item := range items {
		if err := h.services.Items.Update(h.ctx, admin, UpdateItemInput{
			ID:       item.ID,
			Status:   db.AssessmentItemStatusInProgress,
			Score:    &lowScore,
			Priority: item.Priority,
			DueDate:  overdueDate,
		}); err != nil {
			t.Fatal(err)
		}
	}

	dashboard, err := h.services.Dashboard.Get(h.ctx, assessment.ID)
	if err != nil {
		t.Fatal(err)
	}

	groupCodes := make([]string, 0, len(dashboard.ByGroup))
	for _, group := range dashboard.ByGroup {
		groupCodes = append(groupCodes, group.GroupCode)
	}
	if want := []string{"ZZ", "AA", "MM"}; !slices.Equal(groupCodes, want) {
		t.Fatalf("unexpected dashboard group order: got %v want %v", groupCodes, want)
	}

	overdueCodes := make([]string, 0, len(dashboard.Overdue))
	for _, item := range dashboard.Overdue {
		overdueCodes = append(overdueCodes, item.ItemCode)
	}
	if want := []string{"ZZ-two", "AA-one", "AA-nine", "MM-three"}; !slices.Equal(overdueCodes, want) {
		t.Fatalf("unexpected overdue item order: got %v want %v", overdueCodes, want)
	}

	lowScoreCodes := make([]string, 0, len(dashboard.LowScore))
	for _, item := range dashboard.LowScore {
		lowScoreCodes = append(lowScoreCodes, item.ItemCode)
	}
	if want := []string{"ZZ-two", "AA-one", "AA-nine", "MM-three"}; !slices.Equal(lowScoreCodes, want) {
		t.Fatalf("unexpected low score item order: got %v want %v", lowScoreCodes, want)
	}
}
