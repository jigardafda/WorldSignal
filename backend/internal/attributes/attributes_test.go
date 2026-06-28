package attributes

import (
	"testing"

	"github.com/worldsignal/backend/internal/taxonomy"
)

func TestRegistryShape(t *testing.T) {
	defs := Definitions()
	if len(defs) < 10 {
		t.Fatalf("expected a rich dictionary, got %d", len(defs))
	}
	seen := map[string]bool{}
	for _, d := range defs {
		if d.Key == "" || d.Label == "" || d.Kind == "" || d.Description == "" {
			t.Fatalf("incomplete definition: %+v", d)
		}
		if seen[d.Key] {
			t.Fatalf("duplicate key %q", d.Key)
		}
		seen[d.Key] = true
		switch d.Kind {
		case KindEnum, KindTagSet:
			if len(d.Values()) == 0 {
				t.Fatalf("%s is enum/tagset but has no vocabulary", d.Key)
			}
			if len(d.Codes()) != len(d.Values()) {
				t.Fatalf("%s codes/values length mismatch", d.Key)
			}
		case KindScalar:
			if d.Max <= d.Min {
				t.Fatalf("%s has invalid scalar range [%v,%v]", d.Key, d.Min, d.Max)
			}
		}
	}
	for _, key := range []string{"country", "region", "city", "locality", "geoScope",
		"sentiment", "sentimentScore", "influence", "severity", "confidence", "relevance",
		"industry", "category", "entity", "entityType"} {
		if !seen[key] {
			t.Errorf("missing required attribute %q", key)
		}
	}
}

func TestLookup(t *testing.T) {
	d, ok := Lookup("sentiment")
	if !ok || d.Kind != KindEnum {
		t.Fatalf("sentiment lookup wrong: %+v ok=%v", d, ok)
	}
	if _, ok := Lookup("nope"); ok {
		t.Fatal("unknown key should not resolve")
	}
}

func TestNormalizeEnum(t *testing.T) {
	d, _ := Lookup("geoScope")
	cases := map[string]string{
		"GLOBAL":      "GLOBAL", // code
		"Global":      "GLOBAL", // label
		"worldwide":   "GLOBAL", // alias
		"  national ": "NATIONAL",
		"COUNTRY":     "NATIONAL", // alias, case-insensitive
	}
	for in, want := range cases {
		got, ok := d.Normalize(in)
		if !ok || got != want {
			t.Errorf("Normalize(%q) = %q,%v want %q", in, got, ok, want)
		}
	}
	if _, ok := d.Normalize("interstellar"); ok {
		t.Error("out-of-vocabulary value should be rejected")
	}
}

func TestNormalizeTagSet(t *testing.T) {
	d, _ := Lookup("industry")
	if got, ok := d.Normalize("AI"); !ok || got != "ARTIFICIAL_INTELLIGENCE" {
		t.Errorf("AI alias = %q,%v", got, ok)
	}
	if got, ok := d.Normalize("electric vehicles"); !ok || got != "AUTOMOTIVE" {
		t.Errorf("EV alias = %q,%v", got, ok)
	}
	if _, ok := d.Normalize("time travel"); ok {
		t.Error("unknown industry should be rejected")
	}
}

func TestNormalizeNonEnumReturnsFalse(t *testing.T) {
	d, _ := Lookup("city")
	if _, ok := d.Normalize("Mumbai"); ok {
		t.Error("text attribute Normalize should report ok=false (no vocab)")
	}
}

func TestCategoryMirrorsTaxonomy(t *testing.T) {
	d, _ := Lookup("category")
	if got, ok := d.Normalize("DISASTER.EARTHQUAKE"); !ok || got != "DISASTER.EARTHQUAKE" {
		t.Errorf("taxonomy code = %q,%v", got, ok)
	}
	// Every taxonomy code must be representable.
	for code := range taxonomy.ValidCodes {
		if _, ok := d.Normalize(code); !ok {
			t.Errorf("taxonomy code %q not in category vocabulary", code)
		}
	}
}

func TestClampScalar(t *testing.T) {
	conf, _ := Lookup("confidence")
	if got := conf.ClampScalar(1.5); got != 1 {
		t.Errorf("clamp high = %v", got)
	}
	if got := conf.ClampScalar(-0.2); got != 0 {
		t.Errorf("clamp low = %v", got)
	}
	if got := conf.ClampScalar(0.7); got != 0.7 {
		t.Errorf("clamp mid = %v", got)
	}
	sent, _ := Lookup("sentimentScore")
	if got := sent.ClampScalar(-3); got != -1 {
		t.Errorf("signed clamp low = %v", got)
	}
	if got := sent.ClampScalar(3); got != 1 {
		t.Errorf("signed clamp high = %v", got)
	}
}

func TestNormalizeCountry(t *testing.T) {
	if got, ok := NormalizeCountry("us"); !ok || got != "US" {
		t.Errorf("code = %q,%v", got, ok)
	}
	if got, ok := NormalizeCountry("United States"); !ok || got != "US" {
		t.Errorf("name = %q,%v", got, ok)
	}
	if got, ok := NormalizeCountry("  india "); !ok || got != "IN" {
		t.Errorf("trimmed code = %q,%v", got, ok)
	}
	if _, ok := NormalizeCountry("Atlantis"); ok {
		t.Error("unknown country should be rejected")
	}
	if _, ok := NormalizeCountry(""); ok {
		t.Error("empty country should be rejected")
	}
}
