package llm

import (
	"context"
	"testing"
)

type fakeGateway struct {
	enabled bool
	out     []byte
	err     error
}

func (f fakeGateway) Enabled() bool { return f.enabled }
func (f fakeGateway) JSONCompletion(context.Context, string, string, int) ([]byte, error) {
	return f.out, f.err
}

func TestHeuristicTagsAndSeverity(t *testing.T) {
	r := EnrichArticle(context.Background(), nil, EnrichInput{
		Title: "Major earthquake",
		Body:  "A powerful earthquake and flooding killed many; magnitude 7.",
	})
	if r.Source != "heuristic" {
		t.Fatalf("source %s", r.Source)
	}
	if r.Severity != "HIGH" {
		t.Fatalf("severity %s", r.Severity)
	}
	if len(r.Tags) == 0 || r.Confidence != 0.45 {
		t.Fatalf("tags/conf wrong: %+v", r)
	}
	// Highest-scoring tag should be the earthquake/flood disaster code.
	if r.Tags[0].Code[:8] != "DISASTER" {
		t.Fatalf("top tag %s", r.Tags[0].Code)
	}
}

func TestHeuristicFallbackAndSummary(t *testing.T) {
	r := EnrichArticle(context.Background(), nil, EnrichInput{Title: "Plain headline", Body: ""})
	if len(r.Tags) != 1 || r.Tags[0].Code != "GENERAL.OTHER" {
		t.Fatalf("expected fallback tag, got %+v", r.Tags)
	}
	if r.Severity != "MEDIUM" {
		t.Fatalf("severity %s", r.Severity)
	}
	if r.Summary != "Plain headline" { // empty body → summary falls back to title
		t.Fatalf("summary %q", r.Summary)
	}
}

func TestLLMPathValid(t *testing.T) {
	gw := fakeGateway{enabled: true, out: []byte(`{"title":"T","summary":"S","whatHappened":"W","whyItMatters":"Y","severity":"CRITICAL","confidence":0.9,"tags":[{"code":"DISASTER.FLOOD","confidence":0.8},{"code":"NOPE.NOPE","confidence":0.5}]}`)}
	r := EnrichArticle(context.Background(), gw, EnrichInput{Title: "x", Body: "y"})
	if r.Source != "llm" || r.Severity != "CRITICAL" || r.Confidence != 0.9 {
		t.Fatalf("llm result wrong: %+v", r)
	}
	if len(r.Tags) != 1 || r.Tags[0].Code != "DISASTER.FLOOD" {
		t.Fatalf("invalid tag should be filtered: %+v", r.Tags)
	}
}

func TestLLMTranslationFlag(t *testing.T) {
	// Non-English source → translated, with the English narrative the LLM returned.
	fr := fakeGateway{enabled: true, out: []byte(`{"title":"Quake","summary":"S","language":"fr-FR"}`)}
	r := EnrichArticle(context.Background(), fr, EnrichInput{Title: "Séisme", Body: "y"})
	if r.Language != "fr" || !r.Translated {
		t.Fatalf("french source should be translated: lang=%q translated=%v", r.Language, r.Translated)
	}
	if r.Title != "Quake" {
		t.Fatalf("title should be the English translation: %q", r.Title)
	}
	// English source → not translated.
	en := fakeGateway{enabled: true, out: []byte(`{"title":"T","summary":"S","language":"en"}`)}
	if r := EnrichArticle(context.Background(), en, EnrichInput{Title: "x", Body: "y"}); r.Translated || r.Language != "en" {
		t.Fatalf("english source should not be translated: %+v", r)
	}
	// Missing/garbage language → empty, not translated.
	none := fakeGateway{enabled: true, out: []byte(`{"title":"T","summary":"S","language":"xyz"}`)}
	if r := EnrichArticle(context.Background(), none, EnrichInput{Title: "x", Body: "y"}); r.Translated || r.Language != "" {
		t.Fatalf("garbage language should be dropped: %+v", r)
	}
}

func TestNormalizeLang(t *testing.T) {
	cases := map[string]string{
		"en": "en", "EN": "en", " fr ": "fr", "en-US": "en", "pt_BR": "pt",
		"english": "", "e": "", "": "", "e1": "", "zh-Hans-CN": "zh",
	}
	for in, want := range cases {
		if got := normalizeLang(in); got != want {
			t.Errorf("normalizeLang(%q) = %q want %q", in, got, want)
		}
	}
}

func TestLLMPathEmptyTagsFallback(t *testing.T) {
	gw := fakeGateway{enabled: true, out: []byte(`{"title":"T","summary":"S","tags":[{"code":"BAD","confidence":0.5}]}`)}
	r := EnrichArticle(context.Background(), gw, EnrichInput{Title: "x", Body: "y"})
	if len(r.Tags) != 1 || r.Tags[0].Code != "GENERAL.OTHER" {
		t.Fatalf("expected fallback, got %+v", r.Tags)
	}
	if r.Severity != "MEDIUM" || r.Confidence != 0.6 { // defaults
		t.Fatalf("defaults wrong: %+v", r)
	}
}

func TestLLMPathInvalidFallsBackToHeuristic(t *testing.T) {
	cases := [][]byte{
		[]byte(`{"summary":"no title"}`),                       // missing title
		[]byte(`{"title":"t","summary":"s","severity":"WAT"}`), // bad severity
		[]byte(`{"title":"t","summary":"s","confidence":2}`),   // bad confidence
		[]byte(`not json`),                                     // unparseable
		nil,                                                    // gateway returned nothing
	}
	for i, out := range cases {
		gw := fakeGateway{enabled: true, out: out}
		r := EnrichArticle(context.Background(), gw, EnrichInput{Title: "earthquake", Body: "earthquake struck"})
		if r.Source != "heuristic" {
			t.Fatalf("case %d should fall back to heuristic, got %s", i, r.Source)
		}
	}
}

func TestLLMTitleFallbackWhenEmpty(t *testing.T) {
	gw := fakeGateway{enabled: true, out: []byte(`{"title":"","summary":"S"}`)}
	r := EnrichArticle(context.Background(), gw, EnrichInput{Title: "Original", Body: "y"})
	if r.Title != "Original" {
		t.Fatalf("empty title should fall back to input: %q", r.Title)
	}
}

func TestLLMLongBodyTruncated(t *testing.T) {
	body := ""
	for i := 0; i < 7000; i++ {
		body += "a"
	}
	gw := fakeGateway{enabled: true, out: []byte(`{"title":"T","summary":"S"}`)}
	// Should not panic and should produce an llm result.
	if r := EnrichArticle(context.Background(), gw, EnrichInput{Title: "t", Body: body}); r.Source != "llm" {
		t.Fatalf("source %s", r.Source)
	}
}

func TestLLMGatewayError(t *testing.T) {
	gw := fakeGateway{enabled: true, err: context.DeadlineExceeded}
	if r := EnrichArticle(context.Background(), gw, EnrichInput{Title: "x", Body: "y"}); r.Source != "heuristic" {
		t.Fatalf("gateway error should fall back, got %s", r.Source)
	}
}
