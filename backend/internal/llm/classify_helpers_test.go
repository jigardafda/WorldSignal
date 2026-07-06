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
