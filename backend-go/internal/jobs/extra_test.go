package jobs_test

import (
	"context"
	"testing"
	"time"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
	"github.com/worldsignal/backend/internal/jobs"
	"github.com/worldsignal/backend/internal/llm"
)

func closedPool(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Connect(context.Background(), dbtest.URL())
	if err != nil {
		t.Skip("no DB")
	}
	d.Close()
	return d
}

func TestQueueClosedPoolErrors(t *testing.T) {
	d := closedPool(t)
	q := jobs.New(d.Pool)
	ctx := context.Background()
	if err := q.Migrate(ctx); err == nil {
		t.Fatal("Migrate should error on closed pool")
	}
	if err := q.Send(ctx, "q", map[string]int{}, jobs.SendOptions{}); err == nil {
		t.Fatal("Send should error on closed pool")
	}
	if err := q.Send(ctx, "q", map[string]int{}, jobs.SendOptions{SingletonKey: "k"}); err == nil {
		t.Fatal("Send(singleton) should error on closed pool")
	}
}

func TestSchedulerStartStopAndClosed(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	q := jobs.New(d.Pool)
	if err := q.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(context.Background(), `TRUNCATE TABLE ws_jobs`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(context.Background(),
		`INSERT INTO "Source" ("id","name","url","enabled","crawlFrequency","updatedAt") VALUES ('s','S','https://s.example',true,1,now())`); err != nil {
		t.Fatal(err)
	}
	w := jobs.NewWorkers(q, d, llm.NewOpenAIGateway("", ""), "secret")
	s := jobs.NewScheduler(d, w, 30*time.Millisecond)
	s.Start(context.Background())
	time.Sleep(120 * time.Millisecond)
	s.Stop()
	s.Stop() // idempotent

	var n int
	if err := d.Pool.QueryRow(context.Background(), `SELECT count(*) FROM ws_jobs WHERE queue=$1`, jobs.QFetchSource).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("scheduler loop should have enqueued at least one fetch")
	}

	// Tick on a closed pool errors.
	cd := closedPool(t)
	cs := jobs.NewScheduler(cd, jobs.NewWorkers(jobs.New(cd.Pool), cd, llm.NewOpenAIGateway("", ""), "x"), time.Minute)
	if _, err := cs.Tick(context.Background(), time.Now()); err == nil {
		t.Fatal("Tick should error on closed pool")
	}
}

func TestSchedulerTickEnqueueError(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	q := jobs.New(d.Pool)
	if err := q.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(context.Background(),
		`INSERT INTO "Source" ("id","name","url","enabled","crawlFrequency","updatedAt") VALUES ('s','S','https://s.example',true,1,now())`); err != nil {
		t.Fatal(err)
	}
	// Hide the jobs table so enqueue (Send) fails while the Source query succeeds.
	if _, err := d.Pool.Exec(context.Background(), `ALTER TABLE ws_jobs RENAME TO ws_jobs__h`); err != nil {
		t.Fatal(err)
	}
	defer d.Pool.Exec(context.Background(), `ALTER TABLE ws_jobs__h RENAME TO ws_jobs`)
	w := jobs.NewWorkers(q, d, llm.NewOpenAIGateway("", ""), "x")
	s := jobs.NewScheduler(d, w, time.Minute)
	if _, err := s.Tick(context.Background(), time.Now()); err == nil {
		t.Fatal("Tick should error when enqueue fails")
	}
}

func TestQueueSendMarshalError(t *testing.T) {
	d := dbtest.Connect(t)
	q := jobs.New(d.Pool)
	if err := q.Send(context.Background(), "q", make(chan int), jobs.SendOptions{}); err == nil {
		t.Fatal("Send should error on unmarshalable data")
	}
}

func TestSchedulerStartWithFailingTick(t *testing.T) {
	cd := closedPool(t)
	w := jobs.NewWorkers(jobs.New(cd.Pool), cd, llm.NewOpenAIGateway("", ""), "x")
	s := jobs.NewScheduler(cd, w, 30*time.Millisecond)
	s.Start(context.Background()) // initial + ticked Tick calls error and log
	time.Sleep(80 * time.Millisecond)
	s.Stop()
}

func TestQueueStartWithFailingHandler(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	q := jobs.New(d.Pool)
	if err := q.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(context.Background(), `TRUNCATE TABLE ws_jobs`); err != nil {
		t.Fatal(err)
	}
	// A handler that always errors with retryLimit 0 → dead-letter via the loop.
	q.RegisterWorker("q.err", func(context.Context, []byte, bool) error { return context.Canceled })
	if err := q.Send(context.Background(), "q.err", map[string]int{}, jobs.SendOptions{}); err != nil {
		t.Fatal(err)
	}
	q.Start(context.Background())
	deadline := time.Now().Add(5 * time.Second)
	var state string
	for time.Now().Before(deadline) {
		if err := d.Pool.QueryRow(context.Background(), `SELECT state FROM ws_jobs WHERE queue='q.err'`).Scan(&state); err != nil {
			t.Fatal(err)
		}
		if state == "failed" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	q.Stop()
	if state != "failed" {
		t.Fatalf("expected failed, got %s", state)
	}
}

func TestQueueStartStopIdempotent(t *testing.T) {
	d := dbtest.Connect(t)
	q := jobs.New(d.Pool)
	if err := q.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	q.Start(context.Background())
	q.Stop()
	q.Stop() // no panic
}
