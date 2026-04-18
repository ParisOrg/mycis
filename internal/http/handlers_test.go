package httpui

import (
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"

	"mycis/internal/db"
)

func TestBuildGroupOptionsPreservesFirstSeenOrder(t *testing.T) {
	items := []db.ListAssessmentItemsRow{
		{GroupCode: "RS", GroupTitle: "Respond", GroupSortOrder: 4},
		{GroupCode: "GV", GroupTitle: "Govern", GroupSortOrder: 1},
		{GroupCode: "RS", GroupTitle: "Respond", GroupSortOrder: 4},
		{GroupCode: "ID", GroupTitle: "Identify", GroupSortOrder: 2},
	}

	groups := buildGroupOptions(items)
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}

	got := []string{groups[0].Code, groups[1].Code, groups[2].Code}
	want := []string{"RS", "GV", "ID"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected group order: got %v want %v", got, want)
		}
	}
}

func TestBuildTagOptionsDedupesInFirstSeenItemOrder(t *testing.T) {
	items := []db.ListAssessmentItemsRow{
		{Tags: []string{"govern", "shared"}},
		{Tags: []string{"identify", "shared"}},
		{Tags: []string{"respond"}},
	}

	tags := buildTagOptions(items)
	want := []string{"govern", "shared", "identify", "respond"}
	if len(tags) != len(want) {
		t.Fatalf("unexpected tag count: got %d want %d", len(tags), len(want))
	}
	for i := range want {
		if tags[i] != want[i] {
			t.Fatalf("unexpected tag order: got %v want %v", tags, want)
		}
	}
}

func TestReadItemUpdateInputAdminRequiresValidDueDate(t *testing.T) {
	itemID := uuid.New()
	form := url.Values{
		"status":   {"in_progress"},
		"priority": {"high"},
		"due_date": {""},
	}

	_, err := readItemUpdateInput(form, itemID, true)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "Enter a valid due date." {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadItemUpdateInputAdminParsesDueDate(t *testing.T) {
	itemID := uuid.New()
	form := url.Values{
		"status":           {"validated"},
		"priority":         {"critical"},
		"due_date":         {"2026-04-09"},
		"owner_user_id":    {uuid.New().String()},
		"reviewer_user_id": {uuid.New().String()},
		"score":            {"4"},
		"notes":            {"  documented  "},
	}

	input, err := readItemUpdateInput(form, itemID, true)
	if err != nil {
		t.Fatal(err)
	}
	if input.ID != itemID {
		t.Fatalf("unexpected item id: %v", input.ID)
	}
	if input.Status != db.AssessmentItemStatusValidated {
		t.Fatalf("unexpected status: %s", input.Status)
	}
	if input.Priority != db.ItemPriorityCritical {
		t.Fatalf("unexpected priority: %s", input.Priority)
	}
	wantDueDate := time.Date(2026, time.April, 9, 0, 0, 0, 0, time.UTC)
	if input.DueDate != wantDueDate {
		t.Fatalf("unexpected due date: got %s want %s", input.DueDate, wantDueDate)
	}
	if input.Score == nil || *input.Score != 4 {
		t.Fatalf("unexpected score: %v", input.Score)
	}
	if input.Notes == nil || *input.Notes != "documented" {
		t.Fatalf("unexpected notes: %v", input.Notes)
	}
}

func TestReadItemUpdateInputNonAdminIgnoresDueDate(t *testing.T) {
	itemID := uuid.New()
	form := url.Values{
		"status":   {"in_progress"},
		"priority": {"medium"},
		"due_date": {""},
	}

	input, err := readItemUpdateInput(form, itemID, false)
	if err != nil {
		t.Fatal(err)
	}
	if !input.DueDate.IsZero() {
		t.Fatalf("expected zero due date, got %s", input.DueDate)
	}
}
