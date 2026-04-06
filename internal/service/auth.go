package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"mycis/internal/auth"
	"mycis/internal/db"
)

type AuthService struct {
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

func (s *AuthService) CreateUser(ctx context.Context, name, email string, isAdmin bool) (db.User, string, error) {
	password, err := auth.GeneratePassword(20)
	if err != nil {
		return db.User{}, "", fmt.Errorf("generate password: %w", err)
	}

	user, err := s.CreateUserWithPassword(ctx, name, email, password, isAdmin, true)
	if err != nil {
		return db.User{}, "", err
	}

	return user, password, nil
}

func (s *AuthService) CreateUserWithPassword(ctx context.Context, name, email, password string, isAdmin, mustChange bool) (db.User, error) {
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
		IsAdmin:            isAdmin,
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
