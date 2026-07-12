package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/worldsignal/backend/internal/auth"
	"github.com/worldsignal/backend/internal/cuid"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/gql"
)

// ---- tenant context (public API) ----

type tenantCtxKey struct{}

// withTenant tags a request context with the account an API key belongs to.
func withTenant(ctx context.Context, accountID string) context.Context {
	return context.WithValue(ctx, tenantCtxKey{}, accountID)
}

// tenantAccountID returns the account id an API-key request is scoped to, or "".
func tenantAccountID(ctx context.Context) string {
	id, _ := ctx.Value(tenantCtxKey{}).(string)
	return id
}

// ---- account admin (GraphQL, accounts:manage) ----

func (s *Server) registerAccountResolvers(q, m map[string]gql.FieldResolver) {
	q["accounts"] = s.resolveAccounts
	q["account"] = s.resolveAccount
	m["createAccount"] = s.mutCreateAccount
	m["updateAccount"] = s.mutUpdateAccount
}

func accountToMap(a *db.Account) map[string]any {
	return map[string]any{
		"id": a.ID, "name": a.Name, "slug": a.Slug, "status": a.Status,
		"plan": a.Plan, "createdAt": a.CreatedAt, "updatedAt": a.UpdatedAt,
	}
}

// slugify renders a URL/subdomain-safe slug: lowercase alphanumerics with single
// dashes between words and no leading/trailing dash.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '-' || r == '_':
			if b.Len() > 0 && !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.TrimRight(b.String(), "-")
}

func (s *Server) resolveAccounts(ctx context.Context, _ map[string]any) (any, error) {
	if err := authz(ctx, auth.PermAccountsManage); err != nil {
		return nil, err
	}
	rows, err := s.DB.ListAccounts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]any, len(rows))
	for i, a := range rows {
		out[i] = accountToMap(a)
	}
	return out, nil
}

func (s *Server) resolveAccount(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermAccountsManage); err != nil {
		return nil, err
	}
	a, err := s.DB.GetAccount(ctx, strVal(args["id"]))
	if err != nil || a == nil {
		return nil, err
	}
	return accountToMap(a), nil
}

func (s *Server) mutCreateAccount(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermAccountsManage); err != nil {
		return nil, err
	}
	in, _ := args["input"].(map[string]any)
	name := strings.TrimSpace(strVal(in["name"]))
	if name == "" {
		return nil, fmt.Errorf("%w: name is required", errValidation)
	}
	slug := slugify(strVal(in["slug"]))
	if slug == "" {
		slug = slugify(name)
	}
	if slug == "" {
		return nil, fmt.Errorf("%w: a slug (letters/numbers) is required", errValidation)
	}
	plan := strings.TrimSpace(strVal(in["plan"]))
	if plan != "" && !db.ValidAccountPlan(plan) {
		return nil, fmt.Errorf("%w: unknown plan %q", errValidation, plan)
	}
	a, err := s.DB.CreateAccount(ctx, cuid.New(), name, slug, plan)
	if err != nil {
		if err == db.ErrDuplicateSlug {
			return nil, fmt.Errorf("%w: slug %q is already taken", errValidation, slug)
		}
		return nil, err
	}
	s.audit(ctx, "ACCOUNT_CREATED", "account", a.ID, map[string]any{"name": name, "slug": slug, "plan": a.Plan})
	return accountToMap(a), nil
}

func (s *Server) mutUpdateAccount(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermAccountsManage); err != nil {
		return nil, err
	}
	id := strVal(args["id"])
	in, _ := args["input"].(map[string]any)
	var patch db.AccountPatch
	if v, ok := in["name"].(string); ok {
		name := strings.TrimSpace(v)
		if name == "" {
			return nil, fmt.Errorf("%w: name cannot be empty", errValidation)
		}
		patch.Name = &name
	}
	if v, ok := in["status"].(string); ok {
		if !db.ValidAccountStatus(v) {
			return nil, fmt.Errorf("%w: unknown status %q", errValidation, v)
		}
		patch.Status = &v
	}
	if v, ok := in["plan"].(string); ok {
		if !db.ValidAccountPlan(v) {
			return nil, fmt.Errorf("%w: unknown plan %q", errValidation, v)
		}
		patch.Plan = &v
	}
	a, err := s.DB.UpdateAccount(ctx, id, patch)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, fmt.Errorf("%w: account not found", errValidation)
	}
	s.audit(ctx, "ACCOUNT_UPDATED", "account", a.ID, map[string]any{"status": a.Status, "plan": a.Plan})
	return accountToMap(a), nil
}

// ---- tenant self-service (public REST API) ----

// accountForKey returns the account the presented API key belongs to. It lets a
// tenant confirm which account its credential is scoped to.
func (s *Server) accountForKey(w http.ResponseWriter, r *http.Request) {
	aid := tenantAccountID(r.Context())
	if aid == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no tenant context"})
		return
	}
	a, err := s.DB.GetAccount(r.Context(), aid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if a == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "account not found"})
		return
	}
	writeJSON(w, http.StatusOK, struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Slug   string `json:"slug"`
		Status string `json:"status"`
		Plan   string `json:"plan"`
	}{a.ID, a.Name, a.Slug, a.Status, a.Plan})
}
