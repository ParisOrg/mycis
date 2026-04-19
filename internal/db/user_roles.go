package db

import (
	"fmt"
	"strings"
)

func AllUserRoles() []UserRole {
	return []UserRole{
		UserRoleAdmin,
		UserRoleAssessmentManager,
		UserRoleEditor,
		UserRoleViewer,
	}
}

func ParseUserRole(raw string) (UserRole, error) {
	role := UserRole(strings.TrimSpace(strings.ToLower(raw)))
	if !role.Valid() {
		return "", fmt.Errorf("invalid role %q", raw)
	}
	return role, nil
}

func (r UserRole) Valid() bool {
	switch r {
	case UserRoleAdmin, UserRoleAssessmentManager, UserRoleEditor, UserRoleViewer:
		return true
	default:
		return false
	}
}

func (r UserRole) Label() string {
	switch r {
	case UserRoleAdmin:
		return "Admin"
	case UserRoleAssessmentManager:
		return "Assessment manager"
	case UserRoleEditor:
		return "Editor"
	case UserRoleViewer:
		return "Viewer"
	default:
		return string(r)
	}
}

func (u User) HasRole(roles ...UserRole) bool {
	for _, role := range roles {
		if u.Role == role {
			return true
		}
	}
	return false
}

func (u User) CanManageUsers() bool {
	return u.HasRole(UserRoleAdmin)
}

func (u User) CanManageAssessments() bool {
	return u.HasRole(UserRoleAdmin, UserRoleAssessmentManager)
}

func (u User) CanEditAssignedItems() bool {
	return u.HasRole(UserRoleAdmin, UserRoleAssessmentManager, UserRoleEditor)
}

func (u User) CanBeAssignedItems() bool {
	return u.CanEditAssignedItems()
}
