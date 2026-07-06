package taxonomy

import (
	"strings"
	"testing"

	"github.com/worldsignal/backend/internal/jsonx"
)

func TestFlattenCountsDomainsAndLeaves(t *testing.T) {
	all := Flatten(Taxonomy)
	if len(all) <= len(Taxonomy) {
		t.Fatalf("flatten should expand children, got %d", len(all))
	}
	domains := 0
	leaves := 0
	for _, n := range all {
		if n.Children != nil {
			domains++
		} else {
			leaves++
		}
	}
	// Domain count equals the number of top-level nodes (every top-level node is
	// a domain with children).
	if domains != len(Taxonomy) {
		t.Fatalf("want %d domains, got %d", len(Taxonomy), domains)
	}
	if leaves != len(LeafTags()) {
		t.Fatalf("leaf mismatch: flatten %d vs LeafTags %d", leaves, len(LeafTags()))
	}
}

// Every topical (non-GENERAL) domain must expose a `<DOMAIN>.OTHER` leaf so a
// story recognized at domain level never falls through to GENERAL.OTHER.
func TestEveryDomainHasOtherLeaf(t *testing.T) {
	for _, d := range Taxonomy {
		if d.Code == "GENERAL" {
			continue
		}
		want := d.Code + ".OTHER"
		if _, ok := ValidCodes[want]; !ok {
			t.Errorf("domain %s missing fallback leaf %s", d.Code, want)
		}
	}
}

func TestValidCodesContainsDomainAndLeaf(t *testing.T) {
	for _, code := range []string{"POLITICS", "POLITICS.ELECTIONS", FallbackCode} {
		if _, ok := ValidCodes[code]; !ok {
			t.Fatalf("ValidCodes missing %q", code)
		}
	}
	if _, ok := ValidCodes["NOPE.NOPE"]; ok {
		t.Fatal("ValidCodes should not contain bogus code")
	}
}

func TestLeafTagsHaveKeywordsSlice(t *testing.T) {
	for _, l := range LeafTags() {
		if l.Keywords == nil {
			t.Fatalf("leaf %s has nil keywords (would drop the JSON key)", l.Code)
		}
	}
}

// Golden byte-parity: the serialized taxonomy must match the TS JSON.stringify
// output exactly. Captured from `tsx -e "...JSON.stringify(TAXONOMY)"`.
func TestTaxonomyJSONByteParity(t *testing.T) {
	b, err := jsonx.Marshal(Taxonomy)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	// Spot-check the two structural shapes and the empty-keywords leaf.
	if !strings.Contains(got, `{"code":"POLITICS","label":"Politics","children":[`) {
		t.Fatal("domain shape wrong")
	}
	if !strings.Contains(got, `{"code":"POLITICS.ELECTIONS","label":"Elections","keywords":[`) {
		t.Fatal("leaf shape wrong")
	}
	if !strings.HasSuffix(got, `{"code":"GENERAL.OTHER","label":"Other / Uncategorized","keywords":[]}]}]`) {
		t.Fatalf("tail/empty-keywords shape wrong: ...%s", got[len(got)-60:])
	}
	// & must be raw, not HTML-escaped to & (matches JSON.stringify).
	if strings.Contains(got, "\\u0026") {
		t.Fatal("& was HTML-escaped to \\u0026; breaks byte-parity")
	}
	if !strings.Contains(got, `"Jobs & Employment"`) {
		t.Fatal("expected raw ampersand in label")
	}
}
