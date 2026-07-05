package httpapi

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/worldsignal/backend/internal/auth"
	"github.com/worldsignal/backend/internal/crypto"
	"github.com/worldsignal/backend/internal/cuid"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/gql"
)

// apiKeyPrefix identifies a WorldSignal API key ("wsk_" + random). It is stored
// alongside the hash for display and helps consumers recognize the credential.
const apiKeyPrefix = "wsk_"

// APIScopes is the closed set of scopes an API key can be granted. They mirror
// the RBAC permission names so the model is consistent, plus stats:read.
var APIScopes = []string{
	"signals:read", "sources:read", "sources:write",
	"subscriptions:read", "subscriptions:write", "deliveries:read", "stats:read",
}

func validScope(s string) bool {
	for _, v := range APIScopes {
		if v == s {
			return true
		}
	}
	return false
}

// generateAPIKey returns a fresh raw key ("wsk_<43url-safe chars>"), its SHA-256
// hash, and a display prefix ("wsk_" + first 6 chars of the random part).
func generateAPIKey() (raw, hash, prefix string, err error) {
	tok, err := auth.GenerateToken()
	if err != nil {
		return "", "", "", err
	}
	raw = apiKeyPrefix + tok
	hash = crypto.SHA256Hex(raw)
	p := tok
	if len(p) > 6 {
		p = p[:6]
	}
	prefix = apiKeyPrefix + p
	return raw, hash, prefix, nil
}

// keyFromRequest extracts a presented API key from either the Authorization
// bearer header or the X-API-Key header.
func keyFromRequest(r *http.Request) string {
	if h := r.Header.Get("X-API-Key"); h != "" {
		return strings.TrimSpace(h)
	}
	if h := r.Header.Get("Authorization"); h != "" {
		if lower := strings.ToLower(h); strings.HasPrefix(lower, "bearer ") {
			return strings.TrimSpace(h[len("Bearer "):])
		}
	}
	return ""
}

// requireAPIKey wraps a /v1 handler with API-key authentication, scope
// authorization and per-key rate limiting. Failures return JSON with the
// appropriate status: 401 (missing/unknown key), 403 (disabled/expired/missing
// scope), 429 (rate limit exceeded).
func (s *Server) requireAPIKey(scope string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw := keyFromRequest(r)
		if raw == "" {
			apiKeyError(w, http.StatusUnauthorized, "missing API key (send it as 'Authorization: Bearer <key>' or 'X-API-Key: <key>')")
			return
		}
		key, err := s.DB.GetAPIKeyByHash(r.Context(), crypto.SHA256Hex(raw))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		// Constant-time-ish guard: reject unknown keys uniformly.
		if key == nil || subtle.ConstantTimeCompare([]byte(key.KeyHash), []byte(crypto.SHA256Hex(raw))) != 1 {
			apiKeyError(w, http.StatusUnauthorized, "invalid API key")
			return
		}
		now := time.Now()
		if !key.Enabled {
			apiKeyError(w, http.StatusForbidden, "API key is disabled")
			return
		}
		if key.ExpiresAt != nil && !now.Before(*key.ExpiresAt) {
			apiKeyError(w, http.StatusForbidden, "API key has expired")
			return
		}
		if !hasScope(key.Scopes, scope) {
			apiKeyError(w, http.StatusForbidden, "API key is missing the required scope: "+scope)
			return
		}

		limit := key.RateLimitPerMin
		if limit <= 0 {
			limit = 120
		}
		allowed, remaining, err := s.DB.AllowAPIRequest(r.Context(), key.ID, limit, now.Truncate(time.Minute))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		if !allowed {
			retry := 60 - now.Second()
			w.Header().Set("Retry-After", strconv.Itoa(retry))
			apiKeyError(w, http.StatusTooManyRequests, "rate limit exceeded; retry after the current minute")
			return
		}
		// Best-effort usage tracking (never blocks the response).
		_ = s.DB.TouchAPIKey(r.Context(), key.ID, now)
		next(w, r)
	}
}

func hasScope(scopes []string, want string) bool {
	for _, sc := range scopes {
		if sc == want {
			return true
		}
	}
	return false
}

