package httpapi

import (
	"context"
	"fmt"
	"strings"

	"github.com/worldsignal/backend/internal/auth"
	"github.com/worldsignal/backend/internal/cuid"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/gql"
)

// tenantAPIScopes is the capability set a tenant may grant its own API keys.
// It is a read-only subset of APIScopes — tenants cannot mint write credentials.
var tenantAPIScopes = []string{"signals:read", "stats:read", "subscriptions:read", "deliveries:read"}

func validTenantScope(s string) bool {
	for _, v := range tenantAPIScopes {
		if v == s {
			return true
		}
	}
	return false
}

// registerTenantResolvers wires the customer-console self-service surface: a
// tenant sees and manages only its own account + API keys.
func (s *Server) registerTenantResolvers(q, m map[string]gql.FieldResolver) {
	q["myAccount"] = s.resolveMyAccount
	q["myApiKeys"] = s.resolveMyApiKeys
	q["tenantApiScopes"] = s.resolveTenantApiScopes
	m["createMyApiKey"] = s.mutCreateMyApiKey
	m["revokeMyApiKey"] = s.mutRevokeMyApiKey
}

// requireTenant returns the calling identity iff it is account-scoped (a
// customer). Platform staff and unauthenticated callers are rejected.
func requireTenant(ctx context.Context) (*auth.Identity, error) {
	id, err := auth.Require(ctx)
	if err != nil {
		return nil, err
	}
	if id.AccountID == nil {
		return nil, auth.ErrForbidden
	}
	return id, nil
}

func (s *Server) resolveMyAccount(ctx context.Context, _ map[string]any) (any, error) {
	id, err := requireTenant(ctx)
	if err != nil {
		return nil, err
	}
	a, err := s.DB.GetAccount(ctx, *id.AccountID)
	if err != nil || a == nil {
		return nil, err
	}
	return accountToMap(a), nil
}

func (s *Server) resolveTenantApiScopes(ctx context.Context, _ map[string]any) (any, error) {
	if _, err := requireTenant(ctx); err != nil {
		return nil, err
	}
	out := make([]any, len(tenantAPIScopes))
	for i, sc := range tenantAPIScopes {
		out[i] = sc
	}
	return out, nil
}

func (s *Server) resolveMyApiKeys(ctx context.Context, _ map[string]any) (any, error) {
	id, err := requireTenant(ctx)
	if err != nil {
		return nil, err
	}
	keys, err := s.DB.ListAPIKeysByAccount(ctx, *id.AccountID)
	if err != nil {
		return nil, err
	}
	out := make([]any, len(keys))
	for i, k := range keys {
		out[i] = apiKeyToMap(k)
	}
	return out, nil
}

func (s *Server) mutCreateMyApiKey(ctx context.Context, args map[string]any) (any, error) {
	id, err := requireTenant(ctx)
	if err != nil {
		return nil, err
	}
	in, _ := args["input"].(map[string]any)
	name := strings.TrimSpace(strVal(in["name"]))
	if name == "" {
		return nil, fmt.Errorf("%w: name is required", errValidation)
	}
	var scopes []string
	if raw, ok := in["scopes"].([]any); ok {
		for _, sv := range raw {
			sc := strVal(sv)
			if !validTenantScope(sc) {
				return nil, fmt.Errorf("%w: scope %q is not available to tenants", errValidation, sc)
			}
			if !hasScope(scopes, sc) {
				scopes = append(scopes, sc)
			}
		}
	}
	if len(scopes) == 0 {
		return nil, fmt.Errorf("%w: at least one scope is required", errValidation)
	}
	rate := toInt(in["rateLimitPerMin"], 120)
	if rate < 1 {
		rate = 1
	}
	raw, hash, prefix, err := generateAPIKey()
	if err != nil {
		return nil, err
	}
	create := db.CreateAPIKeyInput{AccountID: *id.AccountID, Name: name, Hash: hash, Prefix: prefix, Scopes: scopes, RateLimitPerMin: rate, CreatedBy: &id.UserID}
	k, err := s.DB.CreateAPIKey(ctx, cuid.New(), create)
	if err != nil {
		return nil, err
	}
	s.audit(ctx, "TENANT_API_KEY_CREATED", "apiKey", k.ID, map[string]any{"name": name, "accountId": *id.AccountID})
	out := apiKeyToMap(k)
	out["key"] = raw // shown exactly once
	return out, nil
}

func (s *Server) mutRevokeMyApiKey(ctx context.Context, args map[string]any) (any, error) {
	id, err := requireTenant(ctx)
	if err != nil {
		return nil, err
	}
	keyID := strVal(args["id"])
	ok, err := s.DB.DeleteAPIKeyForAccount(ctx, keyID, *id.AccountID)
	if err != nil {
		return nil, err
	}
	if ok {
		s.audit(ctx, "TENANT_API_KEY_REVOKED", "apiKey", keyID, map[string]any{"accountId": *id.AccountID})
	}
	return ok, nil
}
