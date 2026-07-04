package httpapi_test

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/worldsignal/backend/internal/auth"
	"github.com/worldsignal/backend/internal/crypto"
	"github.com/worldsignal/backend/internal/cuid"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
)

// rawGet issues a GET with explicit headers (bypassing the auto-attached test key).
func rawGet(t *testing.T, base, path string, headers map[string]string) (int, string, http.Header) {
	t.Helper()
	req, _ := http.NewRequest("GET", base+path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), resp.Header
}

// seedKey inserts an API key with the given scopes + rate limit, returning the raw key.
func seedKey(t *testing.T, d *db.DB, scopes []string, rate int) string {
	t.Helper()
	raw := "wsk_" + cuid.New()
	_, err := d.Pool.Exec(context.Background(),
		`INSERT INTO "ApiKey" ("id","name","keyHash","keyPrefix","scopes","rateLimitPerMin","enabled") VALUES ($1,$2,$3,$4,$5,$6,true)`,
		cuid.New(), "k", crypto.SHA256Hex(raw), "wsk_x", scopes, rate)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestAPIKeyMiddleware(t *testing.T) {
	d := dbtest.Connect(t)
	seed(t, d) // resets + seeds a signal 'sg'
	ht, _ := newServer(t, d)
	apiKey = "" // don't auto-attach the full-scope test key

	// Missing key → 401.
	if code, body, _ := rawGet(t, ht.URL, "/v1/signals", nil); code != 401 || !strings.Contains(body, "missing API key") {
		t.Fatalf("missing key: %d %s", code, body)
	}
	// Unknown key → 401.
	if code, _, _ := rawGet(t, ht.URL, "/v1/signals", map[string]string{"X-API-Key": "wsk_nope"}); code != 401 {
		t.Fatalf("unknown key want 401 got %d", code)
	}
	// Valid key, wrong scope (has stats:read, needs signals:read) → 403.
	statsOnly := seedKey(t, d, []string{"stats:read"}, 100)
	if code, body, _ := rawGet(t, ht.URL, "/v1/signals", map[string]string{"X-API-Key": statsOnly}); code != 403 || !strings.Contains(body, "scope") {
		t.Fatalf("wrong scope: %d %s", code, body)
	}
	// stats key CAN hit /v1/stats.
	if code, _, _ := rawGet(t, ht.URL, "/v1/stats", map[string]string{"X-API-Key": statsOnly}); code != 200 {
		t.Fatalf("stats key on /v1/stats want 200 got %d", code)
	}
	// Right scope via Authorization: Bearer → 200 with rate-limit headers.
	readKey := seedKey(t, d, []string{"signals:read"}, 100)
	code, body, hdr := rawGet(t, ht.URL, "/v1/signals", map[string]string{"Authorization": "Bearer " + readKey})
	if code != 200 || !strings.Contains(body, `"id":"sg"`) {
		t.Fatalf("valid read: %d %s", code, body)
	}
	if hdr.Get("X-RateLimit-Limit") != "100" || hdr.Get("X-RateLimit-Remaining") == "" {
		t.Fatalf("rate-limit headers missing: %v", hdr)
	}
	// Disabled key → 403.
	disabled := seedKey(t, d, []string{"signals:read"}, 100)
	if _, err := d.Pool.Exec(context.Background(), `UPDATE "ApiKey" SET "enabled"=false WHERE "keyHash"=$1`, crypto.SHA256Hex(disabled)); err != nil {
		t.Fatal(err)
	}
	if code, body, _ := rawGet(t, ht.URL, "/v1/signals", map[string]string{"X-API-Key": disabled}); code != 403 || !strings.Contains(body, "disabled") {
		t.Fatalf("disabled key: %d %s", code, body)
	}
	// Expired key → 403.
	expired := seedKey(t, d, []string{"signals:read"}, 100)
	if _, err := d.Pool.Exec(context.Background(), `UPDATE "ApiKey" SET "expiresAt"=now()-interval '1 hour' WHERE "keyHash"=$1`, crypto.SHA256Hex(expired)); err != nil {
		t.Fatal(err)
	}
	if code, body, _ := rawGet(t, ht.URL, "/v1/signals", map[string]string{"X-API-Key": expired}); code != 403 || !strings.Contains(body, "expired") {
		t.Fatalf("expired key: %d %s", code, body)
	}
	// A key with a non-positive stored rate limit falls back to the default (120).
	zeroRate := seedKey(t, d, []string{"signals:read"}, 0)
	if _, _, hdr := rawGet(t, ht.URL, "/v1/signals", map[string]string{"X-API-Key": zeroRate}); hdr.Get("X-RateLimit-Limit") != "120" {
		t.Fatalf("zero rate should default to 120, got %q", hdr.Get("X-RateLimit-Limit"))
	}
	// /health stays open (no key).
	if code, _, _ := rawGet(t, ht.URL, "/health", nil); code != 200 {
		t.Fatalf("/health should be open, got %d", code)
	}
}

func TestAPIKeyRateLimit429(t *testing.T) {
	d := dbtest.Connect(t)
	seed(t, d)
	ht, _ := newServer(t, d)
	apiKey = ""
	key := seedKey(t, d, []string{"signals:read"}, 2) // 2 requests/min

	for i := 1; i <= 2; i++ {
		if code, _, _ := rawGet(t, ht.URL, "/v1/signals", map[string]string{"X-API-Key": key}); code != 200 {
			t.Fatalf("request %d want 200 got %d", i, code)
		}
	}
	code, body, hdr := rawGet(t, ht.URL, "/v1/signals", map[string]string{"X-API-Key": key})
	if code != 429 || !strings.Contains(body, "rate limit") {
		t.Fatalf("3rd want 429 got %d %s", code, body)
	}
	if hdr.Get("Retry-After") == "" || hdr.Get("X-RateLimit-Remaining") != "0" {
		t.Fatalf("429 headers: %v", hdr)
	}
}

func TestAPIKeyDBErrors(t *testing.T) {
	d := dbtest.Connect(t)
	seed(t, d)
	ht, _ := newServer(t, d)
	admin, _ := dbtest.AuthToken(t, d, auth.RoleAdmin)
	ctx := context.Background()

	// Rate-limit write failure → 500 (usage table gone, key still resolvable).
	key := seedKey(t, d, []string{"signals:read"}, 100)
	if _, err := d.Pool.Exec(ctx, `ALTER TABLE "ApiKeyUsage" RENAME TO "ApiKeyUsage__h"`); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := rawGet(t, ht.URL, "/v1/signals", map[string]string{"X-API-Key": key}); code != 500 {
		t.Fatalf("rate-limit write failure want 500 got %d", code)
	}
	if _, err := d.Pool.Exec(ctx, `ALTER TABLE "ApiKeyUsage__h" RENAME TO "ApiKeyUsage"`); err != nil {
		t.Fatal(err)
	}

	// Admin resolvers surface DB errors when the table is gone.
	bearer = admin
	defer func() { bearer = "" }()
	if _, err := d.Pool.Exec(ctx, `ALTER TABLE "ApiKey" RENAME TO "ApiKey__h"`); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = d.Pool.Exec(ctx, `ALTER TABLE "ApiKey__h" RENAME TO "ApiKey"`) }()
	for _, q := range []string{
		`{"query":"{ apiKeys{ id } }"}`,
		`{"query":"mutation($i:CreateApiKeyInput!){createApiKey(input:$i){id}}","variables":{"i":{"name":"n","scopes":["signals:read"]}}}`,
		`{"query":"mutation($id:ID!){setApiKeyEnabled(id:$id,enabled:false){id}}","variables":{"id":"x"}}`,
		`{"query":"mutation($id:ID!){deleteApiKey(id:$id)}","variables":{"id":"x"}}`,
	} {
		if _, body := postGQL(t, ht.URL, q); !strings.Contains(body, `"errors"`) {
			t.Fatalf("expected db error for %s: %s", q, body)
		}
	}
}

