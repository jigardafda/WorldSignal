// Package auth provides password hashing, session-token generation, and the
// role → permission matrix used for authorization.
package auth

import (
	"crypto/rand"
	"encoding/base64"
	"sort"

	"golang.org/x/crypto/bcrypt"
)

// Roles.
const (
	RoleAdmin  = "ADMIN"
	RoleEditor = "EDITOR"
	RoleViewer = "VIEWER"
)

// ValidRole reports whether r is a known role.
func ValidRole(r string) bool {
	return r == RoleAdmin || r == RoleEditor || r == RoleViewer
}

// Permissions (resource:action). The frontend uses these to gate UI.
const (
	PermSignalsRead        = "signals:read"
	PermSourcesRead        = "sources:read"
	PermSourcesWrite       = "sources:write"
	PermSubscriptionsRead  = "subscriptions:read"
	PermSubscriptionsWrite = "subscriptions:write"
	PermDeliveriesRead     = "deliveries:read"
	PermDeliveriesRetry    = "deliveries:retry"
	PermJobsRead           = "jobs:read"
	PermJobsManage         = "jobs:manage"
	PermAnalyticsRead      = "analytics:read"
	PermUsersManage        = "users:manage"
	PermTeamsManage        = "teams:manage"
	PermSettingsManage     = "settings:manage"
	PermAccountsManage     = "accounts:manage"
)

var allPerms = []string{
	PermSignalsRead, PermSourcesRead, PermSourcesWrite, PermSubscriptionsRead,
	PermSubscriptionsWrite, PermDeliveriesRead, PermDeliveriesRetry, PermJobsRead,
	PermJobsManage, PermAnalyticsRead, PermUsersManage, PermTeamsManage, PermSettingsManage,
	PermAccountsManage,
}

var readPerms = []string{
	PermSignalsRead, PermSourcesRead, PermSubscriptionsRead, PermDeliveriesRead,
	PermJobsRead, PermAnalyticsRead,
}

var editorPerms = append(append([]string{}, readPerms...),
	PermSourcesWrite, PermSubscriptionsWrite, PermDeliveriesRetry, PermJobsManage)

var rolePerms = map[string][]string{
	RoleAdmin:  allPerms,
	RoleEditor: editorPerms,
	RoleViewer: readPerms,
}

// tenantPerms is the capability set a tenant (account-scoped) user gets,
// regardless of their stored role. Tenants live in the customer console: they
// read the shared signal corpus and analytics, and manage their own account +
// API keys (the latter enforced by account ownership, not a role permission).
// They can never reach operator-only surfaces (sources, users, jobs, settings,
// accounts, other tenants' data).
var tenantPerms = []string{
	PermSignalsRead, PermAnalyticsRead,
}

// Permissions returns the sorted permission list granted to a role (platform
// staff). For tenant users, use TenantPermissions instead.
func Permissions(role string) []string {
	ps := append([]string{}, rolePerms[role]...)
	sort.Strings(ps)
	return ps
}

// TenantPermissions returns the sorted capability set for an account-scoped user.
func TenantPermissions() []string {
	ps := append([]string{}, tenantPerms...)
	sort.Strings(ps)
	return ps
}

// EffectivePermissions returns the permissions in force for a principal: the
// tenant set when account-scoped, else the full role matrix.
func EffectivePermissions(role string, tenant bool) []string {
	if tenant {
		return TenantPermissions()
	}
	return Permissions(role)
}

// Can reports whether a role (platform staff) grants a permission.
func Can(role, perm string) bool {
	for _, p := range rolePerms[role] {
		if p == perm {
			return true
		}
	}
	return false
}

// CanScoped reports whether a principal may exercise perm, honouring tenant
// scoping: tenant users are limited to the tenant capability set.
func CanScoped(role string, tenant bool, perm string) bool {
	if tenant {
		for _, p := range tenantPerms {
			if p == perm {
				return true
			}
		}
		return false
	}
	return Can(role, perm)
}

// HashPassword returns a bcrypt hash of the password.
func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}

// CheckPassword reports whether pw matches the bcrypt hash.
func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

// GenerateToken returns a 256-bit URL-safe random token.
func GenerateToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}
