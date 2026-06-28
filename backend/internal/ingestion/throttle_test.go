package ingestion

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestHostGateSpacesSameHost(t *testing.T) {
	g := NewHostGate(func(string) time.Duration { return 2 * time.Second })
	now := time.Unix(1000, 0)
	if d, ok := g.reserve("news.google.com", now, 0); d != 0 || !ok {
		t.Fatalf("first reserve should be 0/ok, got %v/%v", d, ok)
	}
	d2, ok2 := g.reserve("news.google.com", now, 0)
	if !ok2 || d2 < 2*time.Second {
		t.Fatalf("second reserve should wait >= 2s, got %v/%v", d2, ok2)
	}
	d3, _ := g.reserve("news.google.com", now, 0)
	if d3 <= d2 {
		t.Fatalf("third reserve %v should exceed second %v", d3, d2)
	}
}

func TestHostGateIndependentHosts(t *testing.T) {
	g := NewHostGate(func(string) time.Duration { return time.Second })
	now := time.Unix(2000, 0)
	g.reserve("a.example", now, 0)
	if d, ok := g.reserve("b.example", now, 0); d != 0 || !ok {
		t.Fatalf("different host should not be delayed, got %v/%v", d, ok)
	}
}

func TestHostGateZeroIntervalNoDelay(t *testing.T) {
	g := NewHostGate(func(string) time.Duration { return 0 })
	now := time.Unix(3000, 0)
	for i := 0; i < 5; i++ {
		if d, ok := g.reserve("feeds.bbci.co.uk", now, 0); d != 0 || !ok {
			t.Fatalf("zero-interval host should never wait, got %v/%v", d, ok)
		}
	}
}

func TestHostGateDefersBeyondCap(t *testing.T) {
	g := NewHostGate(func(string) time.Duration { return 10 * time.Second })
	now := time.Unix(5000, 0)
	// Claim a few slots so the next one is far out.
	g.reserve("news.google.com", now, time.Minute) // slot 0
	g.reserve("news.google.com", now, time.Minute) // slot ~10s
	// With a 5s cap, the next slot (~20s+) must be deferred and NOT claimed.
	d, ok := g.reserve("news.google.com", now, 5*time.Second)
	if ok {
		t.Fatalf("slot beyond cap should be deferred, got ok=true wait=%v", d)
	}
	// Deferral must not advance the gate: a later call with a big cap still sees
	// the same pending slot rather than one pushed further out.
	d2, ok2 := g.reserve("news.google.com", now, time.Minute)
	if !ok2 || d2 != d {
		t.Fatalf("deferral should not advance the slot: first=%v second=%v/%v", d, d2, ok2)
	}
}

func TestHostGateSlotAdvancesPastNow(t *testing.T) {
	g := NewHostGate(func(string) time.Duration { return time.Second })
	now := time.Unix(4000, 0)
	g.reserve("h", now, 0)
	if d, ok := g.reserve("h", now.Add(time.Hour), 0); d != 0 || !ok {
		t.Fatalf("stale slot should reset to now, got %v/%v", d, ok)
	}
}

func TestDefaultIntervalThrottlesGoogleOnly(t *testing.T) {
	if defaultInterval("news.google.com") <= 0 {
		t.Error("google news should be throttled")
	}
	if defaultInterval("feeds.bbci.co.uk") != 0 {
		t.Error("direct feeds should not be throttled")
	}
}

func TestHostGateWaitImmediate(t *testing.T) {
	g := NewHostGate(func(string) time.Duration { return 0 })
	if err := g.Wait(context.Background(), "https://feeds.bbci.co.uk/x.xml"); err != nil {
		t.Fatalf("zero-interval Wait should return nil, got %v", err)
	}
}

func TestHostGateWaitDefersWhenSaturated(t *testing.T) {
	g := NewHostGate(func(string) time.Duration { return time.Hour })
	// First Wait claims the slot (immediate). Subsequent ones would be an hour
	// out (> maxGateWait) and must return ErrThrottled rather than block.
	if err := g.Wait(context.Background(), "https://news.google.com/rss"); err != nil {
		t.Fatalf("first Wait should be immediate, got %v", err)
	}
	if err := g.Wait(context.Background(), "https://news.google.com/rss"); !errors.Is(err, ErrThrottled) {
		t.Fatalf("saturated host should defer with ErrThrottled, got %v", err)
	}
}

func TestHostGateWaitCancel(t *testing.T) {
	g := NewHostGate(func(string) time.Duration { return 5 * time.Second })
	g.reserve("news.google.com", time.Now(), 0) // push next slot ~5s out (within cap)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := g.Wait(ctx, "https://news.google.com/rss"); err == nil {
		t.Fatal("canceled context should make Wait return an error")
	}
}

func TestHostOf(t *testing.T) {
	if h := hostOf("https://News.Google.com/rss?q=x"); h != "news.google.com" {
		t.Errorf("hostOf = %q", h)
	}
	if h := hostOf("://bad url"); h == "" {
		t.Errorf("malformed url should fall back to the raw string, got empty")
	}
}
