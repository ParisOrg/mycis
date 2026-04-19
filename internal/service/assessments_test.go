package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"mycis/internal/db"
)

type fakeAssessmentMutationQueries struct {
	assessment             db.Assessment
	createAssessmentParams db.CreateAssessmentParams
	createControlParams    *db.CreateControlRecordsForAssessmentParams
	copyControlParams      *db.CopyControlRecordsFromPreviousAssessmentParams
	createItemsParams      *db.CreateAssessmentItemsFromControlRecordsParams
	createAuditParams      *db.CreateAuditLogParams
}

func (f *fakeAssessmentMutationQueries) CreateAssessment(_ context.Context, arg db.CreateAssessmentParams) (db.Assessment, error) {
	f.createAssessmentParams = arg
	if f.assessment.ID == uuid.Nil {
		f.assessment = db.Assessment{ID: uuid.New()}
	}
	return f.assessment, nil
}

func (f *fakeAssessmentMutationQueries) CreateControlRecordsForAssessment(_ context.Context, arg db.CreateControlRecordsForAssessmentParams) error {
	f.createControlParams = &arg
	return nil
}

func (f *fakeAssessmentMutationQueries) CopyControlRecordsFromPreviousAssessment(_ context.Context, arg db.CopyControlRecordsFromPreviousAssessmentParams) error {
	f.copyControlParams = &arg
	return nil
}

func (f *fakeAssessmentMutationQueries) CreateAssessmentItemsFromControlRecords(_ context.Context, arg db.CreateAssessmentItemsFromControlRecordsParams) error {
	f.createItemsParams = &arg
	return nil
}

func (f *fakeAssessmentMutationQueries) CreateAuditLog(_ context.Context, arg db.CreateAuditLogParams) error {
	f.createAuditParams = &arg
	return nil
}

