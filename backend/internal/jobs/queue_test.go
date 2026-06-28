package jobs_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
	"github.com/worldsignal/backend/internal/jobs"
)

func setup(t *testing.T) (*db.DB, *jobs.Queue) {
	t.Helper()
	d := dbtest.Connect(t)
	q := jobs.New(d.Pool)
	if err := q.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(context.Background(), `TRUNCATE TABLE ws_jobs`); err != nil {
		t.Fatal(err)
	}
	return d, q
}

func jobState(t *testing.T, d *db.DB, queue string) (string, int) {
	t.Helper()
	var state string
	var rc int
	err := d.Pool.QueryRow(context.Background(),
		`SELECT state, retry_count FROM ws_jobs WHERE queue=$1 ORDER BY created_at LIMIT 1`, queue).Scan(&state, &rc)
	if err != nil {
		t.Fatalf("jobState: %v", err)
	}
	return state, rc
}

func TestSendAndProcessSuccess(t *testing.T) {
	d, q := setup(t)
	ctx := context.Background()
	var got string
	q.RegisterWorker("q.success", func(_ context.Context, data []byte, _ bool) error {
		got = string(data)
		return nil
	})
	if err := q.Send(ctx, "q.success", map[string]string{"k": "v"}, jobs.SendOptions{}); err != nil {
		t.Fatal(err)
	}
	worked, err := q.ProcessOneForTest(ctx, "q.success")
	if err != nil || !worked {
		t.Fatalf("processOne worked=%v err=%v", worked, err)
	}
	if got != `{"k": "v"}` { // jsonb round-trip canonicalizes with spaces
		t.Fatalf("handler got %q", got)
	}
	if state, _ := jobState(t, d, "q.success"); state != "completed" {
		t.Fatalf("state %q", state)
	}
}

func TestSingletonDedupe(t *testing.T) {
	d, q := setup(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := q.Send(ctx, "q.single", map[string]int{"n": i}, jobs.SendOptions{SingletonKey: "fetch:1"}); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM ws_jobs WHERE queue='q.single'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("singletonKey should dedupe to 1 job, got %d", count)
	}
}

func TestRetryThenSucceed(t *testing.T) {
	d, q := setup(t)
	ctx := context.Background()
	attempts := 0
	q.RegisterWorker("q.retry", func(_ context.Context, _ []byte, _ bool) error {
		attempts++
		if attempts < 2 {
			return errFail
		}
		return nil
	})
	if err := q.Send(ctx, "q.retry", map[string]int{}, jobs.SendOptions{RetryLimit: 3, RetryDelay: 0}); err != nil {
		t.Fatal(err)
	}
	// First attempt fails → rescheduled.
	q.ProcessOneForTest(ctx, "q.retry")
	if state, rc := jobState(t, d, "q.retry"); state != "created" || rc != 1 {
		t.Fatalf("after fail: state=%s rc=%d", state, rc)
	}
	// Second attempt succeeds.
	q.ProcessOneForTest(ctx, "q.retry")
	if state, _ := jobState(t, d, "q.retry"); state != "completed" {
		t.Fatalf("after success: state=%s", state)
	}
}

func TestDeadLetterAfterLimit(t *testing.T) {
	d, q := setup(t)
	ctx := context.Background()
	finals := 0
	q.RegisterWorker("q.dead", func(_ context.Context, _ []byte, isFinal bool) error {
		if isFinal {
			finals++
		}
		return errFail // always fails
	})
	if err := q.Send(ctx, "q.dead", map[string]int{}, jobs.SendOptions{RetryLimit: 2, RetryDelay: 0}); err != nil {
		t.Fatal(err)
	}
	// Attempts: rc 0,1,2 → then dead-letter.
	for i := 0; i < 3; i++ {
		if worked, err := q.ProcessOneForTest(ctx, "q.dead"); err != nil || !worked {
			t.Fatalf("attempt %d worked=%v err=%v", i, worked, err)
		}
	}
	if state, rc := jobState(t, d, "q.dead"); state != "failed" || rc != 2 {
		t.Fatalf("dead-letter: state=%s rc=%d", state, rc)
	}
	if finals != 1 {
		t.Fatalf("expected handler to see isFinal once, got %d", finals)
	}
	// No more work.
	if worked, _ := q.ProcessOneForTest(ctx, "q.dead"); worked {
		t.Fatal("expected no further work after dead-letter")
	}
}

func TestBackgroundStartStop(t *testing.T) {
	_, q := setup(t)
	ctx := context.Background()
	var mu sync.Mutex
	processed := 0
	q.RegisterWorker("q.bg", func(_ context.Context, _ []byte, _ bool) error {
		mu.Lock()
		processed++
		mu.Unlock()
		return nil
	})
	q.Start(ctx)
	defer q.Stop()
	if err := q.Send(ctx, "q.bg", map[string]int{}, jobs.SendOptions{}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := processed
		mu.Unlock()
		if n > 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("background worker did not process the job")
}

type errString string

func (e errString) Error() string { return string(e) }

const errFail = errString("boom")
