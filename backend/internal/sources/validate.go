package sources

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
)

// Result is the outcome of validating a single candidate.
type Result struct {
	Candidate    Candidate  `json:"candidate"`
	OK           bool       `json:"ok"`
	HTTPStatus   int        `json:"httpStatus"`
	ResponseMs   int        `json:"responseMs"`
	ItemCount    int        `json:"itemCount"`
	NewestItem   *time.Time `json:"newestItem,omitempty"`
	RedirectedTo string     `json:"redirectedTo,omitempty"`
	HealthScore  int        `json:"healthScore"`
	Error        string     `json:"error,omitempty"`
}

// ValidatorConfig tunes validation behavior.
type ValidatorConfig struct {
	Concurrency int           // parallel fetches
	Timeout     time.Duration // per-request timeout
	MaxAge      time.Duration // newest item must be within this window (if dated)
	UserAgent   string
}

// DefaultConfig returns sensible validation defaults.
func DefaultConfig() ValidatorConfig {
	return ValidatorConfig{
		Concurrency: 24,
		Timeout:     15 * time.Second,
		MaxAge:      120 * 24 * time.Hour, // 120 days — research/gov feeds update slowly
		UserAgent:   "WorldSignalBot/1.0 (+https://worldsignal.example/bot; source-validation)",
	}
}

// Validator fetches and validates candidates concurrently.
type Validator struct {
	cfg    ValidatorConfig
	client *http.Client
	parser *gofeed.Parser
	now    func() time.Time
}

// NewValidator builds a Validator. The http.Client records the final URL after
// redirects so permanent redirects to a different host can be flagged.
func NewValidator(cfg ValidatorConfig) *Validator {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 24
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	tr := &http.Transport{
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 8,
		IdleConnTimeout:     30 * time.Second,
	}
	return &Validator{
		cfg:    cfg,
		client: &http.Client{Timeout: cfg.Timeout, Transport: tr},
		parser: gofeed.NewParser(),
		now:    time.Now,
	}
}

// ValidateAll validates every candidate, returning results in input order.
// Candidates are de-duplicated by normalized URL first.
func (v *Validator) ValidateAll(ctx context.Context, cands []Candidate) []Result {
	cands = dedup(cands)
	results := make([]Result, len(cands))
	sem := make(chan struct{}, v.cfg.Concurrency)
	var wg sync.WaitGroup
	for i, c := range cands {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, c Candidate) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = v.validateOne(ctx, c)
		}(i, c)
	}
	wg.Wait()
	return results
}

// validateOne performs the full validation of a single candidate.
func (v *Validator) validateOne(ctx context.Context, c Candidate) Result {
	r := Result{Candidate: c}
	start := v.now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.FeedURL, nil)
	if err != nil {
		r.Error = "bad request: " + err.Error()
		return r
	}
	req.Header.Set("User-Agent", v.cfg.UserAgent)
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml;q=0.9, */*;q=0.5")

	resp, err := v.client.Do(req)
	r.ResponseMs = int(v.now().Sub(start).Milliseconds())
	if err != nil {
		r.Error = "fetch failed: " + err.Error()
		return r
	}
	defer resp.Body.Close()
	r.HTTPStatus = resp.StatusCode
	if final := resp.Request.URL.String(); normalizedURL(final) != normalizedURL(c.FeedURL) {
		r.RedirectedTo = final
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		r.Error = fmt.Sprintf("http %d", resp.StatusCode)
		return r
	}

	feed, err := v.parser.Parse(resp.Body)
	if err != nil {
		r.Error = "parse failed: " + err.Error()
		return r
	}
	r.ItemCount = len(feed.Items)
	if r.ItemCount == 0 {
		r.Error = "feed has no items"
		return r
	}

	// Newest dated item.
	for _, it := range feed.Items {
		t := it.PublishedParsed
		if t == nil {
			t = it.UpdatedParsed
		}
		if t != nil && (r.NewestItem == nil || t.After(*r.NewestItem)) {
			nt := *t
			r.NewestItem = &nt
		}
	}

	// Freshness: if the feed carries dates, the newest must be within MaxAge.
	// Undated feeds (rare but valid) pass on item presence alone.
	if r.NewestItem != nil {
		age := v.now().Sub(*r.NewestItem)
		if age > v.cfg.MaxAge {
			r.Error = fmt.Sprintf("stale: newest item is %.0f days old", age.Hours()/24)
			r.HealthScore = v.health(r)
			return r
		}
	}

	r.OK = true
	r.HealthScore = v.health(r)
	return r
}

// health computes a 0-100 quality score from freshness, volume and latency.
func (v *Validator) health(r Result) int {
	score := 100
	// Freshness.
	switch {
	case r.NewestItem == nil:
		score -= 25 // undated — usable but unverifiable freshness
	default:
		age := v.now().Sub(*r.NewestItem)
		switch {
		case age <= 24*time.Hour:
		case age <= 7*24*time.Hour:
			score -= 5
		case age <= 30*24*time.Hour:
			score -= 15
		case age <= 90*24*time.Hour:
			score -= 30
		default:
			score -= 50
		}
	}
	// Volume.
	switch {
	case r.ItemCount >= 20:
	case r.ItemCount >= 10:
		score -= 3
	case r.ItemCount >= 3:
		score -= 8
	default:
		score -= 15
	}
	// Latency.
	switch {
	case r.ResponseMs < 1000:
	case r.ResponseMs < 3000:
		score -= 3
	case r.ResponseMs < 8000:
		score -= 8
	default:
		score -= 15
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score
}
