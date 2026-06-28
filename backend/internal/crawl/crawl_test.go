package crawl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testCrawler(srv *httptest.Server) *Crawler {
	c := New()
	c.Client = srv.Client()
	return c
}

const samplePage = `<!doctype html><html><head><title>  Quake hits coast </title></head>
<body>
<nav>Home About Contact</nav>
<header>Site banner</header>
<article><h1>Magnitude 6 earthquake</h1><p>A strong earthquake struck the coast.</p>
<script>var x=1;</script><style>.a{}</style></article>
<footer>Copyright</footer>
</body></html>`

func TestFetchExtractsArticle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(samplePage))
	}))
	defer srv.Close()

	r := testCrawler(srv).Fetch(context.Background(), srv.URL)
	if !r.OK() {
		t.Fatalf("expected OK, got err=%q", r.Err)
	}
	if r.Title != "Quake hits coast" {
		t.Errorf("title = %q", r.Title)
	}
	if !strings.Contains(r.Text, "Magnitude 6 earthquake") || !strings.Contains(r.Text, "struck the coast") {
		t.Errorf("article text missing: %q", r.Text)
	}
	for _, junk := range []string{"Home About", "Site banner", "Copyright", "var x", ".a{"} {
		if strings.Contains(r.Text, junk) {
			t.Errorf("boilerplate %q leaked into text: %q", junk, r.Text)
		}
	}
}

func TestFetchFallsBackToBodyWhenNoArticle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><p>Plain body text here.</p></body></html>`))
	}))
	defer srv.Close()
	r := testCrawler(srv).Fetch(context.Background(), srv.URL)
	if !r.OK() || !strings.Contains(r.Text, "Plain body text here") {
		t.Fatalf("body fallback failed: %+v", r)
	}
}

func TestFetchPrefersMainWhenNoArticle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><nav>menu</nav><main><p>Main content body.</p></main></body></html>`))
	}))
	defer srv.Close()
	r := testCrawler(srv).Fetch(context.Background(), srv.URL)
	if !strings.Contains(r.Text, "Main content body") || strings.Contains(r.Text, "menu") {
		t.Fatalf("main extraction failed: %+v", r)
	}
}

func TestFetchTruncates(t *testing.T) {
	long := strings.Repeat("word ", 1000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body><article>" + long + "</article></body></html>"))
	}))
	defer srv.Close()
	c := testCrawler(srv)
	c.MaxChars = 50
	r := c.Fetch(context.Background(), srv.URL)
	if !r.Truncated || r.Chars != 50 {
		t.Fatalf("expected truncation to 50, got truncated=%v chars=%d", r.Truncated, r.Chars)
	}
}

func TestFetchRejectsNonHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.4"))
	}))
	defer srv.Close()
	r := testCrawler(srv).Fetch(context.Background(), srv.URL)
	if r.OK() || !strings.Contains(r.Err, "non-html") {
		t.Fatalf("expected non-html rejection, got %+v", r)
	}
}

func TestFetchHandlesHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	r := testCrawler(srv).Fetch(context.Background(), srv.URL)
	if r.OK() || r.Err != "http 404" {
		t.Fatalf("expected http 404, got %+v", r)
	}
}

func TestFetchRejectsBadURL(t *testing.T) {
	c := New()
	for _, bad := range []string{"", "ftp://x", "not a url", "file:///etc/passwd", "://nohost"} {
		if r := c.Fetch(context.Background(), bad); r.OK() {
			t.Errorf("expected rejection for %q", bad)
		}
	}
}

func TestFetchNetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // now connections fail
	r := New().Fetch(context.Background(), url)
	if r.OK() || !strings.Contains(r.Err, "fetch failed") {
		t.Fatalf("expected fetch failure, got %+v", r)
	}
}

func TestFetchEmptyTextNoArticle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><nav>only nav</nav></body></html>`))
	}))
	defer srv.Close()
	r := testCrawler(srv).Fetch(context.Background(), srv.URL)
	if r.OK() || r.Err != "no extractable text" {
		t.Fatalf("expected no extractable text, got %+v", r)
	}
}

func TestFetchEmptyArticleFallsToMain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><article>   </article><main><p>Real main text.</p></main></body></html>`))
	}))
	defer srv.Close()
	r := testCrawler(srv).Fetch(context.Background(), srv.URL)
	if !strings.Contains(r.Text, "Real main text") {
		t.Fatalf("expected fallback to main, got %+v", r)
	}
}

func TestItoa(t *testing.T) {
	cases := map[int]string{0: "0", 7: "7", 404: "404", -5: "-5"}
	for in, want := range cases {
		if got := itoa(in); got != want {
			t.Errorf("itoa(%d)=%q want %q", in, got, want)
		}
	}
}
