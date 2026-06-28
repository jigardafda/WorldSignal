package sources

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// rssFeed renders a minimal RSS document with n items dated `age` before `ref`.
func rssFeed(n int, ref time.Time, age time.Duration) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?><rss version="2.0"><channel><title>Test</title>`)
	for i := 0; i < n; i++ {
		pub := ref.Add(-age).Format(time.RFC1123Z)
		fmt.Fprintf(&b, `<item><title>Item %d</title><link>http://x/%d</link><pubDate>%s</pubDate></item>`, i, i, pub)
	}
	b.WriteString(`</channel></rss>`)
	return b.String()
}

func TestNormalizeAndDedup(t *testing.T) {
	if normalizedURL(" HTTP://X.com/Feed/ ") != "http://x.com/feed" {
		t.Fatalf("normalize: %q", normalizedURL(" HTTP://X.com/Feed/ "))
	}
	if host("https://www.bbc.com/x") != "bbc.com" {
		t.Fatalf("host: %q", host("https://www.bbc.com/x"))
	}
	if host("::::") != "" {
		t.Fatalf("host bad url should be empty")
	}
	in := []Candidate{{FeedURL: "http://a/x"}, {FeedURL: "http://a/x/"}, {FeedURL: ""}, {FeedURL: "http://b"}}
	out := dedup(in)
	if len(out) != 2 {
		t.Fatalf("dedup expected 2, got %d", len(out))
	}
}

func TestCatalogGenerators(t *testing.T) {
	gn := GNewsCandidates()
	if len(gn) < 500 {
		t.Fatalf("gnews candidates too few: %d", len(gn))
	}
	for _, c := range gn[:5] {
		if !strings.HasPrefix(c.FeedURL, "https://news.google.com/rss") || !strings.Contains(c.FeedURL, "ceid=") {
			t.Fatalf("bad gnews url: %s", c.FeedURL)
		}
	}
	if len(IndustryCandidates()) < 50 {
		t.Fatal("industry candidates too few")
	}
	cur := CuratedCandidates()
	if len(cur) < 50 {
		t.Fatal("curated too few")
	}
	// ATOM inference from URL.
	var sawAtom bool
	for _, c := range cur {
		if strings.Contains(c.FeedURL, ".atom") && c.SourceType == "ATOM" {
			sawAtom = true
		}
		if !c.OfficialFeed {
			t.Fatalf("curated should be official: %s", c.Name)
		}
	}
	if !sawAtom {
		t.Fatal("expected at least one ATOM curated feed")
	}
	all := All()
	if len(all) < 1000 {
		t.Fatalf("All() expected 1000+, got %d", len(all))
	}
}

func TestCurToCandidateDefaults(t *testing.T) {
	// Empty org/scope/cc → defaults: GLOBAL scope, PRIVATE org, RSS type.
	g := cur{name: "G", feed: "https://g.example/feed", langs: "en", tags: "a b"}.toCandidate()
	if g.GeographicScope != "GLOBAL" || g.OrgType != "PRIVATE" || g.SourceType != "RSS" || !g.OfficialFeed {
		t.Fatalf("global defaults wrong: %+v", g)
	}
	if len(g.Languages) != 1 || len(g.Tags) != 2 {
		t.Fatalf("field split wrong: %+v", g)
	}
	// With a country but no scope → NATIONAL; atom URL → ATOM.
	n := cur{name: "N", feed: "https://n.example/feed.atom", cc: "US", org: "PUBLIC"}.toCandidate()
	if n.GeographicScope != "NATIONAL" || n.SourceType != "ATOM" || n.OrgType != "PUBLIC" {
		t.Fatalf("national defaults wrong: %+v", n)
	}
}

func TestGNewsURLQueryPath(t *testing.T) {
	e := edition{country: "US", cc: "US", lang: "en-US", langs: []string{"en"}, region: "North America"}
	// A path already containing "?" must use "&" to append params.
	u := gnewsURL("/search?q=ai", e)
	if !strings.Contains(u, "/search?q=ai&hl=en-US") || !strings.Contains(u, "ceid=US:en") {
		t.Fatalf("query-path url wrong: %s", u)
	}
	if langPrefix("en") != "en" {
		t.Fatal("langPrefix bare")
	}
}

func TestSummarize(t *testing.T) {
	s := Summarize([]Candidate{
		{GeographicScope: "GLOBAL", Region: "Global", Languages: []string{"en"}, Industry: "Tech", Metadata: map[string]any{"discoverySource": "curated"}},
		{GeographicScope: "NATIONAL", Region: "Europe", Country: "GB", Languages: []string{"en"}, Metadata: map[string]any{"discoverySource": "curated"}},
	})
	if s.Total != 2 || s.ByScope["GLOBAL"] != 1 || s.ByCountry["GB"] != 1 || s.ByLanguage["en"] != 2 || s.BySource["curated"] != 2 {
		t.Fatalf("summary wrong: %+v", s)
	}
}

func TestValidatorOK(t *testing.T) {
	ref := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		fmt.Fprint(w, rssFeed(25, ref, time.Hour)) // fresh, plenty of items
	}))
	defer srv.Close()

	v := NewValidator(DefaultConfig())
	v.now = func() time.Time { return ref }
	r := v.ValidateCandidate(context.Background(), Candidate{FeedURL: srv.URL})
	if !r.OK || r.ItemCount != 25 || r.HealthScore < 90 {
		t.Fatalf("expected healthy OK, got %+v", r)
	}
	if r.NewestItem == nil {
		t.Fatal("expected newest item")
	}
}

