package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mycis/internal/db"
)

type UpdateItemInput struct {
	ID             uuid.UUID
	OwnerUserID    *uuid.UUID
	ReviewerUserID *uuid.UUID
	Status         db.AssessmentItemStatus
	Score          *int32
	Priority       db.ItemPriority
	DueDate        time.Time
	Notes          *string
	BlockedReason  *string
}

type ItemDetail struct {
	Item     db.GetAssessmentItemDetailRow
	Comments []db.ListCommentsByControlRecordRow
	Evidence []db.ListEvidenceLinksByControlRecordRow
	Audit    []db.ListAuditLogByEntityRow
}

type AssessmentItemService struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func (s *AssessmentItemService) GetDetail(ctx context.Context, itemID string) (ItemDetail, error) {
	id, err := uuidFromString(itemID)
	if err != nil {
		return ItemDetail{}, err
	}

	item, err := s.queries.GetAssessmentItemDetail(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ItemDetail{}, ErrNotFound
		}
		return ItemDetail{}, fmt.Errorf("get item detail: %w", err)
	}

	comments, err := s.queries.ListCommentsByControlRecord(ctx, item.ControlRecordID)
	if err != nil {
		return ItemDetail{}, fmt.Errorf("list comments: %w", err)
	}
	evidence, err := s.queries.ListEvidenceLinksByControlRecord(ctx, item.ControlRecordID)
	if err != nil {
		return ItemDetail{}, fmt.Errorf("list evidence: %w", err)
	}
	audit, err := s.queries.ListAuditLogByEntity(ctx, db.ListAuditLogByEntityParams{
		EntityType: "assessment_item",
		EntityID:   item.ID,
	})
	if err != nil {
		return ItemDetail{}, fmt.Errorf("list audit log: %w", err)
	}

	return ItemDetail{
		Item:     item,
		Comments: comments,
		Evidence: evidence,
		Audit:    audit,
	}, nil
}

func (s *AssessmentItemService) Update(ctx context.Context, actor db.User, input UpdateItemInput) error {
	if err := validateUpdateItemInput(input); err != nil {
		return err
	}

	_, err := withTx(ctx, s.pool, func(q *db.Queries) (struct{}, error) {
		current, err := q.GetAssessmentItemDetail(ctx, input.ID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return struct{}{}, ErrNotFound
			}
			return struct{}{}, fmt.Errorf("get current item: %w", err)
		}

		input, err = restrictUpdateItemForActor(actor, current, input)
		if err != nil {
			return struct{}{}, err
		}
		if actor.CanManageAssessments() {
			if err := validateAssignableUser(ctx, q, input.OwnerUserID, "owner"); err != nil {
				return struct{}{}, err
			}
			if err := validateAssignableUser(ctx, q, input.ReviewerUserID, "reviewer"); err != nil {
				return struct{}{}, err
			}
		}

		if err := validateUpdateItemState(input); err != nil {
			return struct{}{}, err
		}

		if err := updateAssessmentItemRecord(ctx, q, actor.ID, input); err != nil {
			return struct{}{}, err
		}

		if err := syncControlRecordFields(ctx, q, actor, current, input); err != nil {
			return struct{}{}, err
		}

		if err := q.CreateAuditLog(ctx, db.CreateAuditLogParams{
			EntityType:  "assessment_item",
			EntityID:    input.ID,
			Action:      "item_updated",
			ActorUserID: actor.ID,
			PayloadJson: buildItemUpdateAuditPayload(input),
		}); err != nil {
			return struct{}{}, fmt.Errorf("create update audit: %w", err)
		}

		return struct{}{}, nil
	})
	return err
}

func validateUpdateItemInput(input UpdateItemInput) error {
	if input.Notes != nil {
		if err := errIfTooLong(*input.Notes, maxNotesBytes, "notes"); err != nil {
			return err
		}
	}
	if input.BlockedReason != nil {
		if err := errIfTooLong(*input.BlockedReason, maxBlockedReasonBytes, "blocked reason"); err != nil {
			return err
		}
	}
	return nil
}

