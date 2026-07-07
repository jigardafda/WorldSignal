package relevance

import "testing"

func baseSignal() Signal {
	return Signal{
		ID: "s", Title: "Quake hits coast", Summary: "A big earthquake struck.",
		EventType: "DISASTER.EARTHQUAKE", Tags: []string{"DISASTER.EARTHQUAKE"},
		Country: "IN", Region: "Maharashtra", Entities: []string{"NDMA"},
		Sentiment: "NEGATIVE", Influence: "HIGH", Severity: "HIGH",
		Relevance: 0.8, Confidence: 0.9, AgeHours: 1,
	}
}

func TestInterestMatchBoostsScore(t *testing.T) {
	s := baseSignal()
	with := Score(Profile{Interests: map[string]float64{"tag:DISASTER.EARTHQUAKE": 2}}, s)
	without := Score(Profile{}, s)
	if with.Score <= without.Score {
		t.Fatalf("matched interest should score higher: with=%.3f without=%.3f", with.Score, without.Score)
	}
	if len(with.Reasons) == 0 {
		t.Fatal("a matched interest should produce a reason")
	}
}

func TestDomainLevelTagMatches(t *testing.T) {
	// A domain-level interest (tag:DISASTER) matches a specific leaf eventType.
	s := baseSignal()
	got := Score(Profile{Interests: map[string]float64{"tag:DISASTER": 1.5}}, s)
	if got.Score <= Score(Profile{}, s).Score {
		t.Fatalf("domain-level tag interest should match a leaf eventType")
	}
}

func TestEntityAndKeywordMatch(t *testing.T) {
	s := baseSignal()
	ent := Score(Profile{Interests: map[string]float64{"entity:ndma": 3}}, s)
	if ent.Score <= Score(Profile{}, s).Score {
		t.Fatal("entity interest should match (case-insensitive)")
	}
	kw := Score(Profile{Keywords: []string{"earthquake"}}, s)
	if kw.Score <= Score(Profile{}, s).Score {
		t.Fatal("keyword in title/summary should boost score")
	}
}

func TestCountryInterestMatch(t *testing.T) {
	s := baseSignal()
	in := Score(Profile{Interests: map[string]float64{"country:IN": 2}}, s)
	us := Score(Profile{Interests: map[string]float64{"country:US": 2}}, s)
	if in.Score <= us.Score {
		t.Fatalf("matching country should outscore a non-matching one: IN=%.3f US=%.3f", in.Score, us.Score)
	}
}

func TestRecencyDecay(t *testing.T) {
	p := Profile{Interests: map[string]float64{"tag:DISASTER": 1}}
	fresh := baseSignal()
	fresh.AgeHours = 1
	old := baseSignal()
	old.AgeHours = 240 // 10 days
	if Score(p, fresh).Score <= Score(p, old).Score {
		t.Fatal("a fresher signal should outrank an older one with equal match")
	}
}

func TestQualityMattersWhenNoInterestMatches(t *testing.T) {
	// Empty profile → still ranks by intrinsic quality × recency.
	p := Profile{}
	strong := baseSignal() // HIGH influence/severity, relevance 0.8
	weak := baseSignal()
	weak.Influence, weak.Severity, weak.Relevance, weak.Confidence = "LOW", "LOW", 0.2, 0.3
	if Score(p, strong).Score <= Score(p, weak).Score {
		t.Fatal("higher-quality signal should rank above a weak one for an empty profile")
	}
}

func TestRankSortsDescendingStable(t *testing.T) {
	p := Profile{Interests: map[string]float64{"tag:DISASTER": 2}}
	a := baseSignal()
	a.ID = "match"
	b := baseSignal()
	b.ID = "nomatch"
	b.EventType, b.Tags = "SPORTS.RESULT", []string{"SPORTS.RESULT"}
	ranked := Rank(p, []Signal{b, a})
	if len(ranked) != 2 || ranked[0].ID != "match" {
		t.Fatalf("expected matching signal first, got %+v", []string{ranked[0].ID, ranked[1].ID})
	}
	// scores are non-increasing
	if ranked[0].Score < ranked[1].Score {
		t.Fatal("Rank must be sorted descending by score")
	}
}

func TestQualityClampsOutOfRangeInputs(t *testing.T) {
	// Relevance/Confidence above 1 are clamped, not amplified.
	s := baseSignal()
	s.Relevance, s.Confidence = 1.5, 2.0
	got := Score(Profile{}, s)
	capped := baseSignal()
	capped.Relevance, capped.Confidence = 1.0, 1.0
	if got.Score != Score(Profile{}, capped).Score {
		t.Fatal("relevance/confidence should clamp to 1.0")
	}
}

func TestMatchesAllDimensionsAndMalformedKeys(t *testing.T) {
	s := baseSignal() // country IN, region Maharashtra, sentiment NEGATIVE, entity NDMA
	cases := map[string]bool{
		"region:Maharashtra": true, "region:Goa": false,
		"sentiment:NEGATIVE": true, "sentiment:POSITIVE": false,
		"entity:ndma": true, "entity:other": false,
		"country:IN": true, "unknown:x": false,
	}
	for key, want := range cases {
		matched := len(Score(Profile{Interests: map[string]float64{key: 3}}, s).Reasons) > 0
		if matched != want {
			t.Errorf("interest %q: matched=%v want %v", key, matched, want)
		}
	}
	for _, bad := range []string{"nocolon", "trailing:", ":leading"} {
		if len(Score(Profile{Interests: map[string]float64{bad: 5}}, s).Reasons) != 0 {
			t.Errorf("malformed key %q should not match", bad)
		}
	}
}

func TestRecencyAndClampNegativeEdges(t *testing.T) {
	// Negative age → treated as fresh (0); negative relevance/confidence → clamped to 0.
	s := baseSignal()
	s.AgeHours, s.Relevance, s.Confidence = -5, -1, -1
	fresh := baseSignal()
	fresh.AgeHours, fresh.Relevance, fresh.Confidence = 0, 0, 0
	if Score(Profile{}, s).Score != Score(Profile{}, fresh).Score {
		t.Fatal("negative age/relevance/confidence should clamp")
	}
}
