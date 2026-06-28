package llm

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestLiveOpenAIEnrichment exercises the real OpenAI API end to end. It is
// skipped unless OPENAI_LIVE_TEST=1 and OPENAI_API_KEY are set, so normal CI
// runs offline. Run it once after configuring a key:
//
//	OPENAI_LIVE_TEST=1 OPENAI_API_KEY=sk-... go test ./internal/llm -run Live -v
func TestLiveOpenAIEnrichment(t *testing.T) {
	if os.Getenv("OPENAI_LIVE_TEST") == "" {
		t.Skip("set OPENAI_LIVE_TEST=1 (and OPENAI_API_KEY) to run the live OpenAI test")
	}
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		t.Fatal("OPENAI_LIVE_TEST set but OPENAI_API_KEY is empty")
	}
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}

	gw := NewOpenAIGateway(key, model)
	if !gw.Enabled() {
		t.Fatal("gateway should be enabled with a key")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	res := EnrichArticle(ctx, gw, EnrichInput{
		Title:     "Magnitude 6.2 earthquake strikes off the coast, tsunami advisory issued",
		Body:      "A strong magnitude 6.2 earthquake struck offshore early Tuesday. Authorities issued a tsunami advisory for coastal areas and urged residents to move to higher ground. No casualties have been reported yet.",
		Publisher: "Reuters",
	})

	if res.Source != "llm" {
		t.Fatalf("expected llm-sourced result, got %q (LLM call failed or fell back to heuristic)", res.Source)
	}
	if res.Summary == "" || res.Title == "" {
		t.Fatalf("expected non-empty title/summary, got %+v", res)
	}
	if len(res.Tags) == 0 {
		t.Fatal("expected at least one taxonomy tag")
	}
	t.Logf("LIVE OpenAI OK: severity=%s confidence=%.2f tags=%v", res.Severity, res.Confidence, res.Tags)
}
