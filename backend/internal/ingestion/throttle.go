package ingestion

import (
	"context"
	"errors"
	"math/rand/v2"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ErrThrottled signals that a fetch was deferred by the host gate rather than
// attempted, because the host's next free slot is further out than maxGateWait.
// Callers should treat it as "not fetched this round" — NOT a failure — and let
// the source stay due for a later tick. This bounds how long a worker blocks and
// naturally drip-feeds large same-host source sets (e.g. Google News).
var ErrThrottled = errors.New("fetch deferred by host throttle")

// maxGateWait caps how long a single fetch will block waiting for its slot.
const maxGateWait = 20 * time.Second

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
// should wait before issuing the request. If the wait would exceed cap, the slot
// is NOT claimed and ok=false (the caller should defer). It is the testable core
// of Wait. A non-positive interval means "no throttle" (immediate, ok=true).
func (g *HostGate) reserve(host string, now time.Time, maxWait time.Duration) (time.Duration, bool) {
	iv := g.interval(host)
	if iv <= 0 {
		return 0, true
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	slot := g.next[host]
	if slot.Before(now) {
		slot = now
	}
	wait := slot.Sub(now)
	if maxWait > 0 && wait > maxWait {
		return wait, false // too far out — defer without claiming the slot
	}
	jitter := time.Duration(rand.Int64N(int64(iv/2) + 1))
	g.next[host] = slot.Add(iv + jitter)
	return wait, true
}

// Wait blocks until it is this caller's turn to fetch host (up to maxGateWait),
// or returns ErrThrottled if the slot is too far out, or ctx.Err() if canceled.
func (g *HostGate) Wait(ctx context.Context, rawURL string) error {
	d, ok := g.reserve(hostOf(rawURL), time.Now(), maxGateWait)
	if !ok {
		return ErrThrottled
	}
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
