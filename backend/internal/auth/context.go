package auth

import (
	"context"
	"errors"
)

// Identity is the authenticated principal stored in the request context.
type Identity struct {
	UserID string
	Email  string
	Role   string
	Token  string // the session token used (for logout)
	// AccountID is nil for platform-staff (operator console) users and set to the
	// tenant id for account-scoped users.
	AccountID *string
}

type ctxKey struct{}

// ErrUnauthenticated is returned when no valid identity is present.
var ErrUnauthenticated = errors.New("unauthenticated")

// ErrForbidden is returned when the identity lacks a required permission.
var ErrForbidden = errors.New("forbidden")

// WithIdentity returns a context carrying the identity.
func WithIdentity(ctx context.Context, id *Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// IdentityFrom returns the identity in the context, or nil.
func IdentityFrom(ctx context.Context) *Identity {
	id, _ := ctx.Value(ctxKey{}).(*Identity)
	return id
}

// Require returns the identity or ErrUnauthenticated.
func Require(ctx context.Context) (*Identity, error) {
	id := IdentityFrom(ctx)
	if id == nil {
		return nil, ErrUnauthenticated
	}
	return id, nil
}

// RequirePermission ensures the context identity holds perm. Tenant (account-
// scoped) identities are limited to the tenant capability set, so an account
// user with an ADMIN role still cannot reach operator-only resolvers.
func RequirePermission(ctx context.Context, perm string) error {
	id, err := Require(ctx)
	if err != nil {
		return err
	}
	if !CanScoped(id.Role, id.AccountID != nil, perm) {
		return ErrForbidden
	}
	return nil
}

// IsTenant reports whether the context identity is account-scoped (a customer)
// rather than platform staff.
func IsTenant(ctx context.Context) bool {
	id := IdentityFrom(ctx)
	return id != nil && id.AccountID != nil
}
