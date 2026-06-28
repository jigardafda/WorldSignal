package db_test

import (
	"context"
	"testing"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
	"github.com/worldsignal/backend/internal/jobs"
)

func TestEntityDBClosedErrors(t *testing.T) {
	d := closed(t)
	ctx := context.Background()
	e := func(name string, err error) {
		if err == nil {
			t.Fatalf("%s: expected error", name)
		}
	}
	_, _, err := d.ListArticles(ctx, db.ListFilter{})
	e("ListArticles", err)
	_, err = d.GetArticle(ctx, "x")
	e("GetArticle", err)
	_, _, err = d.ListRawItems(ctx, db.ListFilter{})
	e("ListRawItems", err)
	_, err = d.GetRawItemDetail(ctx, "x")
	e("GetRawItemDetail", err)
	_, _, err = d.ListDeliveriesFiltered(ctx, db.DeliveryFilter{})
	e("ListDeliveriesFiltered", err)
	_, err = d.GetDeliveryDetail(ctx, "x")
	e("GetDeliveryDetail", err)
	_, err = d.ResetDeliveryForRetry(ctx, "x")
	e("ResetDeliveryForRetry", err)
	_, _, err = d.ListJobs(ctx, db.JobFilter{})
	e("ListJobs", err)
	_, err = d.JobStateCounts(ctx)
	e("JobStateCounts", err)
	_, err = d.RetryJob(ctx, "x")
	e("RetryJob", err)
	_, err = d.TaxonomyCounts(ctx)
	e("TaxonomyCounts", err)
	_, err = d.UpdateSourceFull(ctx, "x", db.SourceFullPatch{})
	e("UpdateSourceFull", err)
	_, err = d.DeleteSource(ctx, "x")
	e("DeleteSource", err)
	_, err = d.UpdateSubscription(ctx, "x", db.SubscriptionPatch{Name: strPtr("n")})
	e("UpdateSubscription", err)
	_, err = d.DeleteSubscription(ctx, "x")
	e("DeleteSubscription", err)
	_, err = d.ListSubscribers(ctx)
	e("ListSubscribers", err)
	_, err = d.CreateSubscriber(ctx, "n")
	e("CreateSubscriber", err)
	_, err = d.DeleteSubscriber(ctx, "x")
	e("DeleteSubscriber", err)
	_, err = d.CountSignals(ctx, db.SignalFilter{})
	e("CountSignals", err)
	_, err = d.SignalsBySeverity(ctx)
	e("SignalsBySeverity", err)
	_, err = d.SignalsByStatus(ctx)
	e("SignalsByStatus", err)
	_, err = d.SignalsByEventType(ctx, 5)
	e("SignalsByEventType", err)
	_, err = d.SignalsByCountry(ctx, 5)
	e("SignalsByCountry", err)
	_, err = d.SignalsOverTime(ctx, 7)
	e("SignalsOverTime", err)
	_, err = d.TopSources(ctx, 5)
	e("TopSources", err)
	_, err = d.GetDeliveryStats(ctx)
	e("GetDeliveryStats", err)
	_, err = d.GetIngestionStats(ctx)
	e("GetIngestionStats", err)
}