var apiKeyIDRe = regexp.MustCompile(`"createApiKey":\{[^}]*"id":"([^"]+)"`)

func TestAPIKeyAdminGraphQL(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ht, _ := newServer(t, d)
	admin, _ := dbtest.AuthToken(t, d, auth.RoleAdmin)
	viewer, _ := dbtest.AuthToken(t, d, auth.RoleViewer)

	// Scopes are listed.
	bearer = admin
	defer func() { bearer = "" }()
	if _, body := postGQL(t, ht.URL, `{"query":"{ apiScopes }"}`); !strings.Contains(body, "signals:read") {
		t.Fatalf("apiScopes: %s", body)
	}

	// Create returns the raw key exactly once.
	_, body := postGQL(t, ht.URL, `{"query":"mutation($i:CreateApiKeyInput!){createApiKey(input:$i){id name keyPrefix scopes key}}","variables":{"i":{"name":"Prod","scopes":["signals:read","stats:read"],"rateLimitPerMin":90}}}`)
	if !strings.Contains(body, `"key":"wsk_`) || !strings.Contains(body, `"keyPrefix":"wsk_`) {
		t.Fatalf("create should return raw key + prefix: %s", body)
	}
	m := apiKeyIDRe.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no id: %s", body)
	}
	id := m[1]

	// List never exposes the raw key or hash.
	if _, body := postGQL(t, ht.URL, `{"query":"{ apiKeys{ id name keyPrefix scopes enabled rateLimitPerMin } }"}`); !strings.Contains(body, `"name":"Prod"`) || strings.Contains(body, `"key":"wsk_`) || strings.Contains(body, "keyHash") {
		t.Fatalf("list leaked secret or missing key: %s", body)
	}
	// Disable + delete.
	if _, body := postGQL(t, ht.URL, `{"query":"mutation($id:ID!){setApiKeyEnabled(id:$id,enabled:false){enabled}}","variables":{"id":"`+id+`"}}`); !strings.Contains(body, `"enabled":false`) {
		t.Fatalf("disable: %s", body)
	}
	if _, body := postGQL(t, ht.URL, `{"query":"mutation($id:ID!){deleteApiKey(id:$id)}","variables":{"id":"`+id+`"}}`); !strings.Contains(body, `"deleteApiKey":true`) {
		t.Fatalf("delete: %s", body)
	}

	// Validation: no name / unknown scope / no scopes.
	for _, q := range []string{
		`{"query":"mutation($i:CreateApiKeyInput!){createApiKey(input:$i){id}}","variables":{"i":{"scopes":["signals:read"]}}}`,
		`{"query":"mutation($i:CreateApiKeyInput!){createApiKey(input:$i){id}}","variables":{"i":{"name":"x","scopes":["bogus:scope"]}}}`,
		`{"query":"mutation($i:CreateApiKeyInput!){createApiKey(input:$i){id}}","variables":{"i":{"name":"x","scopes":[]}}}`,
	} {
		if _, body := postGQL(t, ht.URL, q); !strings.Contains(body, "validation") {
			t.Fatalf("expected validation error: %s", body)
		}
	}

	// Expiry: valid future timestamp accepted; malformed rejected. Rate clamped to ≥1.
	bearer = admin
	if _, body := postGQL(t, ht.URL, `{"query":"mutation($i:CreateApiKeyInput!){createApiKey(input:$i){id keyPrefix}}","variables":{"i":{"name":"Exp","scopes":["signals:read"],"expiresAt":"2030-01-01T00:00:00Z","rateLimitPerMin":0}}}`); !strings.Contains(body, `"keyPrefix":"wsk_`) {
		t.Fatalf("create with expiry: %s", body)
	}
	if _, body := postGQL(t, ht.URL, `{"query":"mutation($i:CreateApiKeyInput!){createApiKey(input:$i){id}}","variables":{"i":{"name":"Bad","scopes":["signals:read"],"expiresAt":"nope"}}}`); !strings.Contains(body, "validation") {
		t.Fatalf("invalid expiry should be rejected: %s", body)
	}
	// The clamped key persisted with rateLimitPerMin=1.
	if _, body := postGQL(t, ht.URL, `{"query":"{ apiKeys{ name rateLimitPerMin } }"}`); !strings.Contains(body, `"name":"Exp"`) || !strings.Contains(body, `"rateLimitPerMin":1`) {
		t.Fatalf("rate clamp: %s", body)
	}

	// Creating without a session is unauthenticated.
	bearer = ""
	if _, body := postGQL(t, ht.URL, `{"query":"mutation($i:CreateApiKeyInput!){createApiKey(input:$i){id}}","variables":{"i":{"name":"x","scopes":["signals:read"]}}}`); !strings.Contains(body, "unauthenticated") {
		t.Fatalf("expected unauthenticated: %s", body)
	}

	// Viewer is forbidden from every apiKey operation.
	bearer = viewer
	for _, q := range []string{
		`{"query":"{ apiKeys{ id } }"}`,
		`{"query":"{ apiScopes }"}`,
		`{"query":"mutation($i:CreateApiKeyInput!){createApiKey(input:$i){id}}","variables":{"i":{"name":"x","scopes":["signals:read"]}}}`,
		`{"query":"mutation($id:ID!){setApiKeyEnabled(id:$id,enabled:true){id}}","variables":{"id":"x"}}`,
		`{"query":"mutation($id:ID!){deleteApiKey(id:$id)}","variables":{"id":"x"}}`,
	} {
		if _, body := postGQL(t, ht.URL, q); !strings.Contains(body, "forbidden") {
			t.Fatalf("viewer should be forbidden: %s", body)
		}
	}
}