func apiKeyError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, struct {
		Error string `json:"error"`
	}{msg})
}

// ---- admin management (settings:manage) ----

func (s *Server) registerAPIKeyResolvers(q, m map[string]gql.FieldResolver) {
	q["apiKeys"] = s.resolveAPIKeys
	q["apiScopes"] = s.resolveAPIScopes
	m["createApiKey"] = s.mutCreateAPIKey
	m["setApiKeyEnabled"] = s.mutSetAPIKeyEnabled
	m["deleteApiKey"] = s.mutDeleteAPIKey
}

func apiKeyToMap(k *db.ApiKey) map[string]any {
	return map[string]any{
		"id": k.ID, "name": k.Name, "keyPrefix": k.KeyPrefix, "scopes": strList(k.Scopes),
		"rateLimitPerMin": k.RateLimitPerMin, "enabled": k.Enabled,
		"expiresAt": timePtrT(k.ExpiresAt), "lastUsedAt": timePtrT(k.LastUsedAt),
		"requestCount": int(k.RequestCount), "createdBy": k.CreatedBy, "createdAt": k.CreatedAt,
	}
}

func (s *Server) resolveAPIKeys(ctx context.Context, _ map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSettingsManage); err != nil {
		return nil, err
	}
	keys, err := s.DB.ListAPIKeys(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]any, len(keys))
	for i, k := range keys {
		out[i] = apiKeyToMap(k)
	}
	return out, nil
}

func (s *Server) resolveAPIScopes(ctx context.Context, _ map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSettingsManage); err != nil {
		return nil, err
	}
	out := make([]any, len(APIScopes))
	for i, sc := range APIScopes {
		out[i] = sc
	}
	return out, nil
}

func (s *Server) mutCreateAPIKey(ctx context.Context, args map[string]any) (any, error) {
	id, err := auth.Require(ctx)
	if err != nil {
		return nil, err
	}
	if err := authz(ctx, auth.PermSettingsManage); err != nil {
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
			if !validScope(sc) {
				return nil, fmt.Errorf("%w: unknown scope %q", errValidation, sc)
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
	create := db.CreateAPIKeyInput{Name: name, Scopes: scopes, RateLimitPerMin: rate, CreatedBy: &id.UserID}
	if exp := strings.TrimSpace(strVal(in["expiresAt"])); exp != "" {
		if ts, perr := parseJSDate(exp); perr == nil {
			create.ExpiresAt = &ts
		} else {
			return nil, fmt.Errorf("%w: invalid expiresAt", errValidation)
		}
	}
	raw, hash, prefix, err := generateAPIKey()
	if err != nil {
		return nil, err
	}
	create.Hash, create.Prefix = hash, prefix
	k, err := s.DB.CreateAPIKey(ctx, cuid.New(), create)
	if err != nil {
		return nil, err
	}
	s.audit(ctx, "API_KEY_CREATED", "apiKey", k.ID, map[string]any{"name": name, "scopes": scopes})
	// The raw key is returned exactly once, here.
	out := apiKeyToMap(k)
	out["key"] = raw
	return out, nil
}

func (s *Server) mutSetAPIKeyEnabled(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSettingsManage); err != nil {
		return nil, err
	}
	enabled, _ := args["enabled"].(bool)
	k, err := s.DB.SetAPIKeyEnabled(ctx, strVal(args["id"]), enabled)
	if err != nil || k == nil {
		return nil, err
	}
	action := "API_KEY_ENABLED"
	if !enabled {
		action = "API_KEY_DISABLED"
	}
	s.audit(ctx, action, "apiKey", k.ID, map[string]any{"name": k.Name})
	return apiKeyToMap(k), nil
}

func (s *Server) mutDeleteAPIKey(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSettingsManage); err != nil {
		return nil, err
	}
	id := strVal(args["id"])
	ok, err := s.DB.DeleteAPIKey(ctx, id)
	if err != nil {
		return nil, err
	}
	if ok {
		s.audit(ctx, "API_KEY_DELETED", "apiKey", id, nil)
	}
	return ok, nil
}
