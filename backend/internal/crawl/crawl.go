// Package crawl performs best-effort retrieval of a source article's web page to
// gather richer context than the RSS/feed body. It extracts the main readable
// text (dropping nav/header/footer/script/style boilerplate) for downstream LLM
// enrichment. Every failure is non-fatal: callers fall back to the feed content.
package crawl

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/worldsignal/backend/internal/textutil"
)

// Result is the outcome of crawling one page. On any failure Text is empty and
// Err describes the reason; callers should treat the page as best-effort context.
type Result struct {
	URL        string `json:"url"`
	HTTPStatus int    `json:"httpStatus"`
	Title      string `json:"title,omitempty"`
	Text       string `json:"text,omitempty"`
	Chars      int    `json:"chars"`
	Truncated  bool   `json:"truncated"`
	Err        string `json:"error,omitempty"`
}

// OK reports whether the crawl produced usable text.
func (r Result) OK() bool { return r.Err == "" && r.Text != "" }

// Crawler fetches and cleans article pages.
type Crawler struct {
	Client   *http.Client
	UA       string
	MaxBytes int64 // cap on bytes read from the response body
	MaxChars int   // cap on extracted text length (runes)
}

const defaultUA = "WorldSignalBot/1.0 (+https://worldsignal.example/bot; enrichment)"

// New builds a Crawler with sensible defaults.
func New() *Crawler {
	return &Crawler{
		Client:   &http.Client{Timeout: 12 * time.Second},
		UA:       defaultUA,
		MaxBytes: 2 << 20, // 2 MiB
		MaxChars: 12000,
	}
}

var (
	reTitle    = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	reArticle  = regexp.MustCompile(`(?is)<article[^>]*>(.*?)</article>`)
	reMain     = regexp.MustCompile(`(?is)<main[^>]*>(.*?)</main>`)
	reBoiler   = regexp.MustCompile(`(?is)<(nav|header|footer|aside|form|figure|noscript)[^>]*>.*?</(nav|header|footer|aside|form|figure|noscript)>`)
	reComments = regexp.MustCompile(`(?s)<!--.*?-->`)
)

// Fetch retrieves url and returns its cleaned main text (best-effort).
func (c *Crawler) Fetch(ctx context.Context, rawURL string) Result {
	r := Result{URL: rawURL}
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		r.Err = "invalid url"
		return r
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		r.Err = "bad request: " + err.Error()
		return r
	}
	req.Header.Set("User-Agent", c.UA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.5")

	resp, err := c.Client.Do(req)
	if err != nil {
		r.Err = "fetch failed: " + err.Error()
		return r
	}
	defer func() { _ = resp.Body.Close() }()
	r.HTTPStatus = resp.StatusCode
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		r.Err = "http " + itoa(resp.StatusCode)
		return r
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" && !isHTML(ct) {
		r.Err = "non-html content-type: " + ct
		return r
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, c.MaxBytes))
	if err != nil {
		r.Err = "read failed: " + err.Error()
		return r
	}
	html := string(body)

	if m := reTitle.FindStringSubmatch(html); m != nil {
		r.Title = textutil.StripHtml(m[1])
	}

	text := extractMain(html)
	runes := []rune(text)
	if c.MaxChars > 0 && len(runes) > c.MaxChars {
		text = string(runes[:c.MaxChars])
		r.Truncated = true
	}
	r.Text = text
	r.Chars = len([]rune(text))
	if r.Text == "" {
		r.Err = "no extractable text"
	}
	return r
}

// extractMain returns the cleaned readable text, preferring <article>/<main>
// containers and dropping common boilerplate blocks before stripping tags.
func extractMain(html string) string {
	html = reComments.ReplaceAllString(html, " ")
	region := html
	if m := reArticle.FindStringSubmatch(html); m != nil && len(strings.TrimSpace(m[1])) > 0 {
		region = m[1]
	} else if m := reMain.FindStringSubmatch(html); m != nil && len(strings.TrimSpace(m[1])) > 0 {
		region = m[1]
	}
	region = reBoiler.ReplaceAllString(region, " ")
	return textutil.StripHtml(region)
}

func isHTML(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
