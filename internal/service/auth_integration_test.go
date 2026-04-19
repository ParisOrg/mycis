package service

import (
	"testing"

	"mycis/internal/db"
)

func TestAuthServiceUpdateUserIgnoresWhitespaceOnlyPassword(t *testing.T) {
	h := newIntegrationHarness(t)

	user := h.createUser(t, "Original User", "original@example.com", db.UserRoleEditor)

	updated, err := h.services.Auth.UpdateUser(h.ctx, UpdateUserInput{
		ID:       user.ID.String(),
		Name:     "Updated User",
		Role:     db.UserRoleAssessmentManager,
		Password: "   ",
	})
	if err != nil {
		t.Fatal(err)
	}

	if updated.Name != "Updated User" {
		t.Fatalf("unexpected updated name: got %q want %q", updated.Name, "Updated User")
	}
	if updated.Role != db.UserRoleAssessmentManager {
		t.Fatalf("unexpected updated role: got %q want %q", updated.Role, db.UserRoleAssessmentManager)
	}
	if updated.MustChangePassword {
		t.Fatal("did not expect whitespace-only password to trigger a reset")
	}

	reloaded, err := h.services.Auth.GetUserByID(h.ctx, user.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.MustChangePassword {
		t.Fatal("did not expect persisted user to require a password change")
	}

	authenticated, err := h.services.Auth.Authenticate(h.ctx, user.Email, "password-12345")
	if err != nil {
		t.Fatal(err)
	}
	if authenticated.ID != user.ID {
		t.Fatalf("unexpected authenticated user: got %s want %s", authenticated.ID, user.ID)
	}
}
