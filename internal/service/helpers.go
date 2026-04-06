package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"mycis/internal/db"
)

func withTx[T any](ctx context.Context, pool *pgxpool.Pool, fn func(*db.Queries) (T, error)) (T, error) {
	var zero T

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return zero, fmt.Errorf("begin tx: %w", err)
	}

	queries := db.New(tx)
	result, err := fn(queries)
	if err != nil {
		_ = tx.Rollback(ctx)
		return zero, err
	}

	if err := tx.Commit(ctx); err != nil {
		return zero, fmt.Errorf("commit tx: %w", err)
	}

	return result, nil
}

func pgUUIDFromPtr(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}

func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func ptrUUIDFromPG(id pgtype.UUID) *uuid.UUID {
	if !id.Valid {
		return nil
	}
	value := uuid.UUID(id.Bytes)
	return &value
}

func pgTimestamp(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *value, Valid: true}
}

func auditPayload(payload map[string]any) []byte {
	data, err := json.Marshal(payload)
	if err != nil {
		return []byte(`{"error":"marshal audit payload"}`)
	}
	return data
}

func isCollaborator(user db.User, detail db.GetAssessmentItemDetailRow) bool {
	if user.IsAdmin {
		return true
	}

	if owner := ptrUUIDFromPG(detail.OwnerUserID); owner != nil && *owner == user.ID {
		return true
	}
	if reviewer := ptrUUIDFromPG(detail.ReviewerUserID); reviewer != nil && *reviewer == user.ID {
		return true
	}
	return false
}
