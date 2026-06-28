package jobs_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
	"github.com/worldsignal/backend/internal/jobs"
	"github.com/worldsignal/backend/internal/llm"
)

const feed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel><title>F</title><link>https://f.example</link><description>d</description>
<item><title>Quake hits region</title><link>https://f.example/a</link><guid>g-a</guid>
  <pubDate>Mon, 02 Jan 2026 01:00:00 GMT</pubDate>
  <content:encoded><![CDATA[<p>A strong earthquake struck the region.</p>]]></content:encoded></item>
<item><title>Markets rally today</title><link>https://f.example/b</link><guid>g-b</guid>
  <pubDate>Mon, 02 Jan 2026 02:00:00 GMT</pubDate>
  <content:encoded><![CDATA[<p>Stocks climbed.</p>]]></content:encoded></item>
</channel></rss>`

func TestWorkerPipelineDrain(t *testing.T) {
	if testing.Short() {
		t.Skip("needs DB")
	}
	d := dbtest.Connect(t)
	ctx := context.Background()
	dbtest.Reset(t, d)
	dbtest.SeedTaxonomy(t, d)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(feed))
	}))
	defer srv.Close()

	ex := func(sql string, args ...any) {
		if _, err := d.Pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	ex(`INSERT INTO "Source" ("id","name","url","updatedAt") VALUES ('s1','S',$1,now())`, srv.URL)
	ex(`INSERT INTO "Subscriber" ("id","name","createdAt") VALUES ('__default__','Default',now())`)
	ex(`INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config","createdAt") VALUES ('p','__default__','All','POLLING','{}','{}',now())`)

	q := jobs.New(d.Pool)
	if err := q.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	ex(`TRUNCATE TABLE ws_jobs`)
	w := jobs.NewWorkers(q, d, llm.NewOpenAIGateway("", "gpt-4o-mini"), "secret")
	w.Register()
	q.Start(ctx)
	defer q.Stop()

	if err := w.EnqueueFetchSource("s1"); err != nil {
		t.Fatal(err)
	}

	// Wait for the full pipeline to drain into 2 SENT deliveries.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var sent int
		if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM "DeliveryEvent" WHERE "status"='SENT'`).Scan(&sent); err != nil {
			t.Fatal(err)
		}
		if sent == 2 {
			assertCount(t, d, `SELECT count(*) FROM "RawItem"`, 2)
			assertCount(t, d, `SELECT count(*) FROM "Article"`, 2)
			assertCount(t, d, `SELECT count(*) FROM "Signal"`, 2)
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("pipeline did not drain to 2 SENT deliveries in time")
}

func assertCount(t *testing.T, d *db.DB, query string, want int) {
	t.Helper()
	var got int
	if err := d.Pool.QueryRow(context.Background(), query).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s = %d, want %d", query, got, want)
	}
}

func TestSchedulerTick(t *testing.T) {
	if testing.Short() {
		t.Skip("needs DB")
	}
	d := dbtest.Connect(t)
	ctx := context.Background()
	dbtest.Reset(t, d)

	ex := func(sql string, args ...any) {
		if _, err := d.Pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	// Due: never fetched. Not due: just fetched. Disabled: ignored.
	ex(`INSERT INTO "Source" ("id","name","url","enabled","crawlFrequency","lastFetchedAt","updatedAt") VALUES ('due','D','https://d.example',true,300,NULL,now())`)
	ex(`INSERT INTO "Source" ("id","name","url","enabled","crawlFrequency","lastFetchedAt","updatedAt") VALUES ('fresh','F','https://f2.example',true,300,now(),now())`)
	ex(`INSERT INTO "Source" ("id","name","url","enabled","crawlFrequency","lastFetchedAt","updatedAt") VALUES ('off','O','https://o.example',false,300,NULL,now())`)

	q := jobs.New(d.Pool)
	if err := q.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	ex(`TRUNCATE TABLE ws_jobs`)
	w := jobs.NewWorkers(q, d, llm.NewOpenAIGateway("", ""), "secret")
	s := jobs.NewScheduler(d, w, time.Minute)

	due, err := s.Tick(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if due != 1 {
		t.Fatalf("expected 1 due source, got %d", due)
	}
	var jobsQueued int
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM ws_jobs WHERE queue=$1`, jobs.QFetchSource).Scan(&jobsQueued); err != nil {
		t.Fatal(err)
	}
	if jobsQueued != 1 {
		t.Fatalf("expected 1 fetch job, got %d", jobsQueued)
	}
}
