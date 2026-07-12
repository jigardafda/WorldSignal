package jobs_test

import (
	"context"
	"testing"
	"time"

	"github.com/worldsignal/backend/internal/dbtest"
	"github.com/worldsignal/backend/internal/jobs"
	"github.com/worldsignal/backend/internal/llm"
)

func TestDigesterTickBuildsAndEnqueues(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	q := jobs.New(d.Pool)
	if err := q.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(ctx, `TRUNCATE TABLE ws_jobs`); err != nil {
		t.Fatal(err)
	}
	ex := func(sql string, a ...any) {
		if _, err := d.Pool.Exec(ctx, sql, a...); err != nil {
			t.Fatalf("exec: %v", err)
		}
	}
	// Hourly digest, last fired 2h ago → due now.
	ex(`INSERT INTO "Subscription" ("id","name","channel","filter","config","lastDigestAt","createdAt") VALUES ('dig','digest','EMAIL','{}','{"mode":"digest","interval":"hourly","to":"r@x.com"}', now() - interval '2 hours', now())`)
	ex(`INSERT INTO "Signal" ("id","title","summary","severity","confidence","sourceCount","firstSeenAt","lastSeenAt","updatedAt") VALUES ('s1','First','A','HIGH',0.8,1,now(),now(),now())`)
	ex(`INSERT INTO "DigestQueue" ("subscriptionId","signalId","queuedAt") VALUES ('dig','s1',now())`)

	w := jobs.NewWorkers(q, d, llm.NewOpenAIGateway("", ""), "secret")
	dg := jobs.NewDigester(d, w, time.Minute)
	n, err := dg.Tick(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 digest enqueued, got %d", n)
	}
	// A delivery.send job was enqueued.
	var jobsN int
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM ws_jobs WHERE queue=$1`, jobs.QSendDelivery).Scan(&jobsN); err != nil {
		t.Fatal(err)
	}
	if jobsN != 1 {
		t.Fatalf("expected 1 delivery job, got %d", jobsN)
	}
}

func TestDigesterNotDue(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	ex := func(sql string, a ...any) {
		if _, err := d.Pool.Exec(ctx, sql, a...); err != nil {
			t.Fatalf("exec: %v", err)
		}
	}
	// Daily digest that just fired → not due.
	ex(`INSERT INTO "Subscription" ("id","name","channel","filter","config","lastDigestAt","createdAt") VALUES ('dig','digest','EMAIL','{}','{"mode":"digest","interval":"daily","to":"r@x.com"}', now(), now())`)
	ex(`INSERT INTO "Signal" ("id","title","summary","severity","confidence","sourceCount","firstSeenAt","lastSeenAt","updatedAt") VALUES ('s1','First','A','HIGH',0.8,1,now(),now(),now())`)
	ex(`INSERT INTO "DigestQueue" ("subscriptionId","signalId","queuedAt") VALUES ('dig','s1',now())`)

	w := jobs.NewWorkers(jobs.New(d.Pool), d, llm.NewOpenAIGateway("", ""), "secret")
	dg := jobs.NewDigester(d, w, 0) // 0 → defaults to 1m tick
	n, err := dg.Tick(ctx, time.Now())
	if err != nil || n != 0 {
		t.Fatalf("not-due digest should not fire: n=%d err=%v", n, err)
	}
	// The item is still queued.
	var queued int
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM "DigestQueue" WHERE "subscriptionId"='dig'`).Scan(&queued); err != nil {
		t.Fatal(err)
	}
	if queued != 1 {
		t.Fatalf("item should remain queued, got %d", queued)
	}
}

func TestDigesterTickErrorPaths(t *testing.T) {
	d := dbtest.Connect(t)
	ctx := context.Background()

	// Closed pool → PendingDigests errors → Tick returns error.
	cd := closedPool(t)
	w := jobs.NewWorkers(jobs.New(cd.Pool), cd, llm.NewOpenAIGateway("", ""), "s")
	if _, err := jobs.NewDigester(cd, w, time.Minute).Tick(ctx, time.Now()); err == nil {
		t.Fatal("expected error on closed pool")
	}

	// A due digest whose BuildDigest fails (Signal table hidden) is skipped, not fatal.
	dbtest.Reset(t, d)
	ex := func(sql string, a ...any) {
		if _, err := d.Pool.Exec(ctx, sql, a...); err != nil {
			t.Fatalf("exec: %v", err)
		}
	}
	ex(`INSERT INTO "Subscription" ("id","name","channel","filter","config","lastDigestAt","createdAt") VALUES ('dig','d','EMAIL','{}','{"mode":"digest","interval":"hourly","to":"r@x.com"}', now() - interval '2 hours', now())`)
	ex(`INSERT INTO "Signal" ("id","title","summary","severity","confidence","sourceCount","firstSeenAt","lastSeenAt","updatedAt") VALUES ('s1','T','A','LOW',0.5,1,now(),now(),now())`)
	ex(`INSERT INTO "DigestQueue" ("subscriptionId","signalId","queuedAt") VALUES ('dig','s1',now())`)
	if _, err := d.Pool.Exec(ctx, `ALTER TABLE "Article" RENAME TO "Article__h"`); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = d.Pool.Exec(ctx, `ALTER TABLE "Article__h" RENAME TO "Article"`) }()
	dg := jobs.NewDigester(d, jobs.NewWorkers(jobs.New(d.Pool), d, llm.NewOpenAIGateway("", ""), "s"), time.Minute)
	n, err := dg.Tick(ctx, time.Now())
	if err != nil {
		t.Fatalf("build failure should be skipped, not returned: %v", err)
	}
	if n != 0 {
		t.Fatalf("no digest should be enqueued, got %d", n)
	}
}

func TestDigesterStartStop(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	q := jobs.New(d.Pool)
	if err := q.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	w := jobs.NewWorkers(q, d, llm.NewOpenAIGateway("", ""), "secret")
	dg := jobs.NewDigester(d, w, 20*time.Millisecond)
	dg.Start(ctx)
	time.Sleep(60 * time.Millisecond)
	dg.Stop()
	dg.Stop() // idempotent
}
