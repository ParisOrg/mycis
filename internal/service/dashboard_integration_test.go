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

func TestDashboardGetCountsOnlyOpenUnassignedItems(t *testing.T) {
	h := newIntegrationHarness(t)

	slug := fmt.Sprintf("dashboard-unassigned-%d", time.Now().UnixNano())
	h.writeFrameworkSeed(t, slug, testOrderedFrameworkSeedYAML(slug))
	if err := h.services.Frameworks.SeedFramework(h.ctx, slug, false); err != nil {
		t.Fatal(err)
	}

	admin := h.createAdmin(t)
	owner := h.createUser(t, "Owner", "owner@example.com", db.UserRoleEditor)
	framework := h.onlyFramework(t)

	assessment, err := h.services.Assessments.CreateAssessment(h.ctx, admin, CreateAssessmentInput{
		FrameworkID: framework.ID,
		Name:        "Dashboard Unassigned Assessment",
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

	assigned := findAssessmentItemByCode(t, items, "AA-one")
	if err := h.services.Items.Update(h.ctx, admin, UpdateItemInput{
		ID:          assigned.ID,
		OwnerUserID: &owner.ID,
		Status:      db.AssessmentItemStatusInProgress,
		Priority:    assigned.Priority,
		DueDate:     assigned.DueDate,
	}); err != nil {
		t.Fatal(err)
	}

	openUnassigned := findAssessmentItemByCode(t, items, "ZZ-two")
	if err := h.services.Items.Update(h.ctx, admin, UpdateItemInput{
		ID:       openUnassigned.ID,
		Status:   db.AssessmentItemStatusInProgress,
		Priority: openUnassigned.Priority,
		DueDate:  openUnassigned.DueDate,
	}); err != nil {
		t.Fatal(err)
	}

	completedUnassigned := findAssessmentItemByCode(t, items, "MM-three")
	score := int32(5)
	if err := h.services.Items.Update(h.ctx, admin, UpdateItemInput{
		ID:       completedUnassigned.ID,
		Status:   db.AssessmentItemStatusValidated,
		Score:    &score,
		Priority: completedUnassigned.Priority,
		DueDate:  completedUnassigned.DueDate,
	}); err != nil {
		t.Fatal(err)
	}

	blockedReason := "Waiting on vendor evidence"
	secondOpenUnassigned := findAssessmentItemByCode(t, items, "AA-nine")
	if err := h.services.Items.Update(h.ctx, admin, UpdateItemInput{
		ID:            secondOpenUnassigned.ID,
		Status:        db.AssessmentItemStatusBlocked,
		Priority:      secondOpenUnassigned.Priority,
		DueDate:       secondOpenUnassigned.DueDate,
		BlockedReason: &blockedReason,
	}); err != nil {
		t.Fatal(err)
	}

	dashboard, err := h.services.Dashboard.Get(h.ctx, assessment.ID)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := dashboard.Overview.UnassignedItems, int32(2); got != want {
		t.Fatalf("unexpected unassigned item count: got %d want %d", got, want)
	}
}