func restrictUpdateItemForActor(actor db.User, current db.GetAssessmentItemDetailRow, input UpdateItemInput) (UpdateItemInput, error) {
	if actor.CanManageAssessments() {
		return input, nil
	}
	if !actor.CanEditAssignedItems() {
		return UpdateItemInput{}, ErrForbidden
	}

	owner := ptrUUIDFromPG(current.OwnerUserID)
	reviewer := ptrUUIDFromPG(current.ReviewerUserID)
	if err := authorizeNonAdminItemUpdate(actor.ID, input.Status, owner, reviewer); err != nil {
		return UpdateItemInput{}, err
	}

	input.OwnerUserID = owner
	input.ReviewerUserID = reviewer
	input.Priority = current.Priority
	input.DueDate = current.DueDate
	return input, nil
}

func authorizeNonAdminItemUpdate(actorID uuid.UUID, status db.AssessmentItemStatus, owner, reviewer *uuid.UUID) error {
	isOwner := owner != nil && *owner == actorID
	isReviewer := reviewer != nil && *reviewer == actorID

	switch status {
	case db.AssessmentItemStatusValidated:
		if !isReviewer {
			return ErrForbidden
		}
	default:
		if !isOwner && !(isReviewer && status == db.AssessmentItemStatusInProgress) {
			return ErrForbidden
		}
	}

	return nil
}

func validateUpdateItemState(input UpdateItemInput) error {
	if input.DueDate.IsZero() {
		return fmt.Errorf("%w: due date is required", ErrInvalidInput)
	}
	if input.Status == db.AssessmentItemStatusReadyForReview {
		if input.OwnerUserID == nil || input.ReviewerUserID == nil || input.Score == nil {
			return fmt.Errorf("%w: ready for review requires owner, reviewer, and score", ErrInvalidInput)
		}
	}
	if input.Status == db.AssessmentItemStatusValidated {
		if input.Score == nil {
			return fmt.Errorf("%w: validated items require score", ErrInvalidInput)
		}
	}
	if input.Status == db.AssessmentItemStatusNotApplicable && input.Notes == nil {
		return fmt.Errorf("%w: notes are required for not applicable", ErrInvalidInput)
	}
	if input.Status == db.AssessmentItemStatusBlocked && input.BlockedReason == nil {
		return fmt.Errorf("%w: blocked reason is required", ErrInvalidInput)
	}
	return nil
}

func updateValidationMetadata(status db.AssessmentItemStatus, actorID uuid.UUID) (*time.Time, *uuid.UUID) {
	if status != db.AssessmentItemStatusValidated {
		return nil, nil
	}

	now := time.Now().UTC()
	return &now, &actorID
}

func updateAssessmentItemRecord(ctx context.Context, q *db.Queries, actorID uuid.UUID, input UpdateItemInput) error {
	validatedAt, validatedBy := updateValidationMetadata(input.Status, actorID)
	_, err := q.UpdateAssessmentItem(ctx, db.UpdateAssessmentItemParams{
		ID:            input.ID,
		Status:        input.Status,
		Priority:      input.Priority,
		DueDate:       input.DueDate,
		UpdatedBy:     pgUUID(actorID),
		Score:         input.Score,
		BlockedReason: input.BlockedReason,
		ValidatedAt:   pgTimestamp(validatedAt),
		ValidatedBy:   pgUUIDFromPtr(validatedBy),
	})
	if err != nil {
		return fmt.Errorf("update item: %w", err)
	}
	return nil
}

func syncControlRecordFields(ctx context.Context, q *db.Queries, actor db.User, current db.GetAssessmentItemDetailRow, input UpdateItemInput) error {
	controlRecordID := current.ControlRecordID

	if actor.CanManageAssessments() {
		if err := updateControlRecordAssignments(ctx, q, controlRecordID, input.OwnerUserID, input.ReviewerUserID); err != nil {
			return err
		}
	}
	notes := ""
	if input.Notes != nil {
		notes = *input.Notes
	}
	if err := updateControlRecordNotes(ctx, q, controlRecordID, input.Status, notes); err != nil {
		return err
	}
	return nil
}

func updateControlRecordAssignments(ctx context.Context, q *db.Queries, controlRecordID uuid.UUID, ownerUserID, reviewerUserID *uuid.UUID) error {
	if err := q.UpdateControlRecordOwner(ctx, db.UpdateControlRecordOwnerParams{
		ID:          controlRecordID,
		OwnerUserID: pgUUIDFromPtr(ownerUserID),
	}); err != nil {
		return fmt.Errorf("update control record owner: %w", err)
	}
	if err := q.UpdateControlRecordReviewer(ctx, db.UpdateControlRecordReviewerParams{
		ID:             controlRecordID,
		ReviewerUserID: pgUUIDFromPtr(reviewerUserID),
	}); err != nil {
		return fmt.Errorf("update control record reviewer: %w", err)
	}
	return nil
}

