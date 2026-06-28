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
)

var allPerms = []string{
	PermSignalsRead, PermSourcesRead, PermSourcesWrite, PermSubscriptionsRead,
	PermSubscriptionsWrite, PermDeliveriesRead, PermDeliveriesRetry, PermJobsRead,
	PermJobsManage, PermAnalyticsRead, PermUsersManage, PermTeamsManage, PermSettingsManage,
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

// Permissions returns the sorted permission list granted to a role.
func Permissions(role string) []string {
	ps := append([]string{}, rolePerms[role]...)
	sort.Strings(ps)
	return ps
}

// Can reports whether a role grants a permission.
func Can(role, perm string) bool {
	for _, p := range rolePerms[role] {
		if p == perm {
			return true
		}
	}
	return false
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
