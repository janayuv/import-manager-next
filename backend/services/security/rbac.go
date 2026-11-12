package security

import "strings"

// Role represents a canonical ERP RBAC role.
type Role string

const (
	RoleAdministrator Role = "administrator"
	RoleManager       Role = "manager"
	RoleOperator      Role = "operator"
	RoleViewer        Role = "viewer"
)

var AllRoles = []Role{
	RoleAdministrator,
	RoleManager,
	RoleOperator,
	RoleViewer,
}

// Permission constants matching the canonical 25 Import Manager permissions.
const (
	PermDataRead               = "data.read"
	PermDataWrite              = "data.write"
	PermDataDelete             = "data.delete"
	PermBackupCreate           = "backup.create"
	PermBackupRestore          = "backup.restore"
	PermBackupSchedule         = "backup.schedule"
	PermBackupSettings         = "backup.settings"
	PermAuditView              = "audit.view"
	PermRecycleBinView         = "recycle_bin.view"
	PermRecycleBinRestore      = "recycle_bin.restore"
	PermRecycleBinPurge        = "recycle_bin.purge"
	PermAdminActivityLog       = "admin.activity_log"
	PermAdminUserActivity      = "admin.user_activity"
	PermAdminSystemHealth      = "admin.system_health"
	PermAdminSystemTools       = "admin.system_tools"
	PermAutomationView         = "automation.view"
	PermAutomationMutate       = "automation.mutate"
	PermAutomationOpsCenter    = "automation.ops_center"
	PermAutomationSystemAgent  = "automation.system_agent"
	PermRoleRead               = "role.read"
	PermRoleWrite              = "role.write"
	PermRoleBootstrap          = "role.bootstrap"
	PermSecurityChangePassword = "security.change_password"
	PermSecuritySessionRead    = "security.session_read"
	PermSecurityLockoutManage  = "security.lockout_manage"
)

var AllPermissions = []string{
	PermDataRead,
	PermDataWrite,
	PermDataDelete,
	PermBackupCreate,
	PermBackupRestore,
	PermBackupSchedule,
	PermBackupSettings,
	PermAuditView,
	PermRecycleBinView,
	PermRecycleBinRestore,
	PermRecycleBinPurge,
	PermAdminActivityLog,
	PermAdminUserActivity,
	PermAdminSystemHealth,
	PermAdminSystemTools,
	PermAutomationView,
	PermAutomationMutate,
	PermAutomationOpsCenter,
	PermAutomationSystemAgent,
	PermRoleRead,
	PermRoleWrite,
	PermRoleBootstrap,
	PermSecurityChangePassword,
	PermSecuritySessionRead,
	PermSecurityLockoutManage,
}

var managerPermissions = map[string]bool{
	PermDataRead:              true,
	PermDataWrite:             true,
	PermDataDelete:            true,
	PermBackupCreate:          true,
	PermBackupRestore:         true,
	PermBackupSchedule:        true,
	PermBackupSettings:        true,
	PermAuditView:             true,
	PermRecycleBinView:        true,
	PermRecycleBinRestore:     true,
	PermAutomationView:        true,
	PermAutomationMutate:      true,
	PermAutomationOpsCenter:   true,
	PermAutomationSystemAgent: true,
	PermSecuritySessionRead:   true,
}

var operatorPermissions = map[string]bool{
	PermDataRead:       true,
	PermDataWrite:      true,
	PermRecycleBinView: true,
}

var viewerPermissions = map[string]bool{
	PermDataRead:       true,
	PermAuditView:      true,
	PermRecycleBinView: true,
	PermAutomationView: true,
}

// ParseRole normalizes canonical and legacy role aliases.
func ParseRole(raw string) (Role, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "administrator", "admin":
		return RoleAdministrator, true
	case "manager", "db_manager", "dbmanager":
		return RoleManager, true
	case "operator", "user":
		return RoleOperator, true
	case "viewer", "readonly", "read_only":
		return RoleViewer, true
	default:
		return "", false
	}
}

// NormalizePermission resolves canonical and legacy permission aliases.
func NormalizePermission(raw string) (string, bool) {
	p := strings.ToLower(strings.TrimSpace(raw))
	switch p {
	case "data.read", "data.browse":
		return PermDataRead, true
	case "data.write", "data.edit":
		return PermDataWrite, true
	case "backup.settings", "settings.manage":
		return PermBackupSettings, true
	case "role.read", "user.manage":
		return PermRoleRead, true
	}
	for _, perm := range AllPermissions {
		if p == perm {
			return perm, true
		}
	}
	return "", false
}

// RoleHasPermission checks the canonical 4x25 RBAC matrix.
func RoleHasPermission(role Role, perm string) bool {
	canonPerm, ok := NormalizePermission(perm)
	if !ok {
		return false
	}
	switch role {
	case RoleAdministrator:
		return true
	case RoleManager:
		return managerPermissions[canonPerm]
	case RoleOperator:
		return operatorPermissions[canonPerm]
	case RoleViewer:
		return viewerPermissions[canonPerm]
	default:
		return false
	}
}

// PermissionsForRole returns an ordered list of allowed permissions for a role.
func PermissionsForRole(role Role) []string {
	var allowed []string
	for _, p := range AllPermissions {
		if RoleHasPermission(role, p) {
			allowed = append(allowed, p)
		}
	}
	return allowed
}

// RoleMatrixRow describes a role and its granted permissions for UI rendering.
type RoleMatrixRow struct {
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
}

// GetRoleMatrix returns the complete 4-role x 25-permission matrix.
func GetRoleMatrix() []RoleMatrixRow {
	rows := make([]RoleMatrixRow, 0, len(AllRoles))
	for _, r := range AllRoles {
		rows = append(rows, RoleMatrixRow{
			Role:        string(r),
			Permissions: PermissionsForRole(r),
		})
	}
	return rows
}