func TestValidatorFailures(t *testing.T) {
	ref := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	mux := http.NewServeMux()
	mux.HandleFunc("/404", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	mux.HandleFunc("/html", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "<html><body>not a feed</body></html>") })
	mux.HandleFunc("/empty", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<?xml version="1.0"?><rss version="2.0"><channel><title>e</title></channel></rss>`)
	})
	mux.HandleFunc("/stale", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, rssFeed(5, ref, 200*24*time.Hour)) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	v := NewValidator(DefaultConfig())
	v.now = func() time.Time { return ref }

	cases := map[string]string{"/404": "http 404", "/html": "parse", "/empty": "no items", "/stale": "stale"}
	for path, wantErr := range cases {
		r := v.ValidateCandidate(context.Background(), Candidate{FeedURL: srv.URL + path})
		if r.OK {
			t.Fatalf("%s: expected failure", path)
		}
		if !strings.Contains(r.Error, wantErr) {
			t.Fatalf("%s: want error %q, got %q", path, wantErr, r.Error)
		}
	}

	// Unreachable host → fetch failed.
	r := v.ValidateCandidate(context.Background(), Candidate{FeedURL: "http://127.0.0.1:0/x"})
	if r.OK || !strings.Contains(r.Error, "fetch failed") {
		t.Fatalf("expected fetch failure, got %+v", r)
	}
	// Malformed request URL.
	if r := v.ValidateCandidate(context.Background(), Candidate{FeedURL: "://bad"}); r.OK {
		t.Fatal("expected bad-url failure")
	}
}

func TestValidatorHealthTiers(t *testing.T) {
	ref := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	mk := func(n int, age time.Duration) string { return rssFeed(n, ref, age) }
	mux := http.NewServeMux()
	mux.HandleFunc("/week", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, mk(15, 3*24*time.Hour)) })
	mux.HandleFunc("/month", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, mk(5, 20*24*time.Hour)) })
	mux.HandleFunc("/quarter", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, mk(2, 60*24*time.Hour)) })
	srv := httptest.NewServer(mux)
	defer srv.Close()
	v := NewValidator(DefaultConfig())
	v.now = func() time.Time { return ref }
	for _, p := range []string{"/week", "/month", "/quarter"} {
		r := v.ValidateCandidate(context.Background(), Candidate{FeedURL: srv.URL + p})
		if !r.OK || r.HealthScore <= 0 || r.HealthScore > 100 {
			t.Fatalf("%s: bad health %+v", p, r)
		}
	}
}

func TestNewValidatorDefaults(t *testing.T) {
	v := NewValidator(ValidatorConfig{}) // zero config → defaults applied
	if v.cfg.Concurrency != 24 || v.cfg.Timeout != 15*time.Second {
		t.Fatalf("defaults not applied: %+v", v.cfg)
	}
}

func TestValidatorUndatedAndSlow(t *testing.T) {
	ref := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	mux := http.NewServeMux()
	// Undated feed: items but no pubDate → passes on presence, health docks freshness.
	mux.HandleFunc("/undated", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<?xml version="1.0"?><rss version="2.0"><channel><title>u</title><item><title>x</title><link>http://x/1</link></item></channel></rss>`)
	})
	// Old (100 days) but within the 120-day MaxAge → still OK, lowest freshness tier.
	mux.HandleFunc("/old", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, rssFeed(8, ref, 100*24*time.Hour)) })
	// Slow response → exercises a latency tier.
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1100 * time.Millisecond)
		fmt.Fprint(w, rssFeed(10, ref, time.Hour))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	v := NewValidator(DefaultConfig())
	v.now = func() time.Time { return ref }

	if r := v.ValidateCandidate(context.Background(), Candidate{FeedURL: srv.URL + "/undated"}); !r.OK || r.NewestItem != nil {
		t.Fatalf("undated should pass without a date: %+v", r)
	}
	if r := v.ValidateCandidate(context.Background(), Candidate{FeedURL: srv.URL + "/old"}); !r.OK || r.HealthScore >= 80 {
		t.Fatalf("old feed should pass with low health: %+v", r)
	}
	// /slow uses the real clock for ResponseMs, so don't pin v.now here.
	v2 := NewValidator(DefaultConfig())
	if r := v2.ValidateCandidate(context.Background(), Candidate{FeedURL: srv.URL + "/slow"}); !r.OK || r.ResponseMs < 1000 {
		t.Fatalf("slow feed should record high latency: %+v", r)
	}
}

func TestValidateAllConcurrent(t *testing.T) {
	ref := time.Now()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, rssFeed(10, ref, time.Hour))
	}))
	defer srv.Close()
	cands := make([]Candidate, 10)
	for i := range cands {
		cands[i] = Candidate{Name: fmt.Sprintf("c%d", i), FeedURL: fmt.Sprintf("%s/?i=%d", srv.URL, i)}
	}
	cfg := DefaultConfig()
	cfg.Concurrency = 4
	v := NewValidator(cfg)
	results := v.ValidateAll(context.Background(), cands)
	if len(results) != 10 {
		t.Fatalf("expected 10 results, got %d", len(results))
	}
	for _, r := range results {
		if !r.OK {
			t.Fatalf("expected OK: %+v", r)
		}
	}
}
