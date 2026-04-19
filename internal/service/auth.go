package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mycis/internal/auth"
	"mycis/internal/db"
)

type AuthService struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func (s *AuthService) Authenticate(ctx context.Context, email, password string) (db.User, error) {
	emailNorm, err := NormalizeEmailForAuth(email)
	if err != nil {
		return db.User{}, err
	}

	user, err := s.queries.GetUserByEmail(ctx, emailNorm)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.User{}, ErrUnauthorized
		}
		return db.User{}, fmt.Errorf("get user by email: %w", err)
	}

	match, err := auth.ComparePassword(user.PasswordHash, password)
	if err != nil {
		return db.User{}, fmt.Errorf("compare password: %w", err)
	}
	if !match {
		return db.User{}, ErrUnauthorized
	}

	return user, nil
}

func (s *AuthService) GetUserByID(ctx context.Context, id string) (db.User, error) {
	userID, err := uuidFromString(id)
	if err != nil {
		return db.User{}, err
	}

	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.User{}, ErrNotFound
		}
		return db.User{}, fmt.Errorf("get user: %w", err)
	}

	return user, nil
}

func (s *AuthService) ListUsers(ctx context.Context) ([]db.User, error) {
	return s.queries.ListUsers(ctx)
}

func (s *AuthService) CreateUser(ctx context.Context, name, email string, role db.UserRole) (db.User, string, error) {
	password, err := auth.GeneratePassword(20)
	if err != nil {
		return db.User{}, "", fmt.Errorf("generate password: %w", err)
	}

	user, err := s.CreateUserWithPassword(ctx, name, email, password, role, true)
	if err != nil {
		return db.User{}, "", err
	}

	return user, password, nil
}

func (s *AuthService) CreateUserWithPassword(ctx context.Context, name, email, password string, role db.UserRole, mustChange bool) (db.User, error) {
	name = strings.TrimSpace(name)

	emailNorm, err := ValidateEmailForStorage(email)
	if err != nil {
		return db.User{}, err
	}

	if name == "" {
		return db.User{}, fmt.Errorf("%w: name, email, and password are required", ErrInvalidInput)
	}

	if err := errIfTooLong(name, maxUserNameBytes, "name"); err != nil {
		return db.User{}, err
	}
	if !role.Valid() {
		return db.User{}, fmt.Errorf("%w: valid role is required", ErrInvalidInput)
	}
	password, err = normalizePassword(password)
	if err != nil {
		return db.User{}, err
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return db.User{}, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.queries.CreateUser(ctx, db.CreateUserParams{
		Name:               name,
		Lower:              emailNorm,
		PasswordHash:       hash,
		Role:               role,
		MustChangePassword: mustChange,
	})
	if err != nil {
		return db.User{}, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

func (s *AuthService) ChangePassword(ctx context.Context, userID string, password string) error {
	id, err := uuidFromString(userID)
	if err != nil {
		return err
	}
	password, err = normalizePassword(password)
	if err != nil {
		return err
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := s.queries.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{
		ID:           id,
		PasswordHash: hash,
	}); err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	return nil
}

type UpdateUserInput struct {
	ID       string
	Name     string
	Role     db.UserRole
	Password string
}

func (s *AuthService) UpdateUser(ctx context.Context, input UpdateUserInput) (db.User, error) {
	id, err := uuidFromString(input.ID)
	if err != nil {
		return db.User{}, err
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return db.User{}, fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	if err := errIfTooLong(name, maxUserNameBytes, "name"); err != nil {
		return db.User{}, err
	}
	if !input.Role.Valid() {
		return db.User{}, fmt.Errorf("%w: valid role is required", ErrInvalidInput)
	}

	password := strings.TrimSpace(input.Password)
	if password == "" {
		user, err := s.queries.UpdateUser(ctx, db.UpdateUserParams{
			ID:   id,
			Name: name,
			Role: input.Role,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return db.User{}, ErrNotFound
			}
			return db.User{}, fmt.Errorf("update user: %w", err)
		}
		return user, nil
	}

	password, err = normalizePassword(password)
	if err != nil {
		return db.User{}, err
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return db.User{}, fmt.Errorf("hash password: %w", err)
	}

	if s.pool == nil {
		return db.User{}, fmt.Errorf("update user: auth service transaction pool is not configured")
	}

	user, err := withTx(ctx, s.pool, func(q *db.Queries) (db.User, error) {
		user, err := q.UpdateUser(ctx, db.UpdateUserParams{
			ID:   id,
			Name: name,
			Role: input.Role,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return db.User{}, ErrNotFound
			}
			return db.User{}, fmt.Errorf("update user: %w", err)
		}
		if err := q.UpdateUserPasswordReset(ctx, db.UpdateUserPasswordResetParams{
			ID:           id,
			PasswordHash: hash,
		}); err != nil {
			return db.User{}, fmt.Errorf("update user password: %w", err)
		}
		user.MustChangePassword = true
		return user, nil
	})
	if err != nil {
		return db.User{}, err
	}

	return user, nil
}
