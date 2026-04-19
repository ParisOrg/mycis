package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mycis/internal/db"
)

const (
	bulkActionAssignOwner    = "assign_owner"
	bulkActionAssignReviewer = "assign_reviewer"
	bulkActionSetDueDate     = "set_due_date"
	bulkActionSetPriority    = "set_priority"
)

type CreateAssessmentInput struct {
	FrameworkID uuid.UUID
	Name        string
	Scope       string
	StartDate   time.Time
	DueDate     time.Time
}

type AssessmentItemFilters struct {
	GroupCode      *string
	Tag            *string
	Status         *string
	OwnerUserID    *string
	ReviewerUserID *string
	Unassigned     *bool
	Overdue        *bool
}

type BulkUpdateInput struct {
	AssessmentID uuid.UUID
	ItemIDs      []uuid.UUID
	Action       string
	UserID       *uuid.UUID
	DueDate      *time.Time
	Priority     *db.ItemPriority
}

type AssessmentService struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

type assessmentMutationQueries interface {
	CreateAssessment(ctx context.Context, arg db.CreateAssessmentParams) (db.Assessment, error)
	CreateControlRecordsForAssessment(ctx context.Context, arg db.CreateControlRecordsForAssessmentParams) error
	CopyControlRecordsFromPreviousAssessment(ctx context.Context, arg db.CopyControlRecordsFromPreviousAssessmentParams) error
	CreateAssessmentItemsFromControlRecords(ctx context.Context, arg db.CreateAssessmentItemsFromControlRecordsParams) error
	CreateAuditLog(ctx context.Context, arg db.CreateAuditLogParams) error
}

type assessmentRecordInput struct {
	FrameworkID uuid.UUID
	Name        string
	Scope       string
	StartDate   time.Time
	DueDate     time.Time
}

func (s *AssessmentService) ListAssessments(ctx context.Context) ([]db.ListAssessmentsRow, error) {
	return s.queries.ListAssessments(ctx)
}

func (s *AssessmentService) GetAssessment(ctx context.Context, assessmentID string) (db.GetAssessmentByIDRow, error) {
	id, err := uuidFromString(assessmentID)
	if err != nil {
		return db.GetAssessmentByIDRow{}, err
	}

	row, err := s.queries.GetAssessmentByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.GetAssessmentByIDRow{}, ErrNotFound
		}
		return db.GetAssessmentByIDRow{}, fmt.Errorf("get assessment: %w", err)
	}
	return row, nil
}

func (s *AssessmentService) CreateAssessment(ctx context.Context, actor db.User, input CreateAssessmentInput) (db.Assessment, error) {
	if !actor.CanManageAssessments() {
		return db.Assessment{}, ErrForbidden
	}
	recordInput := assessmentRecordInput{
		FrameworkID: input.FrameworkID,
		Name:        input.Name,
		Scope:       input.Scope,
		StartDate:   input.StartDate,
		DueDate:     input.DueDate,
	}
	if err := validateAssessmentInput(recordInput); err != nil {
		return db.Assessment{}, err
	}

	return s.createAssessmentWithFramework(ctx, actor, recordInput, nil, "assessment_created", map[string]any{
		"name":  input.Name,
		"scope": input.Scope,
	})
}

func (s *AssessmentService) ListAssessmentItems(ctx context.Context, assessmentID string, filters AssessmentItemFilters) ([]db.ListAssessmentItemsRow, error) {
	id, err := uuidFromString(assessmentID)
	if err != nil {
		return nil, err
	}
	return s.queries.ListAssessmentItems(ctx, db.ListAssessmentItemsParams{
		AssessmentID:   id,
		GroupCode:      filters.GroupCode,
		Tag:            filters.Tag,
		Status:         filters.Status,
		OwnerUserID:    filters.OwnerUserID,
		ReviewerUserID: filters.ReviewerUserID,
		Unassigned:     filters.Unassigned,
		Overdue:        filters.Overdue,
	})
}

func (s *AssessmentService) BulkUpdateItems(ctx context.Context, actor db.User, input BulkUpdateInput) error {
	if !actor.CanManageAssessments() {
		return ErrForbidden
	}
	if err := validateBulkUpdateInput(input); err != nil {
		return err
	}

	_, err := withTx(ctx, s.pool, func(q *db.Queries) (struct{}, error) {
		if err := applyBulkUpdateAction(ctx, q, actor, input); err != nil {
			return struct{}{}, err
		}
		if err := createBulkUpdateAuditLogs(ctx, q, actor.ID, input.ItemIDs, input.Action); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	})
	return err
}

