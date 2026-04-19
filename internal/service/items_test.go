package service

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mycis/internal/db"
)

func TestAuthorizeNonAdminItemUpdate(t *testing.T) {
	t.Parallel()

	actorID := uuid.New()
	ownerID := uuid.New()
	reviewerID := uuid.New()

	tests := []struct {
		name     string
		actorID  uuid.UUID
		status   db.AssessmentItemStatus
		owner    *uuid.UUID
		reviewer *uuid.UUID
		wantErr  error
	}{
		{
			name:    "owner can update standard status",
			actorID: ownerID,
			status:  db.AssessmentItemStatusInProgress,
			owner:   &ownerID,
		},
		{
			name:     "reviewer can move to in progress",
			actorID:  reviewerID,
			status:   db.AssessmentItemStatusInProgress,
			reviewer: &reviewerID,
		},
		{
			name:     "reviewer can validate",
			actorID:  reviewerID,
			status:   db.AssessmentItemStatusValidated,
			reviewer: &reviewerID,
		},
		{
			name:    "non collaborator forbidden",
			actorID: actorID,
			status:  db.AssessmentItemStatusInProgress,
			owner:   &ownerID,
			wantErr: ErrForbidden,
		},
		{
			name:     "owner cannot validate",
			actorID:  ownerID,
			status:   db.AssessmentItemStatusValidated,
			owner:    &ownerID,
			reviewer: &reviewerID,
			wantErr:  ErrForbidden,
		},
		{
			name:     "reviewer cannot move to blocked",
			actorID:  reviewerID,
			status:   db.AssessmentItemStatusBlocked,
			owner:    &ownerID,
			reviewer: &reviewerID,
			wantErr:  ErrForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := authorizeNonAdminItemUpdate(tt.actorID, tt.status, tt.owner, tt.reviewer)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got %v want %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateUpdateItemState(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	score := int32(3)
	dueDate := time.Date(2026, time.April, 9, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		input   UpdateItemInput
		wantErr bool
	}{
		{
			name: "ready for review valid",
			input: UpdateItemInput{
				Status:         db.AssessmentItemStatusReadyForReview,
				DueDate:        dueDate,
				OwnerUserID:    &userID,
				ReviewerUserID: &userID,
				Score:          &score,
			},
		},
		{
			name: "ready for review missing score",
			input: UpdateItemInput{
				Status:         db.AssessmentItemStatusReadyForReview,
				DueDate:        dueDate,
				OwnerUserID:    &userID,
				ReviewerUserID: &userID,
			},
			wantErr: true,
		},
		{
			name: "validated missing score",
			input: UpdateItemInput{
				Status:  db.AssessmentItemStatusValidated,
				DueDate: dueDate,
			},
			wantErr: true,
		},
		{
			name: "not applicable missing notes",
			input: UpdateItemInput{
				Status:  db.AssessmentItemStatusNotApplicable,
				DueDate: dueDate,
			},
			wantErr: true,
		},
		{
			name: "blocked missing reason",
			input: UpdateItemInput{
				Status:  db.AssessmentItemStatusBlocked,
				DueDate: dueDate,
			},
			wantErr: true,
		},
		{
			name: "blocked valid",
			input: UpdateItemInput{
				Status:        db.AssessmentItemStatusBlocked,
				DueDate:       dueDate,
				BlockedReason: ptr("dependency"),
			},
		},
		{
			name: "due date required",
			input: UpdateItemInput{
				Status: db.AssessmentItemStatusInProgress,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateUpdateItemState(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if !errors.Is(err, ErrInvalidInput) {
					t.Fatalf("want ErrInvalidInput, got %v", err)
				}
				return
			}

			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRestrictUpdateItemForActor(t *testing.T) {
	t.Parallel()

	ownerID := uuid.New()
	reviewerID := uuid.New()
	actor := db.User{ID: ownerID, Role: db.UserRoleEditor}
	current := db.GetAssessmentItemDetailRow{
		OwnerUserID:    pgtype.UUID{Bytes: ownerID, Valid: true},
		ReviewerUserID: pgtype.UUID{Bytes: reviewerID, Valid: true},
		Priority:       db.ItemPriorityHigh,
		DueDate:        time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
	}
	input := UpdateItemInput{
		Status:         db.AssessmentItemStatusInProgress,
		Priority:       db.ItemPriorityLow,
		DueDate:        time.Date(2026, time.April, 10, 0, 0, 0, 0, time.UTC),
		OwnerUserID:    ptrUUID(uuid.New()),
		ReviewerUserID: ptrUUID(uuid.New()),
	}

	got, err := restrictUpdateItemForActor(actor, current, input)
	if err != nil {
		t.Fatal(err)
	}
	if got.Priority != current.Priority {
		t.Fatalf("priority not preserved: got %s want %s", got.Priority, current.Priority)
	}
	if got.DueDate != current.DueDate {
		t.Fatalf("due date not preserved: got %s want %s", got.DueDate, current.DueDate)
	}
	if got.OwnerUserID == nil || *got.OwnerUserID != ownerID {
		t.Fatalf("owner not preserved: got %v", got.OwnerUserID)
	}
	if got.ReviewerUserID == nil || *got.ReviewerUserID != reviewerID {
		t.Fatalf("reviewer not preserved: got %v", got.ReviewerUserID)
	}
}

func ptr(value string) *string {
	return &value
}

func ptrUUID(value uuid.UUID) *uuid.UUID {
	return &value
}
