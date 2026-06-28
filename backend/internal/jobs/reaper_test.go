package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
)

func TestRequeueStuck(t *testing.T) {
	d := dbtest.Connect(t)
	q := New(d.Pool)
	ctx := context.Background()
	if err := q.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(ctx, `DELETE FROM ws_jobs`); err != nil {
		t.Fatal(err)
	}
	// One job stuck 'active' for 10 min, one freshly active.
	if _, err := d.Pool.Exec(ctx,
		`INSERT INTO ws_jobs (id,queue,data,state,started_at) VALUES
		 ('stuck','source.fetch','{}','active', now() - interval '10 minutes'),
		 ('fresh','source.fetch','{}','active', now())`); err != nil {
		t.Fatal(err)
	}

	n, err := q.requeueStuck(ctx, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 orphaned job requeued, got %d", n)
	}
	var stuckState, freshState string
	d.Pool.QueryRow(ctx, `SELECT state FROM ws_jobs WHERE id='stuck'`).Scan(&stuckState)
	d.Pool.QueryRow(ctx, `SELECT state FROM ws_jobs WHERE id='fresh'`).Scan(&freshState)
	if stuckState != "created" {
		t.Fatalf("stuck job should be requeued to created, got %s", stuckState)
	}
	if freshState != "active" {
		t.Fatalf("fresh job should stay active, got %s", freshState)
	}
}

func TestRequeueStuckClosedPool(t *testing.T) {
	d, err := db.Connect(context.Background(), dbtest.URL())
	if err != nil {
		t.Skip("no DB")
	}
	d.Close()
	q := New(d.Pool)
	if _, err := q.requeueStuck(context.Background(), time.Minute); err == nil {
		t.Fatal("requeueStuck should error on a closed pool")
	}
}

func TestSetConcurrencyAndReaperLoop(t *testing.T) {
	d := dbtest.Connect(t)
	q := New(d.Pool)
	ctx := context.Background()
	if err := q.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(ctx, `DELETE FROM ws_jobs`); err != nil {
		t.Fatal(err)
	}
	q.SetConcurrency(0) // no-op (invalid)
	q.SetConcurrency(2) // override
	if q.concurrency != 2 {
		t.Fatalf("SetConcurrency: got %d", q.concurrency)
	}
	// Shorten reaper timing so the reaper goroutine runs within the test.
	origEvery, origTimeout := stuckReapEvery, stuckJobTimeout
	stuckReapEvery, stuckJobTimeout = 30*time.Millisecond, 10*time.Millisecond
	defer func() { stuckReapEvery, stuckJobTimeout = origEvery, origTimeout }()

	if _, err := d.Pool.Exec(ctx,
		`INSERT INTO ws_jobs (id,queue,data,state,started_at) VALUES ('orphan','source.fetch','{}','active', now() - interval '1 minute')`); err != nil {
		t.Fatal(err)
	}
	q.Start(ctx)
	defer q.Stop()
	deadline := time.Now().Add(3 * time.Second)
	var state string
	for time.Now().Before(deadline) {
		d.Pool.QueryRow(ctx, `SELECT state FROM ws_jobs WHERE id='orphan'`).Scan(&state)
		if state != "active" {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	if state == "active" {
		t.Fatal("reaper should have requeued the orphaned job")
	}
}
