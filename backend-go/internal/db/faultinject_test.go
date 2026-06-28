package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
)

// hideTable renames a table away so queries against it fail, returning a restore
// func. Used to exercise inner error branches that run after a successful first
// query (which a closed pool cannot reach).
func hideTable(t *testing.T, d *db.DB, tbl string) func() {
	t.Helper()
	ctx := context.Background()
	if _, err := d.Pool.Exec(ctx, `ALTER TABLE "`+tbl+`" RENAME TO "`+tbl+`__hidden"`); err != nil {
		t.Fatalf("hide %s: %v", tbl, err)
	}
	return func() {
		_, _ = d.Pool.Exec(ctx, `ALTER TABLE "`+tbl+`__hidden" RENAME TO "`+tbl+`"`)
	}
}

func seedOne(t *testing.T, d *db.DB) {
	t.Helper()
	dbtest.Reset(t, d)
	dbtest.SeedTaxonomy(t, d)
	ctx := context.Background()
	ex := func(q string, a ...any) {
		if _, err := d.Pool.Exec(ctx, q, a...); err != nil {
			t.Fatal(err)
		}
	}
	ex(`INSERT INTO "Source" ("id","name","url","updatedAt") VALUES ('s1','S','https://s.example',now())`)
	ex(`INSERT INTO "Signal" ("id","title","summary","status","severity","confidence","sourceCount","firstSeenAt","lastSeenAt","updatedAt","metadata") VALUES ('sg','T','S','CONFIRMED','HIGH',0.8,1,now(),now(),now(),'{"tokenSet":"x"}')`)
	ex(`INSERT INTO "Article" ("id","sourceId","title","body","publishedAt") VALUES ('a1','s1','A','body',now())`)
	ex(`INSERT INTO "SignalArticle" ("signalId","articleId","relationType","similarityScore") VALUES ('sg','a1','PRIMARY',1)`)
	ex(`INSERT INTO "SignalTag" ("signalId","tagId","confidence") SELECT 'sg',"id",0.9 FROM "TaxonomyTag" WHERE "code"='DISASTER.EARTHQUAKE'`)
	ex(`INSERT INTO "Subscriber" ("id","name","createdAt") VALUES ('__default__','D',now())`)
	ex(`INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config","createdAt") VALUES ('sub','__default__','Sub','POLLING','{}','{}',now())`)
	ex(`INSERT INTO "DeliveryEvent" ("id","subscriptionId","signalId","channel","status","payload","createdAt") VALUES ('d1','sub','sg','POLLING','SENT','{"event_id":"e"}',now())`)
}

func TestInnerQueryErrorBranches(t *testing.T) {
	d := dbtest.Connect(t)
	ctx := context.Background()

	type tc struct {
		name string
		hide string
		call func() error
	}
	cases := []tc{
		{"loadAggregate.signalTags", "SignalTag", func() error { _, e := d.GetSignal(ctx, "sg"); return e }},
		{"loadAggregate.signalSources", "Article", func() error { _, e := d.GetSignal(ctx, "sg"); return e }},
		{"listSignals.aggregate", "SignalTag", func() error { _, e := d.ListSignals(ctx, db.SignalFilter{}); return e }},
		{"listSubscriptions.subscriber", "Subscriber", func() error { _, e := d.ListSubscriptions(ctx); return e }},
		{"listSubscriptions.count", "DeliveryEvent", func() error { _, e := d.ListSubscriptions(ctx); return e }},
		{"listDeliveries.subscription", "Subscription", func() error { _, e := d.ListDeliveries(ctx, 10); return e }},
		{"listDeliveries.signalTitle", "Signal", func() error { _, e := d.ListDeliveries(ctx, 10); return e }},
		{"loadSignalForEnrich.links", "Article", func() error { _, e := d.LoadSignalForEnrich(ctx, "sg"); return e }},
		{"applyEnrichment.tx", "SignalTag", func() error {
			return d.ApplyEnrichment(ctx, "sg", db.EnrichmentUpdate{PublishedAt: time.Now(), Metadata: map[string]any{}})
		}},
		{"createSignalFromArticle.tx", "SignalArticle", func() error {
			_, e := d.CreateSignalFromArticle(ctx, &db.ClusterArticle{ID: "a1", Title: "T"}, time.Now())
			return e
		}},
		{"attachArticle.tx", "SignalArticle", func() error {
			return d.AttachArticleToSignal(ctx, "sg", "a1", 0.6, time.Now())
		}},
		{"createSubscription.insert", "Subscription", func() error {
			_, e := d.CreateSubscription(ctx, db.CreateSubscriptionInput{Name: "n"})
			return e
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			seedOne(t, d)
			restore := hideTable(t, d, c.hide)
			defer restore()
			if err := c.call(); err == nil {
				t.Fatalf("%s: expected error with %s hidden", c.name, c.hide)
			}
		})
	}
}
