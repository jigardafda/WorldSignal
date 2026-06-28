package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/worldsignal/backend/internal/taxonomy"
)

// Gateway is the single point of contact with an LLM provider.
type Gateway interface {
	// Enabled reports whether completions can be made (an API key is set).
	Enabled() bool
	// JSONCompletion returns a strict JSON object, or nil if disabled/failed.
	JSONCompletion(ctx context.Context, system, user string, maxTokens int) ([]byte, error)
}

// OpenAIGateway calls the OpenAI chat completions API.
type OpenAIGateway struct {
	APIKey  string
	Model   string
	BaseURL string // defaults to the OpenAI API
	Client  *http.Client
}

// NewOpenAIGateway builds a gateway from a key and model.
func NewOpenAIGateway(apiKey, model string) *OpenAIGateway {
	return &OpenAIGateway{APIKey: apiKey, Model: model, Client: &http.Client{Timeout: 30 * time.Second}}
}

// Enabled reports whether a key is configured.
func (g *OpenAIGateway) Enabled() bool { return len(g.APIKey) > 0 }

// JSONCompletion posts a chat completion expecting a JSON object back.
func (g *OpenAIGateway) JSONCompletion(ctx context.Context, system, user string, maxTokens int) ([]byte, error) {
	if !g.Enabled() {
		return nil, nil
	}
	if maxTokens == 0 {
		maxTokens = 600
	}
	base := g.BaseURL
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	body, _ := json.Marshal(map[string]any{
		"model":           g.Model,
		"temperature":     0.1,
		"max_tokens":      maxTokens,
		"response_format": map[string]string{"type": "json_object"},
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.APIKey)
	client := g.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil // caller falls back to heuristic
	}
	defer resp.Body.Close()
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, nil
	}
	if len(parsed.Choices) == 0 || parsed.Choices[0].Message.Content == "" {
		return nil, nil
	}
	return []byte(parsed.Choices[0].Message.Content), nil
}

func buildTaxonomyList() string {
	var b strings.Builder
	for i, t := range taxonomy.LeafTags() {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "- %s (%s)", t.Code, t.Label)
	}
	return b.String()
}

type llmRaw struct {
	Title        *string   `json:"title"`
	Summary      *string   `json:"summary"`
	WhatHappened *string   `json:"whatHappened"`
	WhyItMatters *string   `json:"whyItMatters"`
	Severity     *string   `json:"severity"`
	Confidence   *float64  `json:"confidence"`
	Tags         []TagConf `json:"tags"`
}

var validSeverity = map[string]bool{"LOW": true, "MEDIUM": true, "HIGH": true, "CRITICAL": true}

// runLLM mirrors runLlm in enrich.ts: prompt, parse, validate, constrain tags.
func runLLM(ctx context.Context, gw Gateway, in EnrichInput) *EnrichmentResult {
	system := strings.Join([]string{
		"You are an analyst that turns a news article into a canonical event Signal.",
		"Return JSON only. Do not invent facts not present in the article.",
		"Choose tags ONLY from the provided taxonomy. Never create new tag codes.",
		"If nothing fits, use GENERAL.OTHER.",
		"",
		"Taxonomy:",
		buildTaxonomyList(),
	}, "\n")
	body := in.Body
	if len(body) > 6000 {
		body = body[:6000]
	}
	publisher := in.Publisher
	if publisher == "" {
		publisher = "unknown"
	}
	user := strings.Join([]string{
		"Produce JSON with keys: title, summary, whatHappened, whyItMatters,",
		"severity (LOW|MEDIUM|HIGH|CRITICAL), confidence (0..1),",
		"tags (array of {code, confidence}). Max 3 tags.",
		"",
		"PUBLISHER: " + publisher,
		"TITLE: " + in.Title,
		"BODY: " + body,
	}, "\n")

	raw, err := gw.JSONCompletion(ctx, system, user, 700)
	if err != nil || raw == nil {
		return nil
	}
	var p llmRaw
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil
	}
	// zod-equivalent validation: title and summary required.
	if p.Title == nil || p.Summary == nil {
		return nil
	}
	severity := "MEDIUM"
	if p.Severity != nil {
		if !validSeverity[*p.Severity] {
			return nil
		}
		severity = *p.Severity
	}
	confidence := 0.6
	if p.Confidence != nil {
		if *p.Confidence < 0 || *p.Confidence > 1 {
			return nil
		}
		confidence = *p.Confidence
	}

	var tags []TagConf
	for _, tg := range p.Tags {
		if _, ok := taxonomy.ValidCodes[tg.Code]; ok {
			tags = append(tags, tg)
			if len(tags) == 3 {
				break
			}
		}
	}
	if len(tags) == 0 {
		tags = []TagConf{{Code: taxonomy.FallbackCode, Confidence: 0.4}}
	}

	title := *p.Title
	if title == "" {
		title = in.Title
	}
	return &EnrichmentResult{
		Title:        title,
		Summary:      *p.Summary,
		WhatHappened: strDefault(p.WhatHappened, ""),
		WhyItMatters: strDefault(p.WhyItMatters, ""),
		Severity:     severity,
		Confidence:   confidence,
		Tags:         tags,
		Source:       "llm",
	}
}

func strDefault(s *string, def string) string {
	if s == nil {
		return def
	}
	return *s
}
