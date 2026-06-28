package countries

import "testing"

func TestAllAndGet(t *testing.T) {
	all := All()
	if len(all) < 180 {
		t.Fatalf("expected a comprehensive list, got %d", len(all))
	}
	// Sorted by name.
	for i := 1; i < len(all); i++ {
		if all[i-1].Name > all[i].Name {
			t.Fatalf("not sorted: %s before %s", all[i-1].Name, all[i].Name)
		}
	}
	// No duplicate codes (data.json has a duplicate TW that must be deduped).
	seen := map[string]bool{}
	for _, c := range all {
		if seen[c.Code] {
			t.Fatalf("duplicate code in list: %s", c.Code)
		}
		seen[c.Code] = true
	}

	us, ok := Get("us") // case-insensitive
	if !ok || us.Name != "United States" || us.Capital != "Washington, D.C." || us.Currency != "USD" {
		t.Fatalf("US lookup wrong: %+v ok=%v", us, ok)
	}
	if us.CapitalLat == 0 || us.CapitalLng == 0 {
		t.Fatal("US should have capital coordinates")
	}
	if us.Flag != "🇺🇸" {
		t.Fatalf("US flag wrong: %q", us.Flag)
	}
	if _, ok := Get("ZZ"); ok {
		t.Fatal("unknown code should not resolve")
	}
}

func TestFlagOf(t *testing.T) {
	if flagOf("GB") != "🇬🇧" {
		t.Fatalf("GB flag: %q", flagOf("GB"))
	}
	if flagOf("x") != "" || flagOf("12") != "" {
		t.Fatal("invalid codes should yield empty flag")
	}
}