func updateControlRecordNotes(ctx context.Context, q *db.Queries, controlRecordID uuid.UUID, status db.AssessmentItemStatus, notes string) error {
	if err := q.UpdateControlRecordNotes(ctx, db.UpdateControlRecordNotesParams{
		ID:              controlRecordID,
		Notes:           notes,
		IsNotApplicable: status == db.AssessmentItemStatusNotApplicable,
	}); err != nil {
		return fmt.Errorf("update control record notes: %w", err)
	}
	return nil
}

func buildItemUpdateAuditPayload(input UpdateItemInput) []byte {
	return auditPayload(map[string]any{
		"status":         input.Status,
		"score":          input.Score,
		"priority":       input.Priority,
		"due_date":       input.DueDate.Format("2006-01-02"),
		"owner_user_id":  uuidString(input.OwnerUserID),
		"reviewer_id":    uuidString(input.ReviewerUserID),
		"blocked_reason": input.BlockedReason,
	})
}

func (s *AssessmentItemService) AddComment(ctx context.Context, actor db.User, itemID string, body string) error {
	id, err := uuidFromString(itemID)
	if err != nil {
		return err
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return fmt.Errorf("%w: comment body is required", ErrInvalidInput)
	}
	if err := errIfTooLong(body, maxCommentBytes, "comment"); err != nil {
		return err
	}

	_, err = withTx(ctx, s.pool, func(q *db.Queries) (struct{}, error) {
		detail, err := q.GetAssessmentItemDetail(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return struct{}{}, ErrNotFound
			}
			return struct{}{}, err
		}
		if !isCollaborator(actor, detail) {
			return struct{}{}, ErrForbidden
		}

		if _, err := q.CreateComment(ctx, db.CreateCommentParams{
			ControlRecordID: detail.ControlRecordID,
			UserID:          actor.ID,
			Body:            body,
		}); err != nil {
			return struct{}{}, fmt.Errorf("create comment: %w", err)
		}

		if err := q.CreateAuditLog(ctx, db.CreateAuditLogParams{
			EntityType:  "assessment_item",
			EntityID:    id,
			Action:      "comment_added",
			ActorUserID: actor.ID,
			PayloadJson: auditPayload(map[string]any{"body": body}),
		}); err != nil {
			return struct{}{}, fmt.Errorf("create comment audit: %w", err)
		}

		return struct{}{}, nil
	})
	return err
}

func (s *AssessmentItemService) AddEvidenceLink(ctx context.Context, actor db.User, itemID, label, url string) error {
	id, err := uuidFromString(itemID)
	if err != nil {
		return err
	}

	label = strings.TrimSpace(label)
	url = strings.TrimSpace(url)
	if label == "" || url == "" {
		return fmt.Errorf("%w: evidence label and url are required", ErrInvalidInput)
	}
	if err := errIfTooLong(label, maxEvidenceLabelBytes, "evidence label"); err != nil {
		return err
	}
	url, err = ValidateEvidenceURL(url)
	if err != nil {
		return err
	}

	_, err = withTx(ctx, s.pool, func(q *db.Queries) (struct{}, error) {
		detail, err := q.GetAssessmentItemDetail(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return struct{}{}, ErrNotFound
			}
			return struct{}{}, err
		}
		if !isCollaborator(actor, detail) {
			return struct{}{}, ErrForbidden
		}

		if _, err := q.CreateEvidenceLink(ctx, db.CreateEvidenceLinkParams{
			ControlRecordID: detail.ControlRecordID,
			Label:           label,
			Url:             url,
			CreatedBy:       actor.ID,
		}); err != nil {
			return struct{}{}, fmt.Errorf("create evidence link: %w", err)
		}

		if err := q.CreateAuditLog(ctx, db.CreateAuditLogParams{
			EntityType:  "assessment_item",
			EntityID:    id,
			Action:      "evidence_added",
			ActorUserID: actor.ID,
			PayloadJson: auditPayload(map[string]any{"label": label, "url": url}),
		}); err != nil {
			return struct{}{}, fmt.Errorf("create evidence audit: %w", err)
		}

		return struct{}{}, nil
	})
	return err
}
