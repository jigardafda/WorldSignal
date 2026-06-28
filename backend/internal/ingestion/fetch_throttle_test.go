package ingestion

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestFetchRSSSourceDefersWhenThrottled verifies the fetch path short-circuits
// with ErrThrottled (no HTTP request attempted) when the host gate is saturated.
// It whitebox-swaps the process gate and pre-claims the host's slot so the call
// is deferred without any network access.
func TestFetchRSSSourceDefersWhenThrottled(t *testing.T) {
	orig := fetchGate
	t.Cleanup(func() { fetchGate = orig })

	fetchGate = NewHostGate(func(string) time.Duration { return time.Hour })
	url := "https://news.google.com/rss/search?q=x"
	// Claim the host's first slot so the next request is an hour out (> cap).
	fetchGate.reserve("news.google.com", time.Now(), 0)

	if _, err := FetchRSSSource(context.Background(), url); !errors.Is(err, ErrThrottled) {
		t.Fatalf("saturated gate should defer with ErrThrottled, got %v", err)
	}
}
