package ingestion

import (
	"context"
	"testing"
	"time"
)

func TestHostGateSpacesSameHost(t *testing.T) {
	g := NewHostGate(func(string) time.Duration { return 2 * time.Second })
	now := time.Unix(1000, 0)
	// First request to a host is immediate.
	if d := g.reserve("news.google.com", now); d != 0 {
		t.Fatalf("first reserve should be 0, got %v", d)
	}
	// Second request to the same host must wait at least the base interval.
	d2 := g.reserve("news.google.com", now)
	if d2 < 2*time.Second {
		t.Fatalf("second reserve should wait >= 2s, got %v", d2)
	}
	// Third waits even longer (slots accumulate).
	d3 := g.reserve("news.google.com", now)
	if d3 <= d2 {
		t.Fatalf("third reserve %v should exceed second %v", d3, d2)
	}
}

func TestHostGateIndependentHosts(t *testing.T) {
	g := NewHostGate(func(string) time.Duration { return time.Second })
	now := time.Unix(2000, 0)
	g.reserve("a.example", now)
	if d := g.reserve("b.example", now); d != 0 {
		t.Fatalf("different host should not be delayed, got %v", d)
	}
}

func TestHostGateZeroIntervalNoDelay(t *testing.T) {
	g := NewHostGate(func(string) time.Duration { return 0 })
	now := time.Unix(3000, 0)
	for i := 0; i < 5; i++ {
		if d := g.reserve("feeds.bbci.co.uk", now); d != 0 {
			t.Fatalf("zero-interval host should never wait, got %v", d)
		}
	}
}

func TestHostGateSlotAdvancesPastNow(t *testing.T) {
	g := NewHostGate(func(string) time.Duration { return time.Second })
	now := time.Unix(4000, 0)
	g.reserve("h", now)
	// A request far in the future starts fresh (slot in the past resets to now).
	if d := g.reserve("h", now.Add(time.Hour)); d != 0 {
		t.Fatalf("stale slot should reset to now, got %v", d)
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

func TestHostGateWaitImmediateAndCancel(t *testing.T) {
	g := NewHostGate(func(string) time.Duration { return 0 })
	if err := g.Wait(context.Background(), "https://feeds.bbci.co.uk/x.xml"); err != nil {
		t.Fatalf("zero-interval Wait should return nil, got %v", err)
	}
	// A throttled host with a canceled context returns the context error.
	gg := NewHostGate(func(string) time.Duration { return time.Minute })
	gg.reserve("news.google.com", time.Now()) // make the next slot far out
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := gg.Wait(ctx, "https://news.google.com/rss"); err == nil {
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
