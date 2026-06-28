package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
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
	q["llmModels"] = s.resolveLLMModels
	q["auditLogs"] = s.resolveAuditLogs
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
	s.audit(ctx, "LLM_KEY_CREATED", "llmKey", k.ID, map[string]any{"label": label, "provider": provider, "status": status})
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
	s.audit(ctx, "LLM_KEY_ACTIVATED", "llmKey", k.ID, map[string]any{"label": k.Label})
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
	id := strVal(args["id"])
	ok, err := s.DB.DeleteLLMKey(ctx, id)
	if err != nil {
		return nil, err
	}
	s.invalidateLLMCache()
	if ok {
		s.audit(ctx, "LLM_KEY_DELETED", "llmKey", id, nil)
	}
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

// openAIModelsURL is the models/validation endpoint; overridable in tests.
var openAIModelsURL = "https://api.openai.com/v1/models"

// resolveLLMModels lists chat-capable models from the provider, using the
// effective key. Returns [] when no key is configured so the UI degrades
// gracefully (e.g. before the first key is added on a fresh install).
func (s *Server) resolveLLMModels(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSettingsManage); err != nil {
		return nil, err
	}
	// Prefer a key the admin is testing/just supplied; else the effective key.
	key := strings.TrimSpace(strVal(args["key"]))
	if key == "" {
		key, _ = s.ResolveLLMKey(ctx)
	}
	if key == "" {
		return []any{}, nil
	}
	models, err := listOpenAIModels(ctx, key)
	if err != nil {
		return []any{}, nil // surface as "no models" rather than a hard error
	}
	out := make([]any, len(models))
	for i, m := range models {
		out[i] = m
	}
	return out, nil
}

// listOpenAIModels fetches model ids and keeps only chat-completion-capable ones,
// newest-looking first (reverse lexical, which puts gpt-4.x / o-series on top).
func listOpenAIModels(ctx context.Context, key string) ([]string, error) {
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, openAIModelsURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("provider returned HTTP %d", resp.StatusCode)
	}
	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	var ids []string
	for _, m := range parsed.Data {
		if isChatModel(m.ID) {
			ids = append(ids, m.ID)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	return ids, nil
}

// isChatModel keeps chat/completions families and drops embeddings, audio,
// image, moderation and realtime variants that can't drive enrichment.
func isChatModel(id string) bool {
	switch {
	case strings.Contains(id, "embedding"), strings.Contains(id, "whisper"),
		strings.Contains(id, "tts"), strings.Contains(id, "dall-e"),
		strings.Contains(id, "audio"), strings.Contains(id, "realtime"),
		strings.Contains(id, "moderation"), strings.Contains(id, "image"),
		strings.Contains(id, "transcribe"), strings.Contains(id, "search"):
		return false
	}
	return strings.HasPrefix(id, "gpt-") || strings.HasPrefix(id, "chatgpt") ||
		strings.HasPrefix(id, "o1") || strings.HasPrefix(id, "o3") || strings.HasPrefix(id, "o4")
}

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
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusOK {
		return "VALID", nil
	}
	msg := fmt.Sprintf("provider rejected key (HTTP %d)", resp.StatusCode)
	return "INVALID", &msg
}
