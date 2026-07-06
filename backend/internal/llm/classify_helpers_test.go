package llm

import "testing"

// TestClassifyHelpers covers the small classification helpers directly, including
// the boundary cases (empty keyword, domain-less code, stemming variants) that
// the corpus test does not reliably exercise.
func TestClassifyHelpers(t *testing.T) {
	if containsWord("hello world", "") {
		t.Error("empty keyword must not match")
	}
	if !containsWord("the quick brown fox", "quick") {
		t.Error("bounded word should match")
	}
	if containsWord("warmth spreads", "war") {
		t.Error("substring inside a word must not match on a boundary")
	}

	if domainOf("POLITICS") != "POLITICS" {
		t.Error("code without a dot should return itself")
	}
	if domainOf("DISASTER.FLOOD") != "DISASTER" {
		t.Error("dotted code should return its domain prefix")
	}

	if !isOtherLeaf("POLITICS.OTHER") {
		t.Error("POLITICS.OTHER is a domain catch-all")
	}
	if isOtherLeaf("GENERAL.OTHER") {
		t.Error("GENERAL.OTHER is the true fallback, not a domain catch-all")
	}
	if isOtherLeaf("DISASTER.FLOOD") {
		t.Error("a specific leaf is not an OTHER leaf")
	}

	// stemToken: plural 's', 'es', 'ies'->y, the 'ss' guard, and short words.
	for in, want := range map[string]string{
		"floods":    "flood",
		"markets":   "market",
		"companies": "company",
		"cases":     "cas",
		"business":  "business", // ss guard
		"press":     "press",    // ss guard
		"gas":       "gas",      // too short to stem
	} {
		if got := stemToken(in); got != want {
			t.Errorf("stemToken(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestClassifyTextBranches exercises the fallback branches of ClassifyText: a
// specific leaf subsuming its domain's `.OTHER`, an `.OTHER`-only match, and the
// no-match GENERAL fallback.
func TestClassifyTextBranches(t *testing.T) {
	// Specific leaf (POLITICS.ELECTIONS via "election") plus domain-level noise
	// ("political"): the specific tag wins and the domain's .OTHER is dropped.
	tags := ClassifyText("election result as political parties react", "")
	if tags[0].Code != "POLITICS.ELECTIONS" {
		t.Fatalf("want POLITICS.ELECTIONS primary, got %s", tags[0].Code)
	}
	for _, tg := range tags {
		if tg.Code == "POLITICS.OTHER" {
			t.Fatal("POLITICS.OTHER should be dropped when a specific POLITICS leaf matched")
		}
	}

	// Only domain-level keywords match → the domain's .OTHER catch-all, capped low.
	only := ClassifyText("the politician and party leader addressed the opposition", "")
	if only[0].Code != "POLITICS.OTHER" {
		t.Fatalf("want POLITICS.OTHER, got %s", only[0].Code)
	}
	if only[0].Confidence > 0.6 {
		t.Fatalf("OTHER-leaf confidence should be capped at 0.6, got %v", only[0].Confidence)
	}

	// Nothing matches → GENERAL.OTHER fallback.
	none := ClassifyText("xyzzy plugh frobnicate", "")
	if none[0].Code != "GENERAL.OTHER" {
		t.Fatalf("want GENERAL.OTHER fallback, got %s", none[0].Code)
	}

	// More than three domains match → the result is capped at 3 tags.
	many := ClassifyText("election earthquake stock market layoffs wildfire", "")
	if len(many) != 3 {
		t.Fatalf("expected at most 3 tags, got %d: %+v", len(many), many)
	}
}
