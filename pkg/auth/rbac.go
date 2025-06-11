package auth

import (
	"fmt"
	"sync"
)

type Role string

const (
	RoleAdmin     Role = "admin"
	RoleApprover  Role = "approver"
	RoleRequester Role = "requester"
)

type Permission string

const (
	PermissionManageClusters  Permission = "clusters:manage"
	PermissionManageUsers     Permission = "users:manage"
	PermissionApproveRequests Permission = "requests:approve"
	PermissionCreateRequests  Permission = "requests:create"
	PermissionViewRequests    Permission = "requests:view"
	PermissionRevokeAccess    Permission = "access:revoke"
	PermissionViewAuditLog    Permission = "audit:view"
)

var rolePermissions = map[Role][]Permission{
	RoleAdmin: {
		PermissionManageClusters,
		PermissionManageUsers,
		PermissionApproveRequests,
		PermissionCreateRequests,
		PermissionViewRequests,
		PermissionRevokeAccess,
		PermissionViewAuditLog,
	},
	RoleApprover: {
		PermissionApproveRequests,
		PermissionCreateRequests,
		PermissionViewRequests,
		PermissionRevokeAccess,
		PermissionViewAuditLog,
	},
	RoleRequester: {
		PermissionCreateRequests,
		PermissionViewRequests,
	},
}

type RBAC struct {
	mu     sync.RWMutex
	users  map[string]Role
	admins []string
}

func NewRBAC(adminUsers []string) *RBAC {
	rbac := &RBAC{
		users:  make(map[string]Role),
		admins: adminUsers,
	}

	for _, admin := range adminUsers {
		rbac.users[admin] = RoleAdmin
	}

	return rbac
}

func (r *RBAC) SetUserRole(userID string, role Role) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.users[userID] = role
}

func (r *RBAC) GetUserRole(userID string) Role {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if role, exists := r.users[userID]; exists {
		return role
	}
	return RoleRequester
}

func (r *RBAC) UserHasPermission(userID string, permission Permission) bool {
	role := r.GetUserRole(userID)
	permissions, exists := rolePermissions[role]
	if !exists {
		return false
	}

	for _, p := range permissions {
		if p == permission {
			return true
		}
	}
	return false
}

func (r *RBAC) IsAdmin(userID string) bool {
	return r.GetUserRole(userID) == RoleAdmin
}

func (r *RBAC) ValidatePermission(userID string, permission Permission) error {
	if !r.UserHasPermission(userID, permission) {
		return fmt.Errorf("user %s does not have permission %s", userID, permission)
	}
	return nil
}
