package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
)

func insertFetchSource(t *testing.T, d *db.DB, id, url string, crawl int, enabled bool, lastFetched *time.Time, cooldownUntil *time.Time) {
	t.Helper()
	_, err := d.Pool.Exec(context.Background(),
		`INSERT INTO "Source" ("id","name","url","enabled","crawlFrequency","priority","lastFetchedAt","cooldownUntil","updatedAt")
		 VALUES ($1,$2,$3,$4,$5,2,$6,$7,now())`,
		id, "S-"+id, url, enabled, crawl, lastFetched, cooldownUntil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMarkSourceFetchFailureCooldown(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	insertFetchSource(t, d, "f1", "https://f1.example", 300, true, nil, nil)

	now := time.Now()
	threshold := 3
	cooldown := 2 * time.Hour

	// Failures below threshold: no cooldown yet.
	for i := 0; i < threshold-1; i++ {
		if err := d.MarkSourceFetchFailure(ctx, "f1", now, threshold, cooldown, "timeout"); err != nil {
			t.Fatal(err)
		}
	}
	s, _ := d.GetSource(ctx, "f1")
	if s.FailureCount != threshold-1 || s.CooldownUntil != nil {
		t.Fatalf("before threshold: failures=%d cooldown=%v", s.FailureCount, s.CooldownUntil)
	}
	if s.LastValidationError == nil || *s.LastValidationError != "timeout" {
		t.Fatalf("expected error recorded, got %v", s.LastValidationError)
	}

	// The failure that hits the threshold sets cooldown.
	if err := d.MarkSourceFetchFailure(ctx, "f1", now, threshold, cooldown, "timeout"); err != nil {
		t.Fatal(err)
	}
	s, _ = d.GetSource(ctx, "f1")
	if s.FailureCount != threshold || s.CooldownUntil == nil {
		t.Fatalf("at threshold: failures=%d cooldown=%v", s.FailureCount, s.CooldownUntil)
	}
	if got := s.CooldownUntil.Time.Sub(now); got < cooldown-time.Minute || got > cooldown+time.Minute {
		t.Fatalf("cooldown duration off: %v", got)
	}

	// A success clears the failure count and cooldown.
	if err := d.MarkSourceFetchSuccess(ctx, "f1", now); err != nil {
		t.Fatal(err)
	}
	s, _ = d.GetSource(ctx, "f1")
	if s.FailureCount != 0 || s.CooldownUntil != nil {
		t.Fatalf("after success: failures=%d cooldown=%v", s.FailureCount, s.CooldownUntil)
	}
}

func TestListDueSources(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	now := time.Now()
	old := now.Add(-time.Hour)
	recent := now.Add(-time.Second)
	future := now.Add(time.Hour)
	past := now.Add(-time.Minute)

	insertFetchSource(t, d, "due_never", "https://a", 300, true, nil, nil)    // never fetched → due
	insertFetchSource(t, d, "due_old", "https://b", 300, true, &old, nil)     // fetched 1h ago, crawl 300s → due
	insertFetchSource(t, d, "notdue", "https://c", 3600, true, &recent, nil)  // fetched 1s ago, crawl 1h → not due
	insertFetchSource(t, d, "disabled", "https://d", 1, false, nil, nil)      // disabled → excluded
	insertFetchSource(t, d, "cooling", "https://e", 1, true, &old, &future)   // in cooldown → excluded
	insertFetchSource(t, d, "cooled_done", "https://f", 1, true, &old, &past) // cooldown elapsed → due

	ids, err := d.ListDueSources(ctx, now, 100)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, id := range ids {
		got[id] = true
	}
	for _, want := range []string{"due_never", "due_old", "cooled_done"} {
		if !got[want] {
			t.Fatalf("expected %s to be due; got %v", want, ids)
		}
	}
	for _, bad := range []string{"notdue", "disabled", "cooling"} {
		if got[bad] {
			t.Fatalf("did not expect %s to be due; got %v", bad, ids)
		}
	}

	// Limit is respected.
	limited, err := d.ListDueSources(ctx, now, 1)
	if err != nil || len(limited) != 1 {
		t.Fatalf("limit: len=%d err=%v", len(limited), err)
	}
}