func TestValidateAssessmentInput(t *testing.T) {
	t.Parallel()

	startDate := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	dueDate := startDate.Add(24 * time.Hour)

	tests := []struct {
		name    string
		input   assessmentRecordInput
		wantErr bool
	}{
		{
			name: "valid",
			input: assessmentRecordInput{
				Name:      "Q2",
				Scope:     "Production",
				StartDate: startDate,
				DueDate:   dueDate,
			},
		},
		{
			name: "missing name",
			input: assessmentRecordInput{
				Scope:     "Production",
				StartDate: startDate,
				DueDate:   dueDate,
			},
			wantErr: true,
		},
		{
			name: "missing scope",
			input: assessmentRecordInput{
				Name:      "Q2",
				StartDate: startDate,
				DueDate:   dueDate,
			},
			wantErr: true,
		},
		{
			name: "due before start",
			input: assessmentRecordInput{
				Name:      "Q2",
				Scope:     "Production",
				StartDate: dueDate,
				DueDate:   startDate,
			},
			wantErr: true,
		},
		{
			name: "name too long",
			input: assessmentRecordInput{
				Name:      strings.Repeat("a", maxAssessmentNameBytes+1),
				Scope:     "Production",
				StartDate: startDate,
				DueDate:   dueDate,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateAssessmentInput(tt.input)
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

func TestValidateBulkUpdateInput(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	userID := uuid.New()
	itemID := uuid.New()
	priority := db.ItemPriorityHigh

	tests := []struct {
		name    string
		input   BulkUpdateInput
		wantErr bool
	}{
		{
			name: "assign owner",
			input: BulkUpdateInput{
				Action:  bulkActionAssignOwner,
				ItemIDs: []uuid.UUID{itemID},
				UserID:  &userID,
			},
		},
		{
			name: "missing items",
			input: BulkUpdateInput{
				Action: bulkActionAssignOwner,
				UserID: &userID,
			},
			wantErr: true,
		},
		{
			name: "missing owner",
			input: BulkUpdateInput{
				Action:  bulkActionAssignOwner,
				ItemIDs: []uuid.UUID{itemID},
			},
			wantErr: true,
		},
		{
			name: "missing reviewer",
			input: BulkUpdateInput{
				Action:  bulkActionAssignReviewer,
				ItemIDs: []uuid.UUID{itemID},
			},
			wantErr: true,
		},
		{
			name: "missing due date",
			input: BulkUpdateInput{
				Action:  bulkActionSetDueDate,
				ItemIDs: []uuid.UUID{itemID},
			},
			wantErr: true,
		},
		{
			name: "missing priority",
			input: BulkUpdateInput{
				Action:  bulkActionSetPriority,
				ItemIDs: []uuid.UUID{itemID},
			},
			wantErr: true,
		},
		{
			name: "set due date",
			input: BulkUpdateInput{
				Action:  bulkActionSetDueDate,
				ItemIDs: []uuid.UUID{itemID},
				DueDate: &now,
			},
		},
		{
			name: "set priority",
			input: BulkUpdateInput{
				Action:   bulkActionSetPriority,
				ItemIDs:  []uuid.UUID{itemID},
				Priority: &priority,
			},
		},
		{
			name: "unsupported action",
			input: BulkUpdateInput{
				Action:  "archive",
				ItemIDs: []uuid.UUID{itemID},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateBulkUpdateInput(tt.input)
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

func TestCreateAssessmentWithFrameworkTxCreatesFreshControlRecords(t *testing.T) {
	t.Parallel()

	actor := db.User{ID: uuid.New(), Role: db.UserRoleAdmin}
	input := assessmentRecordInput{
		FrameworkID: uuid.New(),
		Name:        "Q2",
		Scope:       "Production",
		StartDate:   time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		DueDate:     time.Date(2026, time.April, 30, 0, 0, 0, 0, time.UTC),
	}
	q := &fakeAssessmentMutationQueries{}

	assessment, err := createAssessmentWithFrameworkTx(context.Background(), q, actor, input, nil, "assessment_created", map[string]any{"name": input.Name})
	if err != nil {
		t.Fatal(err)
	}
	if q.createControlParams == nil {
		t.Fatal("expected fresh control records to be created")
	}
	if q.createControlParams.AssessmentID != assessment.ID {
		t.Fatalf("unexpected assessment id: %v", q.createControlParams.AssessmentID)
	}
	if q.createControlParams.FrameworkID != input.FrameworkID {
		t.Fatalf("unexpected framework id: %v", q.createControlParams.FrameworkID)
	}
	if q.copyControlParams != nil {
		t.Fatal("did not expect previous control records to be copied")
	}
	if q.createItemsParams == nil {
		t.Fatal("expected assessment items to be created")
	}
	if q.createItemsParams.AssessmentID != assessment.ID {
		t.Fatalf("unexpected item assessment id: %v", q.createItemsParams.AssessmentID)
	}
	if q.createItemsParams.DueDate != input.DueDate {
		t.Fatalf("unexpected due date: %s", q.createItemsParams.DueDate)
	}
}

func TestCreateAssessmentWithFrameworkTxCopiesCycleAssignments(t *testing.T) {
	t.Parallel()

	actor := db.User{ID: uuid.New(), Role: db.UserRoleAdmin}
	previousAssessmentID := uuid.New()
	input := assessmentRecordInput{
		FrameworkID: uuid.New(),
		Name:        "Q3",
		Scope:       "Production",
		StartDate:   time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC),
		DueDate:     time.Date(2026, time.May, 31, 0, 0, 0, 0, time.UTC),
	}
	q := &fakeAssessmentMutationQueries{}

	assessment, err := createAssessmentWithFrameworkTx(context.Background(), q, actor, input, &previousAssessmentID, "cycle_created", map[string]any{"name": input.Name})
	if err != nil {
		t.Fatal(err)
	}
	if q.copyControlParams == nil {
		t.Fatal("expected previous control records to be copied")
	}
	if q.copyControlParams.AssessmentID != assessment.ID {
		t.Fatalf("unexpected assessment id: %v", q.copyControlParams.AssessmentID)
	}
	if q.copyControlParams.PreviousAssessmentID != previousAssessmentID {
		t.Fatalf("unexpected previous assessment id: %v", q.copyControlParams.PreviousAssessmentID)
	}
	if q.createControlParams != nil {
		t.Fatal("did not expect fresh framework control record creation")
	}
	if q.createItemsParams == nil {
		t.Fatal("expected assessment items to be created")
	}
}
