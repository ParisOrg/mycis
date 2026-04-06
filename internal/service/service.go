package service

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"mycis/internal/db"
)

type Services struct {
	Auth        *AuthService
	Frameworks  *FrameworkService
	Assessments *AssessmentService
	Items       *AssessmentItemService
	Dashboard   *DashboardService
}

func New(pool *pgxpool.Pool) *Services {
	queries := db.New(pool)
	authSvc := &AuthService{queries: queries}
	return &Services{
		Auth:        authSvc,
		Frameworks:  &FrameworkService{pool: pool, queries: queries},
		Assessments: &AssessmentService{pool: pool, queries: queries},
		Items:       &AssessmentItemService{pool: pool, queries: queries},
		Dashboard:   &DashboardService{queries: queries},
	}
}
