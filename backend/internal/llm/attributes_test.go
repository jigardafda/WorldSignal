package llm

import (
	"context"
	"testing"
)

func TestExtractHeuristic(t *testing.T) {
	r := ExtractAttributes(context.Background(), nil, AttrInput{
		Title: "Major cyberattack and data breach hits bank",
		Body:  "A ransomware attack killed services at a major bank; cybersecurity teams responded.",
	})
	if r.Source != "heuristic" {
		t.Fatalf("source = %s", r.Source)
	}
	if r.Sentiment != "NEGATIVE" || r.SentimentScore >= 0 {
		t.Fatalf("expected negative sentiment from severity cues: %+v", r)
	}
	if r.Influence != "HIGH" {
		t.Fatalf("expected HIGH influence: %+v", r)
	}
	if !contains(r.Industries, "CYBERSECURITY") || !contains(r.Industries, "BANKING") {
		t.Fatalf("expected industries via alias match, got %v", r.Industries)
	}
}

func TestExtractHeuristicNeutral(t *testing.T) {
	r := ExtractAttributes(context.Background(), nil, AttrInput{Title: "Quarterly retail sales steady", Body: "Retail sales were flat."})
	if r.Sentiment != "NEUTRAL" || r.SentimentScore != 0 || r.Relevance != 0.5 {
		t.Fatalf("neutral defaults wrong: %+v", r)
	}
	if !contains(r.Industries, "RETAIL") {
		t.Fatalf("expected RETAIL, got %v", r.Industries)
	}
}

func TestExtractLLMValid(t *testing.T) {
	out := []byte(`{
	  "country":"United States","region":"California","city":"Los Angeles","locality":"Downtown",
	  "geoScope":"local","sentiment":"negative","sentimentScore":-0.7,"influence":"high","relevance":0.9,
	  "industries":["AI","cybersecurity","time travel"],
	  "entities":[{"name":"Acme Corp","type":"company"},{"name":"Jane Doe","type":"person"},{"name":"","type":"x"},{"name":"Acme Corp","type":"company"}]
	}`)
	gw := fakeGateway{enabled: true, out: out}
	r := ExtractAttributes(context.Background(), gw, AttrInput{Title: "x", Body: "y", Context: "richer page text"})
	if r.Source != "llm" {
		t.Fatalf("source = %s", r.Source)
	}
	if r.Country != "US" || r.Region != "California" || r.City != "Los Angeles" || r.Locality != "Downtown" {
		t.Fatalf("geo wrong: %+v", r)
	}
	if r.GeoScope != "LOCAL" || r.Sentiment != "NEGATIVE" || r.Influence != "HIGH" {
		t.Fatalf("enums not normalized: %+v", r)
	}
	if r.SentimentScore != -0.7 || r.Relevance != 0.9 {
		t.Fatalf("scalars wrong: %+v", r)
	}
	// "time travel" dropped; AI -> ARTIFICIAL_INTELLIGENCE; cybersecurity kept.
	if len(r.Industries) != 2 || !contains(r.Industries, "ARTIFICIAL_INTELLIGENCE") || !contains(r.Industries, "CYBERSECURITY") {
		t.Fatalf("industries not normalized/filtered: %v", r.Industries)
	}
	// entity dedupe + empty-name drop + type normalization.
	if len(r.Entities) != 2 {
		t.Fatalf("expected 2 entities, got %+v", r.Entities)
	}
	if r.Entities[0].Type != "ORGANIZATION" || r.Entities[1].Type != "PERSON" {
		t.Fatalf("entity types not normalized: %+v", r.Entities)
	}
}

func TestExtractLLMClampsAndDropsUnknownEnums(t *testing.T) {
	out := []byte(`{"country":"Atlantis","geoScope":"galactic","sentiment":"???","sentimentScore":5,"relevance":2,"influence":"bad"}`)
	gw := fakeGateway{enabled: true, out: out}
	r := ExtractAttributes(context.Background(), gw, AttrInput{Title: "x", Body: "y"})
	if r.Country != "" || r.GeoScope != "" || r.Sentiment != "" || r.Influence != "" {
		t.Fatalf("unknown enums/country should be dropped: %+v", r)
	}
	if r.SentimentScore != 1 || r.Relevance != 1 {
		t.Fatalf("scalars should clamp to max: %+v", r)
	}
}

func TestExtractLLMRelevanceDefaultWhenMissing(t *testing.T) {
	gw := fakeGateway{enabled: true, out: []byte(`{"sentiment":"neutral"}`)}
	r := ExtractAttributes(context.Background(), gw, AttrInput{Title: "x", Body: "y"})
	if r.Relevance != 0.5 {
		t.Fatalf("missing relevance should default to 0.5, got %v", r.Relevance)
	}
}

func TestExtractLLMInvalidFallsBackToHeuristic(t *testing.T) {
	for i, out := range [][]byte{[]byte(`not json`), nil} {
		gw := fakeGateway{enabled: true, out: out}
		r := ExtractAttributes(context.Background(), gw, AttrInput{Title: "earthquake", Body: "earthquake killed many"})
		if r.Source != "heuristic" {
			t.Fatalf("case %d: expected heuristic fallback, got %s", i, r.Source)
		}
	}
}

func TestExtractLLMGatewayError(t *testing.T) {
	gw := fakeGateway{enabled: true, err: context.DeadlineExceeded}
	if r := ExtractAttributes(context.Background(), gw, AttrInput{Title: "x", Body: "y"}); r.Source != "heuristic" {
		t.Fatalf("gateway error should fall back, got %s", r.Source)
	}
}

func TestExtractLLMLongContextTruncated(t *testing.T) {
	ctxText := ""
	for i := 0; i < 7000; i++ {
		ctxText += "a"
	}
	gw := fakeGateway{enabled: true, out: []byte(`{"sentiment":"neutral"}`)}
	if r := ExtractAttributes(context.Background(), gw, AttrInput{Title: "t", Context: ctxText}); r.Source != "llm" {
		t.Fatalf("source %s", r.Source)
	}
}

func TestAttributesPromptDigest(t *testing.T) {
	d := AttributesPromptDigest()
	for _, want := range []string{"GLOBAL", "POSITIVE", "CRITICAL", "PERSON"} {
		if !containsStr(d, want) {
			t.Errorf("digest missing %q: %s", want, d)
		}
	}
}

func TestAllowedListUnknownKey(t *testing.T) {
	if got := allowedList("nope"); got != "" {
		t.Errorf("unknown key should yield empty list, got %q", got)
	}
}

func TestNormEnumEdgeCases(t *testing.T) {
	if got := normEnum("sentiment", nil); got != "" {
		t.Errorf("nil raw should yield empty, got %q", got)
	}
	if got := normEnum("nope", ptrStr("x")); got != "" {
		t.Errorf("unknown key should yield empty, got %q", got)
	}
	bad := "definitely-not-a-sentiment"
	if got := normEnum("sentiment", &bad); got != "" {
		t.Errorf("out-of-vocab should yield empty, got %q", got)
	}
}

func ptrStr(s string) *string { return &s }

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func containsStr(hay, needle string) bool {
	return len(hay) >= len(needle) && (indexOf(hay, needle) >= 0)
}

func indexOf(hay, needle string) int {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if hay[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
