package urlutil

import "testing"

func canon(t *testing.T, in string) string {
	t.Helper()
	out, ok := Canonicalize(in)
	if !ok {
		t.Fatalf("Canonicalize(%q) returned null unexpectedly", in)
	}
	return out
}

func TestStripsTrackingWwwLowercaseHost(t *testing.T) {
	got := canon(t, "https://WWW.Example.com/Story?utm_source=twitter&id=5&fbclid=abc")
	if got != "https://example.com/Story?id=5" {
		t.Fatalf("got %q", got)
	}
}

func TestRemovesFragmentAndTrailingSlash(t *testing.T) {
	if got := canon(t, "https://example.com/path/#section"); got != "https://example.com/path" {
		t.Fatalf("got %q", got)
	}
}

func TestKeepsRootSlash(t *testing.T) {
	if got := canon(t, "https://example.com/"); got != "https://example.com/" {
		t.Fatalf("got %q", got)
	}
}

func TestTrackingOnlyDifferenceEqual(t *testing.T) {
	a := canon(t, "https://news.com/a?utm_campaign=x")
	b := canon(t, "https://news.com/a?gclid=y")
	if a != b {
		t.Fatalf("expected equal, got %q vs %q", a, b)
	}
}

func TestNullForEmpty(t *testing.T) {
	for _, in := range []string{"", "   "} {
		if _, ok := Canonicalize(in); ok {
			t.Fatalf("Canonicalize(%q) should be null", in)
		}
	}
}

func TestRawWhenUnparseable(t *testing.T) {
	if got := canon(t, "not a real url"); got != "not a real url" {
		t.Fatalf("got %q", got)
	}
}

func TestBareDomainGetsRootSlash(t *testing.T) {
	if got := canon(t, "https://example.com"); got != "https://example.com/" {
		t.Fatalf("got %q", got)
	}
}

func TestPathOfOnlySlashesCollapses(t *testing.T) {
	if got := canon(t, "https://example.com///"); got != "https://example.com/" {
		t.Fatalf("got %q", got)
	}
}

func TestNonHttpSchemePortKept(t *testing.T) {
	// ftp default isn't 80/443, so the port is preserved.
	if got := canon(t, "ftp://example.com:21/file"); got != "ftp://example.com:21/file" {
		t.Fatalf("got %q", got)
	}
}

func TestDefaultPorts(t *testing.T) {
	cases := map[string]string{
		"https://example.com:443/x":  "https://example.com/x",
		"http://example.com:80/y":    "http://example.com/y",
		"https://example.com:8443/z": "https://example.com:8443/z",
	}
	for in, want := range cases {
		if got := canon(t, in); got != want {
			t.Fatalf("Canonicalize(%q) = %q, want %q", in, got, want)
		}
	}
}
