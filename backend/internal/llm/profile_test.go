package llm

import (
	"context"
	"strings"
	"testing"
)

func TestDraftHeuristicFromDocument(t *testing.T) {
	doc := `Nike Media Kit 2026. Nike is the world's leading athletic footwear brand.
	Our sponsored sprinter Marcus Vale broke the national record. Key markets are the
	United States and India. This season's marathon and championship campaigns focus on
	elite running and the Olympics.`
	d := DraftProfileFromDocument(context.Background(), nil, doc) // nil gateway → heuristic
	if d.Source != "heuristic" {
		t.Fatalf("expected heuristic source, got %q", d.Source)
	}
	if len(d.Interests) == 0 {
		t.Fatal("draft should produce interests")
	}
	// A sports topic should be picked up from the text.
	hasSportsTopic := false
	for k := range d.Interests {
		if strings.HasPrefix(k, "tag:SPORTS") {
			hasSportsTopic = true
		}
	}
	if !hasSportsTopic {
		t.Fatalf("expected a SPORTS topic interest, got %v", d.Interests)
	}
	// Prominent proper nouns should surface as entity interests (deterministically).
	entities := 0
	for k := range d.Interests {
		if strings.HasPrefix(k, "entity:") {
			entities++
		}
	}
	if entities == 0 {
		t.Fatalf("expected at least one entity interest, got %v", d.Interests)
	}
	// Deterministic: the same document always drafts the same interests.
	d2 := DraftProfileFromDocument(context.Background(), nil, doc)
	if len(d2.Interests) != len(d.Interests) {
		t.Fatalf("heuristic must be deterministic: %v vs %v", d.Interests, d2.Interests)
	}
	for k, v := range d.Interests {
		if d2.Interests[k] != v {
			t.Fatalf("non-deterministic draft: %q %v vs %v", k, v, d2.Interests[k])
		}
	}
	if d.MinScore <= 0 || d.MinSeverity == "" {
		t.Fatal("draft should carry default gates")
	}
}

// fakeGW returns a canned draft JSON to exercise the LLM parse/validation path.
type fakeGW struct{ resp string }

func (f fakeGW) Enabled() bool { return true }
func (f fakeGW) JSONCompletion(_ context.Context, _, _ string, _ int) ([]byte, error) {
	return []byte(f.resp), nil
}

func TestDraftLLMParsesAndConstrains(t *testing.T) {
	// Includes one bogus tag (NOPE.NOPE) that must be dropped, and a valid one.
	gw := fakeGW{resp: `{"name":"Sponsorship risk","summary":"s",
	  "interests":{"entity:Marcus Vale":5,"tag:CRIME":3,"tag:NOPE.NOPE":4,"country:US":2,"sentiment:NEGATIVE":2},
	  "minScore":7,"minSeverity":"HIGH",
	  "reasons":[{"key":"entity:Marcus Vale","why":"sponsored athlete","origin":"doc"},
	             {"key":"tag:NOPE.NOPE","why":"bogus","origin":"inferred"}]}`}
	d := DraftProfileFromDocument(context.Background(), gw, "doc")
	if d.Source != "llm" {
		t.Fatalf("expected llm source, got %q", d.Source)
	}
	if _, ok := d.Interests["tag:NOPE.NOPE"]; ok {
		t.Fatal("invalid taxonomy code must be dropped")
	}
	if d.Interests["tag:CRIME"] != 3 || d.Interests["entity:Marcus Vale"] != 5 {
		t.Fatalf("valid interests missing/altered: %v", d.Interests)
	}
	if d.MinScore != 7 || d.MinSeverity != "HIGH" {
		t.Fatalf("gates not parsed: %v %v", d.MinScore, d.MinSeverity)
	}
	// Reasons for dropped interests must be filtered out.
	for _, r := range d.Reasons {
		if r.Key == "tag:NOPE.NOPE" {
			t.Fatal("reason for a dropped interest should be removed")
		}
	}
}

func TestDraftLLMFallsBackWhenEmpty(t *testing.T) {
	// LLM returns only invalid interests → fall back to the heuristic on the doc.
	gw := fakeGW{resp: `{"name":"x","interests":{"tag:NOPE.NOPE":3},"reasons":[]}`}
	d := DraftProfileFromDocument(context.Background(), gw, "Nike and Adidas compete in sports.")
	if d.Source != "heuristic" {
		t.Fatalf("expected heuristic fallback, got %q", d.Source)
	}
}