type CreateCycleInput struct {
	PreviousAssessmentID uuid.UUID
	Name                 string
	Scope                string
	StartDate            time.Time
	DueDate              time.Time
}

func (s *AssessmentService) CreateCycleFromPrevious(ctx context.Context, actor db.User, input CreateCycleInput) (db.Assessment, error) {
	if !actor.CanManageAssessments() {
		return db.Assessment{}, ErrForbidden
	}
	recordInput := assessmentRecordInput{
		Name:      input.Name,
		Scope:     input.Scope,
		StartDate: input.StartDate,
		DueDate:   input.DueDate,
	}
	if err := validateAssessmentInput(recordInput); err != nil {
		return db.Assessment{}, err
	}

	return withTx(ctx, s.pool, func(q *db.Queries) (db.Assessment, error) {
		prev, err := q.GetAssessmentByID(ctx, input.PreviousAssessmentID)
		if err != nil {
			return db.Assessment{}, fmt.Errorf("get previous assessment: %w", err)
		}

		recordInput.FrameworkID = prev.FrameworkID
		return createAssessmentWithFrameworkTx(ctx, q, actor, recordInput, &input.PreviousAssessmentID, "cycle_created", map[string]any{
			"name":                   input.Name,
			"scope":                  input.Scope,
			"previous_assessment_id": input.PreviousAssessmentID.String(),
		})
	})
}

func validateAssessmentInput(input assessmentRecordInput) error {
	if input.Name == "" || input.Scope == "" {
		return fmt.Errorf("%w: name and scope are required", ErrInvalidInput)
	}
	if err := errIfTooLong(input.Name, maxAssessmentNameBytes, "assessment name"); err != nil {
		return err
	}
	if err := errIfTooLong(input.Scope, maxAssessmentScopeBytes, "scope"); err != nil {
		return err
	}
	if input.DueDate.Before(input.StartDate) {
		return fmt.Errorf("%w: due date must be after start date", ErrInvalidInput)
	}
	return nil
}

func (s *AssessmentService) createAssessmentWithFramework(ctx context.Context, actor db.User, input assessmentRecordInput, previousAssessmentID *uuid.UUID, action string, payload map[string]any) (db.Assessment, error) {
	return withTx(ctx, s.pool, func(q *db.Queries) (db.Assessment, error) {
		return createAssessmentWithFrameworkTx(ctx, q, actor, input, previousAssessmentID, action, payload)
	})
}

func createAssessmentWithFrameworkTx(ctx context.Context, q assessmentMutationQueries, actor db.User, input assessmentRecordInput, previousAssessmentID *uuid.UUID, action string, payload map[string]any) (db.Assessment, error) {
	assessment, err := q.CreateAssessment(ctx, db.CreateAssessmentParams{
		FrameworkID: input.FrameworkID,
		Name:        input.Name,
		Scope:       input.Scope,
		StartDate:   input.StartDate,
		DueDate:     input.DueDate,
		Status:      db.AssessmentStatusActive,
		CreatedBy:   actor.ID,
	})
	if err != nil {
		return db.Assessment{}, fmt.Errorf("create assessment: %w", err)
	}

	if previousAssessmentID == nil {
		if err := q.CreateControlRecordsForAssessment(ctx, db.CreateControlRecordsForAssessmentParams{
			AssessmentID: assessment.ID,
			FrameworkID:  input.FrameworkID,
		}); err != nil {
			return db.Assessment{}, fmt.Errorf("create control records: %w", err)
		}
	} else {
		if err := q.CopyControlRecordsFromPreviousAssessment(ctx, db.CopyControlRecordsFromPreviousAssessmentParams{
			AssessmentID:         assessment.ID,
			PreviousAssessmentID: *previousAssessmentID,
		}); err != nil {
			return db.Assessment{}, fmt.Errorf("copy control records: %w", err)
		}
	}

	if err := q.CreateAssessmentItemsFromControlRecords(ctx, db.CreateAssessmentItemsFromControlRecordsParams{
		AssessmentID: assessment.ID,
		DueDate:      input.DueDate,
		UpdatedBy:    actor.ID,
	}); err != nil {
		return db.Assessment{}, fmt.Errorf("create assessment items: %w", err)
	}

	if err := q.CreateAuditLog(ctx, db.CreateAuditLogParams{
		EntityType:  "assessment",
		EntityID:    assessment.ID,
		Action:      action,
		ActorUserID: actor.ID,
		PayloadJson: auditPayload(payload),
	}); err != nil {
		return db.Assessment{}, fmt.Errorf("create assessment audit: %w", err)
	}

	return assessment, nil
}

