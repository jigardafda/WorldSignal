package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/worldsignal/backend/internal/auth"
	"github.com/worldsignal/backend/internal/crypto"
	"github.com/worldsignal/backend/internal/cuid"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/gql"
)

const llmProvider = "OPENAI"

// ResolveLLMKey returns the effective (key, model): the active admin-managed key
// if present and decryptable, else the system env key. Cached for 30s.
func (s *Server) ResolveLLMKey(ctx context.Context) (string, string) {
	s.llmCacheMu.Lock()
	defer s.llmCacheMu.Unlock()
	if time.Now().Before(s.llmCacheExp) {
		return s.llmCacheKey, s.llmCacheModel
	}
	key, model := s.OpenAIAPIKey, s.OpenAIModel
	if k, err := s.DB.GetActiveLLMKey(ctx, llmProvider); err == nil && k != nil {
		if pt, err := crypto.Decrypt(s.SigningSecret, k.KeyCiphertext); err == nil && pt != "" {
			key = pt
			if k.Model != nil && *k.Model != "" {
				model = *k.Model
			}
		}
	}
	s.llmCacheKey, s.llmCacheModel = key, model
	s.llmCacheExp = time.Now().Add(30 * time.Second)
	return key, model
}

// invalidateLLMCache forces the next ResolveLLMKey to re-read the DB (called
// after any key mutation so changes take effect immediately).
func (s *Server) invalidateLLMCache() {
	s.llmCacheMu.Lock()
	s.llmCacheExp = time.Time{}
	s.llmCacheMu.Unlock()
}

func (s *Server) registerLLMResolvers(q, m map[string]gql.FieldResolver) {
	q["llmKeys"] = s.resolveLLMKeys
	q["llmStatus"] = s.resolveLLMStatus
	m["createLLMKey"] = s.mutCreateLLMKey
	m["setActiveLLMKey"] = s.mutSetActiveLLMKey
	m["testLLMKey"] = s.mutTestLLMKey
	m["deleteLLMKey"] = s.mutDeleteLLMKey
}

func llmKeyToMap(k *db.LLMKey) map[string]any {
	return map[string]any{
		"id": k.ID, "provider": k.Provider, "label": k.Label, "keyLast4": k.KeyLast4,
		"model": k.Model, "isActive": k.IsActive, "status": k.Status,
		"lastTestedAt": timePtr(k.LastTestedAt), "lastError": k.LastError,
		"createdBy": k.CreatedBy, "createdAt": k.CreatedAt.Time, "updatedAt": k.UpdatedAt.Time,
	}
}

func (s *Server) resolveLLMKeys(ctx context.Context, _ map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSettingsManage); err != nil {
		return nil, err
	}
	keys, err := s.DB.ListLLMKeys(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]any, len(keys))
	for i, k := range keys {
		out[i] = llmKeyToMap(k)
	}
	return out, nil
}

// resolveLLMStatus reports the effective LLM configuration without leaking keys.
func (s *Server) resolveLLMStatus(ctx context.Context, _ map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSettingsManage); err != nil {
		return nil, err
	}
	active, err := s.DB.GetActiveLLMKey(ctx, llmProvider)
	if err != nil {
		return nil, err
	}
	key, model := s.ResolveLLMKey(ctx)
	source := "NONE"
	switch {
	case active != nil:
		source = "DB"
	case s.OpenAIAPIKey != "":
		source = "ENV"
	}
	return map[string]any{
		"provider":     llmProvider,
		"enabled":      key != "",
		"source":       source,
		"model":        model,
		"hasSystemKey": s.OpenAIAPIKey != "",
		"activeLabel":  llmActiveLabel(active),
	}, nil
}

func llmActiveLabel(k *db.LLMKey) any {
	if k == nil {
		return nil
	}
	return k.Label
}

