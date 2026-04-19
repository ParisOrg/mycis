package db

import "testing"

func TestParseUserRole(t *testing.T) {
	t.Parallel()

	role, err := ParseUserRole(" assessment_manager ")
	if err != nil {
		t.Fatal(err)
	}
	if role != UserRoleAssessmentManager {
		t.Fatalf("unexpected role: %s", role)
	}
}

func TestUserPermissions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                  string
		user                  User
		wantManageUsers       bool
		wantManageAssessments bool
		wantEditAssignedItems bool
	}{
		{
			name:                  "admin",
			user:                  User{Role: UserRoleAdmin},
			wantManageUsers:       true,
			wantManageAssessments: true,
			wantEditAssignedItems: true,
		},
		{
			name:                  "assessment manager",
			user:                  User{Role: UserRoleAssessmentManager},
			wantManageAssessments: true,
			wantEditAssignedItems: true,
		},
		{
			name:                  "editor",
			user:                  User{Role: UserRoleEditor},
			wantEditAssignedItems: true,
		},
		{
			name: "viewer",
			user: User{Role: UserRoleViewer},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.user.CanManageUsers(); got != tt.wantManageUsers {
				t.Fatalf("CanManageUsers() = %t, want %t", got, tt.wantManageUsers)
			}
			if got := tt.user.CanManageAssessments(); got != tt.wantManageAssessments {
				t.Fatalf("CanManageAssessments() = %t, want %t", got, tt.wantManageAssessments)
			}
			if got := tt.user.CanEditAssignedItems(); got != tt.wantEditAssignedItems {
				t.Fatalf("CanEditAssignedItems() = %t, want %t", got, tt.wantEditAssignedItems)
			}
		})
	}
}