func TestEntityDBHappyEdges(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	dbtest.SeedTaxonomy(t, d)
	ctx := context.Background()
	ex := func(q string, a ...any) {
		if _, err := d.Pool.Exec(ctx, q, a...); err != nil {
			t.Fatal(err)
		}
	}
	ex(`INSERT INTO "Source" ("id","name","url","updatedAt") VALUES ('s','S','https://s.example',now())`)
	ex(`INSERT INTO "RawItem" ("id","sourceId","rawTitle","status","rawPayload") VALUES ('r','s','T','PARSED','{"a":1}')`)
	ex(`INSERT INTO "Article" ("id","sourceId","title","body") VALUES ('a','s','Quake','body')`)
	ex(`INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","updatedAt") VALUES ('sg','T','S',now(),now(),now())`)
	ex(`INSERT INTO "SignalArticle" ("signalId","articleId","relationType") VALUES ('sg','a','PRIMARY')`)
	ex(`INSERT INTO "Subscriber" ("id","name") VALUES ('__default__','D')`)
	ex(`INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config") VALUES ('sub','__default__','All','POLLING','{}','{}')`)
	ex(`INSERT INTO "DeliveryEvent" ("id","subscriptionId","signalId","channel","status","payload") VALUES ('d','sub','sg','POLLING','FAILED','{}')`)

	// Filtered lists + clampLimit branches.
	src := "s"
	st := "PARSED"
	if rows, total, err := d.ListArticles(ctx, db.ListFilter{SourceID: &src, Search: strPtr("Quake"), Limit: 0}); err != nil || total != 1 || len(rows) != 1 {
		t.Fatalf("ListArticles: %v %d", err, total)
	}
	if _, _, err := d.ListArticles(ctx, db.ListFilter{Limit: 500}); err != nil { // clamp >200
		t.Fatal(err)
	}
	if rows, total, err := d.ListRawItems(ctx, db.ListFilter{SourceID: &src, Status: &st}); err != nil || total != 1 || len(rows) != 1 {
		t.Fatalf("ListRawItems: %v %d", err, total)
	}
	subID := "sub"
	dst := "FAILED"
	if _, total, err := d.ListDeliveriesFiltered(ctx, db.DeliveryFilter{Status: &dst, SubscriptionID: &subID}); err != nil || total != 1 {
		t.Fatalf("ListDeliveriesFiltered: %v %d", err, total)
	}

	// Detail nils.
	if a, err := d.GetArticle(ctx, "missing"); err != nil || a != nil {
		t.Fatalf("GetArticle missing: %v %v", a, err)
	}
	if r, err := d.GetRawItemDetail(ctx, "missing"); err != nil || r != nil {
		t.Fatalf("GetRawItemDetail missing: %v %v", r, err)
	}
	if dd, err := d.GetDeliveryDetail(ctx, "missing"); err != nil || dd != nil {
		t.Fatalf("GetDeliveryDetail missing: %v %v", dd, err)
	}
	if a, err := d.GetArticle(ctx, "a"); err != nil || a == nil || len(a.Signals) != 1 {
		t.Fatalf("GetArticle: %v %v", a, err)
	}
	if r, err := d.GetRawItemDetail(ctx, "r"); err != nil || r == nil {
		t.Fatalf("GetRawItemDetail: %v %v", r, err)
	}
	if dd, err := d.GetDeliveryDetail(ctx, "d"); err != nil || dd == nil {
		t.Fatalf("GetDeliveryDetail: %v %v", dd, err)
	}

	// Retry + deletes (present + missing).
	if ok, err := d.ResetDeliveryForRetry(ctx, "d"); err != nil || !ok {
		t.Fatalf("reset delivery: %v %v", ok, err)
	}
	if ok, _ := d.ResetDeliveryForRetry(ctx, "missing"); ok {
		t.Fatal("reset missing should be false")
	}
	if u, err := d.UpdateSourceFull(ctx, "missing", db.SourceFullPatch{Name: strPtr("x")}); err != nil || u != nil {
		t.Fatalf("update missing source: %v %v", u, err)
	}
	if u, err := d.UpdateSourceFull(ctx, "s", db.SourceFullPatch{Name: strPtr("New"), Country: strPtr("US"), Priority: intPtr(2), Credibility: f64Ptr(0.8), CrawlFrequency: intPtr(600), Enabled: boolPtr(false)}); err != nil || u.Name != "New" {
		t.Fatalf("update source: %v %v", u, err)
	}
	if sub, err := d.UpdateSubscription(ctx, "sub", db.SubscriptionPatch{Name: strPtr("Renamed"), Enabled: boolPtr(false), Filter: db.RawJSON(`{"x":1}`), Config: db.RawJSON(`{"y":2}`)}); err != nil || sub.Name != "Renamed" {
		t.Fatalf("update subscription: %v %v", sub, err)
	}
	if sub, err := d.UpdateSubscription(ctx, "sub", db.SubscriptionPatch{}); err != nil || sub == nil { // no-op path
		t.Fatalf("update subscription no-op: %v %v", sub, err)
	}

	// Subscribers.
	if subs, err := d.ListSubscribers(ctx); err != nil || len(subs) != 1 || subs[0].SubscriptionCount != 1 {
		t.Fatalf("ListSubscribers: %+v %v", subs, err)
	}
	ns, err := d.CreateSubscriber(ctx, "New")
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := d.DeleteSubscriber(ctx, ns.ID); err != nil || !ok {
		t.Fatalf("delete subscriber: %v %v", ok, err)
	}
	if ok, _ := d.DeleteSubscriber(ctx, "missing"); ok {
		t.Fatal("delete missing subscriber should be false")
	}

	// Counts + analytics.
	if n, err := d.CountSignals(ctx, db.SignalFilter{}); err != nil || n != 1 {
		t.Fatalf("CountSignals: %d %v", n, err)
	}
	for _, fn := range []func() error{
		func() error { _, e := d.SignalsBySeverity(ctx); return e },
		func() error { _, e := d.SignalsByStatus(ctx); return e },
		func() error { _, e := d.SignalsByEventType(ctx, 5); return e },
		func() error { _, e := d.SignalsByCountry(ctx, 5); return e },
		func() error { _, e := d.SignalsOverTime(ctx, 7); return e },
		func() error { _, e := d.TopSources(ctx, 5); return e },
		func() error { _, e := d.GetDeliveryStats(ctx); return e },
		func() error { _, e := d.GetIngestionStats(ctx); return e },
		func() error { _, e := d.TaxonomyCounts(ctx); return e },
	} {
		if err := fn(); err != nil {
			t.Fatalf("analytics fn: %v", err)
		}
	}

	// Jobs (needs ws_jobs).
	q := jobs.New(d.Pool)
	if err := q.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	ex(`TRUNCATE TABLE ws_jobs`)
	ex(`INSERT INTO ws_jobs (id,queue,state) VALUES ('j','source.fetch','failed')`)
	queue := "source.fetch"
	state := "failed"
	if rows, total, err := d.ListJobs(ctx, db.JobFilter{Queue: &queue, State: &state}); err != nil || total != 1 || len(rows) != 1 {
		t.Fatalf("ListJobs: %v %d", err, total)
	}
	if bs, err := d.JobStateCounts(ctx); err != nil || len(bs) != 1 {
		t.Fatalf("JobStateCounts: %v %v", bs, err)
	}
	if ok, err := d.RetryJob(ctx, "j"); err != nil || !ok {
		t.Fatalf("RetryJob: %v %v", ok, err)
	}
	if ok, _ := d.RetryJob(ctx, "missing"); ok {
		t.Fatal("retry missing job should be false")
	}
}

func intPtr(n int) *int         { return &n }
func f64Ptr(f float64) *float64 { return &f }
func boolPtr(b bool) *bool      { return &b }
