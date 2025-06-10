package auth

import (
	"testing"
)

func TestNewRBAC(t *testing.T) {
	adminUsers := []string{"admin1", "admin2"}
	rbac := NewRBAC(adminUsers)

	if rbac == nil {
		t.Fatal("NewRBAC returned nil")
	}

	// Check that admin users are set correctly
	for _, admin := range adminUsers {
		if role := rbac.GetUserRole(admin); role != RoleAdmin {
			t.Errorf("Expected admin user %s to have role %s, got %s", admin, RoleAdmin, role)
		}
	}
}

func TestSetUserRole(t *testing.T) {
	rbac := NewRBAC([]string{})
	userID := "user123"

	rbac.SetUserRole(userID, RoleApprover)

	if role := rbac.GetUserRole(userID); role != RoleApprover {
		t.Errorf("Expected role %s, got %s", RoleApprover, role)
	}
}

func TestGetUserRole(t *testing.T) {
	rbac := NewRBAC([]string{"admin1"})

	tests := []struct {
		userID       string
		expectedRole Role
	}{
		{"admin1", RoleAdmin},
		{"unknown", RoleRequester}, // Default role
	}

	for _, test := range tests {
		role := rbac.GetUserRole(test.userID)
		if role != test.expectedRole {
			t.Errorf("For user %s, expected role %s, got %s", test.userID, test.expectedRole, role)
		}
	}
}

func TestUserHasPermission(t *testing.T) {
	rbac := NewRBAC([]string{"admin1"})
	rbac.SetUserRole("approver1", RoleApprover)
	rbac.SetUserRole("requester1", RoleRequester)

	tests := []struct {
		userID     string
		permission Permission
		expected   bool
	}{
		{"admin1", PermissionManageClusters, true},
		{"admin1", PermissionApproveRequests, true},
		{"admin1", PermissionCreateRequests, true},
		{"approver1", PermissionApproveRequests, true},
		{"approver1", PermissionManageClusters, false},
		{"requester1", PermissionCreateRequests, true},
		{"requester1", PermissionApproveRequests, false},
		{"requester1", PermissionManageClusters, false},
		{"unknown", PermissionCreateRequests, true}, // Default role
		{"unknown", PermissionApproveRequests, false},
	}

	for _, test := range tests {
		hasPermission := rbac.UserHasPermission(test.userID, test.permission)
		if hasPermission != test.expected {
			t.Errorf("User %s permission %s: expected %v, got %v", test.userID, test.permission, test.expected, hasPermission)
		}
	}
}

func TestIsAdmin(t *testing.T) {
	rbac := NewRBAC([]string{"admin1"})
	rbac.SetUserRole("approver1", RoleApprover)

	tests := []struct {
		userID   string
		expected bool
	}{
		{"admin1", true},
		{"approver1", false},
		{"unknown", false},
	}

	for _, test := range tests {
		isAdmin := rbac.IsAdmin(test.userID)
		if isAdmin != test.expected {
			t.Errorf("User %s IsAdmin: expected %v, got %v", test.userID, test.expected, isAdmin)
		}
	}
}

func TestValidatePermission(t *testing.T) {
	rbac := NewRBAC([]string{"admin1"})
	rbac.SetUserRole("requester1", RoleRequester)

	// Should not return error for valid permission
	err := rbac.ValidatePermission("admin1", PermissionManageClusters)
	if err != nil {
		t.Errorf("Expected no error for admin with manage clusters permission, got: %v", err)
	}

	// Should return error for invalid permission
	err = rbac.ValidatePermission("requester1", PermissionManageClusters)
	if err == nil {
		t.Error("Expected error for requester with manage clusters permission, got nil")
	}
}

func TestRolePermissions(t *testing.T) {
	// Test that role permissions are defined correctly
	adminPerms := rolePermissions[RoleAdmin]
	if len(adminPerms) == 0 {
		t.Error("Admin role should have permissions")
	}

	approverPerms := rolePermissions[RoleApprover]
	if len(approverPerms) == 0 {
		t.Error("Approver role should have permissions")
	}

	requesterPerms := rolePermissions[RoleRequester]
	if len(requesterPerms) == 0 {
		t.Error("Requester role should have permissions")
	}

	// Admin should have more permissions than approver
	if len(adminPerms) <= len(approverPerms) {
		t.Error("Admin should have more permissions than approver")
	}

	// Approver should have more permissions than requester
	if len(approverPerms) <= len(requesterPerms) {
		t.Error("Approver should have more permissions than requester")
	}
}
