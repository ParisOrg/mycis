package httpui

import (
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mycis/internal/db"
)

func TestBuildGroupOptionsUsesFrameworkSortOrder(t *testing.T) {
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
	want := []string{"GV", "ID", "RS"}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected group order: got %v want %v", got, want)
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
	if !slices.Equal(tags, want) {
		t.Fatalf("unexpected tag order: got %v want %v", tags, want)
	}
}

func TestBuildAssessmentWorkspaceStatsCountsOnlyOpenOwnerlessItems(t *testing.T) {
	t.Parallel()

	blankOwnerName := ""
	allItems := make([]db.ListAssessmentItemsRow, 5)
	visibleItems := []db.ListAssessmentItemsRow{
		{
			Status:      db.AssessmentItemStatusInProgress,
			OwnerUserID: uuidToPG(uuid.New()),
			OwnerName:   &blankOwnerName,
		},
		{
			Status: db.AssessmentItemStatusValidated,
		},
		{
			Status: db.AssessmentItemStatusBlocked,
		},
	}

	stats := buildAssessmentWorkspaceStats(allItems, visibleItems)

	if got, want := stats.TotalItems, len(allItems); got != want {
		t.Fatalf("unexpected total items: got %d want %d", got, want)
	}
	if got, want := stats.VisibleItems, len(visibleItems); got != want {
		t.Fatalf("unexpected visible items: got %d want %d", got, want)
	}
	if got, want := stats.UnassignedItems, 1; got != want {
		t.Fatalf("unexpected unassigned item count: got %d want %d", got, want)
	}
}

func TestReadItemUpdateInputAdminRequiresValidDueDate(t *testing.T) {
	srv := &Server{}
	itemID := uuid.New()
	form := url.Values{
		"status":   {"in_progress"},
		"priority": {"high"},
		"due_date": {""},
	}

	_, err := srv.readItemUpdateInput(form, itemID, true)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "Enter a valid due date." {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadItemUpdateInputAdminParsesDueDate(t *testing.T) {
	srv := &Server{}
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

	input, err := srv.readItemUpdateInput(form, itemID, true)
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
	srv := &Server{}
	itemID := uuid.New()
	form := url.Values{
		"status":   {"in_progress"},
		"priority": {"medium"},
		"due_date": {""},
	}

	input, err := srv.readItemUpdateInput(form, itemID, false)
	if err != nil {
		t.Fatal(err)
	}
	if !input.DueDate.IsZero() {
		t.Fatalf("expected zero due date, got %s", input.DueDate)
	}
}

func TestReadItemUpdateInputNonAdminAllowsMissingPriority(t *testing.T) {
	srv := &Server{}
	itemID := uuid.New()
	form := url.Values{
		"status": {"in_progress"},
		"score":  {"3"},
		"notes":  {"updated"},
	}

	input, err := srv.readItemUpdateInput(form, itemID, false)
	if err != nil {
		t.Fatal(err)
	}
	if input.Score == nil || *input.Score != 3 {
		t.Fatalf("unexpected score: %v", input.Score)
	}
	if input.Notes == nil || *input.Notes != "updated" {
		t.Fatalf("unexpected notes: %v", input.Notes)
	}
}

func TestCanEditItem(t *testing.T) {
	t.Parallel()

	ownerID := uuid.New()
	reviewerID := uuid.New()
	otherID := uuid.New()
	item := db.GetAssessmentItemDetailRow{
		OwnerUserID:    uuidToPG(ownerID),
		ReviewerUserID: uuidToPG(reviewerID),
	}

	tests := []struct {
		name string
		user *db.User
		want bool
	}{
		{
			name: "admin",
			user: &db.User{ID: otherID, Role: db.UserRoleAdmin},
			want: true,
		},
		{
			name: "assessment manager",
			user: &db.User{ID: otherID, Role: db.UserRoleAssessmentManager},
			want: true,
		},
		{
			name: "assigned editor owner",
			user: &db.User{ID: ownerID, Role: db.UserRoleEditor},
			want: true,
		},
		{
			name: "assigned editor reviewer",
			user: &db.User{ID: reviewerID, Role: db.UserRoleEditor},
			want: true,
		},
		{
			name: "unassigned editor",
			user: &db.User{ID: otherID, Role: db.UserRoleEditor},
		},
		{
			name: "viewer",
			user: &db.User{ID: ownerID, Role: db.UserRoleViewer},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := canEditItem(tt.user, item); got != tt.want {
				t.Fatalf("canEditItem() = %t, want %t", got, tt.want)
			}
		})
	}
}

func uuidToPG(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}
