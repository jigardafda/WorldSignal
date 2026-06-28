package ingestion

import (
	"context"
	"math/rand/v2"
	"net/url"
	"strings"
	"sync"
	"time"
)

// HostGate paces outbound feed requests per host so aggressive aggregators
// (notably Google News, which returns HTTP 503 "Sorry…" under bulk access) are
// fetched politely instead of being rate-limited into cooldown. Requests to the
// same host are spaced by an interval (+ jitter); distinct hosts proceed
// independently. Hosts with a zero interval are not delayed at all.
type HostGate struct {
	mu       sync.Mutex
	next     map[string]time.Time
	interval func(host string) time.Duration
}

// NewHostGate builds a gate using the given per-host interval policy.
func NewHostGate(interval func(host string) time.Duration) *HostGate {
	return &HostGate{next: map[string]time.Time{}, interval: interval}
}

// reserve claims the next time slot for host and returns how long the caller
// should wait before issuing the request. It is the testable core of Wait.
func (g *HostGate) reserve(host string, now time.Time) time.Duration {
	iv := g.interval(host)
	if iv <= 0 {
		return 0
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	slot := g.next[host]
	if slot.Before(now) {
		slot = now
	}
	jitter := time.Duration(rand.Int64N(int64(iv/2) + 1))
	g.next[host] = slot.Add(iv + jitter)
	return slot.Sub(now)
}

// Wait blocks until it is this caller's turn to fetch host, or ctx is done.
func (g *HostGate) Wait(ctx context.Context, rawURL string) error {
	d := g.reserve(hostOf(rawURL), time.Now())
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return strings.ToLower(u.Host)
}

// throttledHosts maps a host substring to the minimum spacing between requests.
// Google News blocks bulk RSS access hard, so it gets generous spacing.
func defaultInterval(host string) time.Duration {
	if strings.Contains(host, "news.google.com") || strings.Contains(host, "google.com") {
		return 4 * time.Second
	}
	return 0 // direct feeds (distinct hosts, few sources each) need no pacing
}

// fetchGate is the process-wide gate applied by FetchRSSSource.
var fetchGate = NewHostGate(defaultInterval)
