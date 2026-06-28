package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/worldsignal/backend/internal/auth"
	"github.com/worldsignal/backend/internal/crypto"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
)

// these tests are in package httpapi (white-box) to drive resolvers directly
// with an identity-bearing context and to override openAIModelsURL.

func adminCtx(t *testing.T, d *db.DB, role string) context.Context {
	t.Helper()
	_, u := dbtest.AuthToken(t, d, role)
	return auth.WithIdentity(context.Background(), &auth.Identity{UserID: u.ID, Role: role, Token: "tok"})
}

func mockOpenAI(t *testing.T, status int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(401)
			return
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	old := openAIModelsURL
	openAIModelsURL = srv.URL
	t.Cleanup(func() { openAIModelsURL = old })
}

func TestLLMKeyResolversLifecycle(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	mockOpenAI(t, http.StatusOK)
	s := &Server{DB: d, SigningSecret: "secret", OpenAIAPIKey: "env-key", OpenAIModel: "env-model"}
	ctx := adminCtx(t, d, auth.RoleAdmin)

	// Status with only the env key → ENV source, enabled.
	st, err := s.resolveLLMStatus(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if m := st.(map[string]any); m["source"] != "ENV" || m["enabled"] != true {
		t.Fatalf("status env: %+v", m)
	}

	// Create a key (auto-validated against the mocked OpenAI → VALID).
	created, err := s.mutCreateLLMKey(ctx, map[string]any{"input": map[string]any{"label": "Prod", "key": "sk-test-1234567890", "model": "gpt-4o"}})
	if err != nil {
		t.Fatal(err)
	}
	cm := created.(map[string]any)
	if cm["status"] != "VALID" || cm["keyLast4"] != "7890" {
		t.Fatalf("create: %+v", cm)
	}
	id := cm["id"].(string)

	// List shows it (masked — no plaintext key field).
	list, _ := s.resolveLLMKeys(ctx, nil)
	if len(list.([]any)) != 1 {
		t.Fatalf("list len: %v", list)
	}

	// Not active yet → ResolveLLMKey still returns env key.
	if k, m := s.ResolveLLMKey(context.Background()); k != "env-key" || m != "env-model" {
		t.Fatalf("expected env key before activation, got %s/%s", k, m)
	}

	// Activate → ResolveLLMKey returns the decrypted DB key + its model.
	if _, err := s.mutSetActiveLLMKey(ctx, map[string]any{"id": id}); err != nil {
		t.Fatal(err)
	}
	if k, m := s.ResolveLLMKey(context.Background()); k != "sk-test-1234567890" || m != "gpt-4o" {
		t.Fatalf("expected DB key after activation, got %s/%s", k, m)
	}

	// Status now reports DB source.
	st2, _ := s.resolveLLMStatus(ctx, nil)
	if st2.(map[string]any)["source"] != "DB" {
		t.Fatalf("status db: %+v", st2)
	}

	// Test the key (valid).
	tr, _ := s.mutTestLLMKey(ctx, map[string]any{"id": id})
	if tr.(map[string]any)["ok"] != true {
		t.Fatalf("test ok: %+v", tr)
	}

	// Delete → active gone, ResolveLLMKey falls back to env.
	del, _ := s.mutDeleteLLMKey(ctx, map[string]any{"id": id})
	if del != true {
		t.Fatalf("delete: %v", del)
	}
	if k, _ := s.ResolveLLMKey(context.Background()); k != "env-key" {
		t.Fatalf("expected env fallback after delete, got %s", k)
	}
}

func TestLLMKeyInvalidProviderReject(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	mockOpenAI(t, http.StatusUnauthorized) // provider rejects
	s := &Server{DB: d, SigningSecret: "secret"}
	ctx := adminCtx(t, d, auth.RoleAdmin)

	created, err := s.mutCreateLLMKey(ctx, map[string]any{"input": map[string]any{"label": "Bad", "key": "sk-bad-1234567890"}})
	if err != nil {
		t.Fatal(err)
	}
	if created.(map[string]any)["status"] != "INVALID" {
		t.Fatalf("expected INVALID on 401, got %+v", created)
	}

	// Validation errors.
	if _, err := s.mutCreateLLMKey(ctx, map[string]any{"input": map[string]any{"label": "", "key": "sk-x"}}); err == nil {
		t.Fatal("empty label should fail validation")
	}
	if _, err := s.mutCreateLLMKey(ctx, map[string]any{"input": map[string]any{"label": "X", "key": "short"}}); err == nil {
		t.Fatal("short key should fail validation")
	}
}

func TestLLMStatusNoneAndModelFallback(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	mockOpenAI(t, http.StatusOK)
	ctx := adminCtx(t, d, auth.RoleAdmin)

	// No env key, no DB key → source NONE, disabled.
	s := &Server{DB: d, SigningSecret: "secret"}
	st, _ := s.resolveLLMStatus(ctx, nil)
	if m := st.(map[string]any); m["source"] != "NONE" || m["enabled"] != false {
		t.Fatalf("expected NONE/disabled, got %+v", m)
	}
	// Activating a missing id returns nil.
	if k, err := s.mutSetActiveLLMKey(ctx, map[string]any{"id": "nope"}); err != nil || k != nil {
		t.Fatalf("missing activate: k=%v err=%v", k, err)
	}

	// A DB key WITHOUT a model → ResolveLLMKey keeps the env model.
	s2 := &Server{DB: d, SigningSecret: "secret", OpenAIAPIKey: "env-key", OpenAIModel: "env-model"}
	created, _ := s2.mutCreateLLMKey(ctx, map[string]any{"input": map[string]any{"label": "NoModel", "key": "sk-nomodel-123456"}})
	id := created.(map[string]any)["id"].(string)
	if _, err := s2.mutSetActiveLLMKey(ctx, map[string]any{"id": id}); err != nil {
		t.Fatal(err)
	}
	if k, m := s2.ResolveLLMKey(context.Background()); k != "sk-nomodel-123456" || m != "env-model" {
		t.Fatalf("expected DB key + env model, got %s/%s", k, m)
	}
}

func TestTestProviderKeyBranches(t *testing.T) {
	// Unsupported provider.
	if status, msg := testProviderKey(context.Background(), "ANTHROPIC", "k"); status != "INVALID" || msg == nil || !strings.Contains(*msg, "unsupported provider") {
		t.Fatalf("unsupported provider: %s %v", status, msg)
	}
	// Network failure (unreachable endpoint).
	old := openAIModelsURL
	openAIModelsURL = "http://127.0.0.1:0/models"
	defer func() { openAIModelsURL = old }()
	if status, msg := testOpenAIKey(context.Background(), "k"); status != "INVALID" || msg == nil || !strings.Contains(*msg, "request failed") {
		t.Fatalf("network failure: %s %v", status, msg)
	}
}

func TestLLMResolverDBErrors(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := adminCtx(t, d, auth.RoleAdmin)
	// Seed one key so mutations have a valid id, then hide the table.
	k, _ := d.CreateLLMKey(ctx, "err-k", db.CreateLLMKeyInput{Provider: "OPENAI", Label: "L", Ciphertext: "c", Last4: "zzzz"})
	if _, err := d.Pool.Exec(ctx, `ALTER TABLE "LLMKey" RENAME TO "LLMKey__h2"`); err != nil {
		t.Fatal(err)
	}
	defer d.Pool.Exec(ctx, `ALTER TABLE "LLMKey__h2" RENAME TO "LLMKey"`)

	s := &Server{DB: d, SigningSecret: "secret", OpenAIAPIKey: "env"}
	for name, fn := range map[string]func(context.Context, map[string]any) (any, error){
		"keys":   s.resolveLLMKeys,
		"status": s.resolveLLMStatus,
		"create": s.mutCreateLLMKey,
		"active": s.mutSetActiveLLMKey,
		"test":   s.mutTestLLMKey,
		"delete": s.mutDeleteLLMKey,
	} {
		if _, err := fn(ctx, map[string]any{"id": k.ID, "input": map[string]any{"label": "L", "key": "sk-abcdefgh"}}); err == nil {
			t.Fatalf("%s should surface the DB error", name)
		}
	}
}

func TestLLMKeyForbiddenForNonAdmin(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	s := &Server{DB: d, SigningSecret: "secret"}
	ctx := adminCtx(t, d, auth.RoleEditor) // editor lacks settings:manage
	for _, fn := range []func(context.Context, map[string]any) (any, error){
		s.resolveLLMKeys, s.resolveLLMStatus, s.mutCreateLLMKey, s.mutSetActiveLLMKey, s.mutTestLLMKey, s.mutDeleteLLMKey,
	} {
		if _, err := fn(ctx, map[string]any{"id": "x", "input": map[string]any{}}); err == nil {
			t.Fatal("editor should be forbidden from LLM key management")
		}
	}
}

func TestDecryptFailureOnTest(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := adminCtx(t, d, auth.RoleAdmin)
	// Store a key encrypted under a different secret, then test with the server's secret.
	other, _ := crypto.Encrypt("other-secret", "sk-something")
	s := &Server{DB: d, SigningSecret: "secret"}
	k, _ := d.CreateLLMKey(ctx, "k-dec", db.CreateLLMKeyInput{Provider: "OPENAI", Label: "L", Ciphertext: other, Last4: "ing"})
	res, err := s.mutTestLLMKey(ctx, map[string]any{"id": k.ID})
	if err != nil {
		t.Fatal(err)
	}
	m := res.(map[string]any)
	if m["ok"] != false || !strings.Contains(m["error"].(string), "decrypt") {
		t.Fatalf("expected decrypt failure, got %+v", m)
	}
}
