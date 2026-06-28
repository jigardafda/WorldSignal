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

// RequirePermission ensures the context identity holds perm.
func RequirePermission(ctx context.Context, perm string) error {
	id, err := Require(ctx)
	if err != nil {
		return err
	}
	if !Can(id.Role, perm) {
		return ErrForbidden
	}
	return nil
}