func (s *Server) mutCreateLLMKey(ctx context.Context, args map[string]any) (any, error) {
	id, err := auth.Require(ctx)
	if err != nil {
		return nil, err
	}
	if err := authz(ctx, auth.PermSettingsManage); err != nil {
		return nil, err
	}
	in, _ := args["input"].(map[string]any)
	label := strings.TrimSpace(strVal(in["label"]))
	rawKey := strings.TrimSpace(strVal(in["key"]))
	if label == "" {
		return nil, fmt.Errorf("%w: label is required", errValidation)
	}
	if len(rawKey) < 8 {
		return nil, fmt.Errorf("%w: api key looks invalid", errValidation)
	}
	provider := strVal(in["provider"])
	if provider == "" {
		provider = llmProvider
	}
	cipher, err := crypto.Encrypt(s.SigningSecret, rawKey)
	if err != nil {
		return nil, err
	}
	create := db.CreateLLMKeyInput{
		Provider: provider, Label: label, Ciphertext: cipher, Last4: crypto.Last4(rawKey),
		CreatedBy: &id.UserID,
	}
	if model := strings.TrimSpace(strVal(in["model"])); model != "" {
		create.Model = &model
	}
	k, err := s.DB.CreateLLMKey(ctx, cuid.New(), create)
	if err != nil {
		return nil, err
	}
	// Validate immediately against the provider so the admin gets instant feedback.
	status, testErr := testProviderKey(ctx, provider, rawKey)
	_ = s.DB.UpdateLLMKeyStatus(ctx, k.ID, status, testErr)
	s.invalidateLLMCache()
	updated, err := s.DB.GetLLMKey(ctx, k.ID)
	if err != nil || updated == nil {
		return llmKeyToMap(k), nil
	}
	return llmKeyToMap(updated), nil
}

func (s *Server) mutSetActiveLLMKey(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSettingsManage); err != nil {
		return nil, err
	}
	k, err := s.DB.SetActiveLLMKey(ctx, strVal(args["id"]))
	if err != nil || k == nil {
		return nil, err
	}
	s.invalidateLLMCache()
	return llmKeyToMap(k), nil
}

func (s *Server) mutTestLLMKey(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSettingsManage); err != nil {
		return nil, err
	}
	k, err := s.DB.GetLLMKey(ctx, strVal(args["id"]))
	if err != nil || k == nil {
		return nil, err
	}
	plain, err := crypto.Decrypt(s.SigningSecret, k.KeyCiphertext)
	if err != nil {
		_ = s.DB.UpdateLLMKeyStatus(ctx, k.ID, "INVALID", strPtr("could not decrypt stored key"))
		return map[string]any{"ok": false, "status": "INVALID", "error": "could not decrypt stored key"}, nil
	}
	status, testErr := testProviderKey(ctx, k.Provider, plain)
	_ = s.DB.UpdateLLMKeyStatus(ctx, k.ID, status, testErr)
	s.invalidateLLMCache()
	out := map[string]any{"ok": status == "VALID", "status": status}
	if testErr != nil {
		out["error"] = *testErr
	}
	return out, nil
}

func (s *Server) mutDeleteLLMKey(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSettingsManage); err != nil {
		return nil, err
	}
	ok, err := s.DB.DeleteLLMKey(ctx, strVal(args["id"]))
	if err != nil {
		return nil, err
	}
	s.invalidateLLMCache()
	return ok, nil
}

func strPtr(s string) *string { return &s }

// testProviderKey validates a key against its provider. For OpenAI it performs a
// cheap GET /v1/models (no token consumption). Returns ("VALID"|"INVALID", err).
func testProviderKey(ctx context.Context, provider, key string) (string, *string) {
	switch provider {
	case llmProvider, "":
		return testOpenAIKey(ctx, key)
	default:
		msg := "unsupported provider: " + provider
		return "INVALID", &msg
	}
}

// openAIModelsURL is the validation endpoint; overridable in tests.
var openAIModelsURL = "https://api.openai.com/v1/models"

func testOpenAIKey(ctx context.Context, key string) (string, *string) {
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, openAIModelsURL, nil)
	if err != nil {
		msg := err.Error()
		return "INVALID", &msg
	}
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		msg := "request failed: " + err.Error()
		return "INVALID", &msg
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return "VALID", nil
	}
	msg := fmt.Sprintf("provider rejected key (HTTP %d)", resp.StatusCode)
	return "INVALID", &msg
}