func validateBulkUpdateInput(input BulkUpdateInput) error {
	if len(input.ItemIDs) == 0 {
		return fmt.Errorf("%w: select at least one item", ErrInvalidInput)
	}

	switch input.Action {
	case bulkActionAssignOwner:
		if input.UserID == nil {
			return fmt.Errorf("%w: owner is required", ErrInvalidInput)
		}
	case bulkActionAssignReviewer:
		if input.UserID == nil {
			return fmt.Errorf("%w: reviewer is required", ErrInvalidInput)
		}
	case bulkActionSetDueDate:
		if input.DueDate == nil {
			return fmt.Errorf("%w: due date is required", ErrInvalidInput)
		}
	case bulkActionSetPriority:
		if input.Priority == nil {
			return fmt.Errorf("%w: priority is required", ErrInvalidInput)
		}
	default:
		return fmt.Errorf("%w: unsupported bulk action", ErrInvalidInput)
	}

	return nil
}

func applyBulkUpdateAction(ctx context.Context, q *db.Queries, actor db.User, input BulkUpdateInput) error {
	switch input.Action {
	case bulkActionAssignOwner:
		if err := validateAssignableUser(ctx, q, input.UserID, "owner"); err != nil {
			return err
		}
		return applyBulkAssignOwner(ctx, q, input)
	case bulkActionAssignReviewer:
		if err := validateAssignableUser(ctx, q, input.UserID, "reviewer"); err != nil {
			return err
		}
		return applyBulkAssignReviewer(ctx, q, input)
	case bulkActionSetDueDate:
		return applyBulkSetDueDate(ctx, q, actor.ID, input)
	case bulkActionSetPriority:
		return applyBulkSetPriority(ctx, q, actor.ID, input)
	default:
		return fmt.Errorf("%w: unsupported bulk action", ErrInvalidInput)
	}
}

func applyBulkAssignOwner(ctx context.Context, q *db.Queries, input BulkUpdateInput) error {
	if err := q.BulkAssignControlRecordOwner(ctx, db.BulkAssignControlRecordOwnerParams{
		Column1:      input.ItemIDs,
		OwnerUserID:  pgUUIDFromPtr(input.UserID),
		AssessmentID: input.AssessmentID,
	}); err != nil {
		return fmt.Errorf("bulk assign owner: %w", err)
	}
	return nil
}

func applyBulkAssignReviewer(ctx context.Context, q *db.Queries, input BulkUpdateInput) error {
	if err := q.BulkAssignControlRecordReviewer(ctx, db.BulkAssignControlRecordReviewerParams{
		Column1:        input.ItemIDs,
		ReviewerUserID: pgUUIDFromPtr(input.UserID),
		AssessmentID:   input.AssessmentID,
	}); err != nil {
		return fmt.Errorf("bulk assign reviewer: %w", err)
	}
	return nil
}

func applyBulkSetDueDate(ctx context.Context, q *db.Queries, actorID uuid.UUID, input BulkUpdateInput) error {
	if err := q.BulkSetDueDate(ctx, db.BulkSetDueDateParams{
		AssessmentID: input.AssessmentID,
		Column2:      input.ItemIDs,
		DueDate:      *input.DueDate,
		UpdatedBy:    pgUUID(actorID),
	}); err != nil {
		return fmt.Errorf("bulk set due date: %w", err)
	}
	return nil
}

func applyBulkSetPriority(ctx context.Context, q *db.Queries, actorID uuid.UUID, input BulkUpdateInput) error {
	if err := q.BulkSetPriority(ctx, db.BulkSetPriorityParams{
		AssessmentID: input.AssessmentID,
		Column2:      input.ItemIDs,
		Priority:     *input.Priority,
		UpdatedBy:    pgUUID(actorID),
	}); err != nil {
		return fmt.Errorf("bulk set priority: %w", err)
	}
	return nil
}

func createBulkUpdateAuditLogs(ctx context.Context, q *db.Queries, actorID uuid.UUID, itemIDs []uuid.UUID, action string) error {
	for _, itemID := range itemIDs {
		if err := q.CreateAuditLog(ctx, db.CreateAuditLogParams{
			EntityType:  "assessment_item",
			EntityID:    itemID,
			Action:      "bulk_" + action,
			ActorUserID: actorID,
			PayloadJson: auditPayload(map[string]any{"action": action}),
		}); err != nil {
			return fmt.Errorf("create bulk audit: %w", err)
		}
	}
	return nil
}
