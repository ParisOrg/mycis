package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"mycis/internal/db"
)

type DashboardData struct {
	Overview    db.GetDashboardOverviewRow
	ByGroup     []db.ListDashboardGroupCompletionRow
	ByOwner     []db.ListDashboardOwnerWorkloadRow
	Overdue     []db.ListDashboardOverdueItemsRow
	ReviewQueue []db.ListDashboardReviewQueueRow
	LowScore    []db.ListDashboardLowScoreItemsRow
}

type DashboardService struct {
	queries *db.Queries
}

func (s *DashboardService) Get(ctx context.Context, assessmentID uuid.UUID) (DashboardData, error) {
	overview, err := s.queries.GetDashboardOverview(ctx, assessmentID)
	if err != nil {
		return DashboardData{}, fmt.Errorf("dashboard overview: %w", err)
	}
	byGroup, err := s.queries.ListDashboardGroupCompletion(ctx, assessmentID)
	if err != nil {
		return DashboardData{}, fmt.Errorf("dashboard groups: %w", err)
	}
	byOwner, err := s.queries.ListDashboardOwnerWorkload(ctx, assessmentID)
	if err != nil {
		return DashboardData{}, fmt.Errorf("dashboard owners: %w", err)
	}
	overdue, err := s.queries.ListDashboardOverdueItems(ctx, assessmentID)
	if err != nil {
		return DashboardData{}, fmt.Errorf("dashboard overdue: %w", err)
	}
	reviewQueue, err := s.queries.ListDashboardReviewQueue(ctx, assessmentID)
	if err != nil {
		return DashboardData{}, fmt.Errorf("dashboard review queue: %w", err)
	}
	lowScore, err := s.queries.ListDashboardLowScoreItems(ctx, assessmentID)
	if err != nil {
		return DashboardData{}, fmt.Errorf("dashboard low score: %w", err)
	}

	return DashboardData{
		Overview:    overview,
		ByGroup:     byGroup,
		ByOwner:     byOwner,
		Overdue:     overdue,
		ReviewQueue: reviewQueue,
		LowScore:    lowScore,
	}, nil
}
