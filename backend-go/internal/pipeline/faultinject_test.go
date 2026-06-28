package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
	"github.com/worldsignal/backend/internal/llm"
)

func hide(t *testing.T, d *db.DB, tbl string) func() {
	t.Helper()
	ctx := context.Background()
	if _, err := d.Pool.Exec(ctx, `ALTER TABLE "`+tbl+`" RENAME TO "`+tbl+`__h"`); err != nil {
		t.Fatalf("hide %s: %v", tbl, err)
	}
	return func() { d.Pool.Exec(ctx, `ALTER TABLE "`+tbl+`__h" RENAME TO "`+tbl+`"`) }
}

// TestStagePostQueryErrors hits each stage's error branch that runs after its
// first query succeeds, by hiding a table used only by a later query.
func TestStagePostQueryErrors(t *testing.T) {
	d := conn(t)
	ctx := context.Background()
	now := time.Now()
	gw := llm.NewOpenAIGateway("", "")
	mk := func(sql string, a ...any) { mustExec(t, d, sql, a...) }

	t.Run("normalize.findDup", func(t *testing.T) {
		dbtest.Reset(t, d)
		mk(`INSERT INTO "Source" ("id","name","url","updatedAt") VALUES ('s','S','https://s.example',now())`)
		mk(`INSERT INTO "RawItem" ("id","sourceId","rawTitle","rawContent","status") VALUES ('r','s','T','B','PENDING')`)
		defer hide(t, d, "Article")()
		if _, err := NormalizeRawItem(ctx, d, "r"); err == nil {
			t.Fatal("expected dup-check error")
		}
	})

	t.Run("cluster.candidates", func(t *testing.T) {
		dbtest.Reset(t, d)
		mk(`INSERT INTO "Source" ("id","name","url","updatedAt") VALUES ('s','S','https://s.example',now())`)
		mk(`INSERT INTO "Article" ("id","sourceId","title","tokenSet") VALUES ('a','s','T','quake')`)
		defer hide(t, d, "Signal")()
		if _, err := ClusterArticle(ctx, d, "a", now); err == nil {
			t.Fatal("expected candidates error")
		}
	})

	t.Run("enrich.tagIDs", func(t *testing.T) {
		dbtest.Reset(t, d)
		dbtest.SeedTaxonomy(t, d)
		mk(`INSERT INTO "Source" ("id","name","url","updatedAt") VALUES ('s','S','https://s.example',now())`)
		mk(`INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","updatedAt") VALUES ('sg','T','S',now(),now(),now())`)
		mk(`INSERT INTO "Article" ("id","sourceId","title","body") VALUES ('a','s','T','earthquake struck')`)
		mk(`INSERT INTO "SignalArticle" ("signalId","articleId","relationType") VALUES ('sg','a','PRIMARY')`)
		defer hide(t, d, "TaxonomyTag")()
		if err := EnrichSignal(ctx, d, gw, "sg", now); err == nil {
			t.Fatal("expected tagIDs error")
		}
	})

	t.Run("match.enabledSubs", func(t *testing.T) {
		dbtest.Reset(t, d)
		dbtest.SeedTaxonomy(t, d)
		mk(`INSERT INTO "Signal" ("id","title","summary","severity","confidence","firstSeenAt","lastSeenAt","updatedAt") VALUES ('sg','T','S','HIGH',0.8,now(),now(),now())`)
		mk(`INSERT INTO "SignalTag" ("signalId","tagId","confidence") SELECT 'sg',"id",0.9 FROM "TaxonomyTag" WHERE "code"='DISASTER.FLOOD'`)
		defer hide(t, d, "Subscription")()
		if _, err := MatchSubscriptions(ctx, d, "sg", now); err == nil {
			t.Fatal("expected enabledSubs error")
		}
	})

	t.Run("cluster.createNewTxError", func(t *testing.T) {
		dbtest.Reset(t, d)
		mk(`INSERT INTO "Source" ("id","name","url","updatedAt") VALUES ('s','S','https://s.example',now())`)
		mk(`INSERT INTO "Article" ("id","sourceId","title","tokenSet","publishedAt") VALUES ('a','s','T','unique tokens here',now())`)
		defer hide(t, d, "SignalArticle")()
		if _, err := ClusterArticle(ctx, d, "a", now); err == nil {
			t.Fatal("expected createSignal tx error")
		}
	})

	t.Run("cluster.attachTxError", func(t *testing.T) {
		dbtest.Reset(t, d)
		mk(`INSERT INTO "Source" ("id","name","url","updatedAt") VALUES ('s','S','https://s.example',now())`)
		mk(`INSERT INTO "Article" ("id","sourceId","title","tokenSet","publishedAt") VALUES ('a','s','T','quake region strong',now())`)
		mk(`INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","sourceCount","metadata","updatedAt") VALUES ('sg','T','S',now(),now(),1,'{"tokenSet":"quake region strong"}',now())`)
		defer hide(t, d, "SignalArticle")()
		if _, err := ClusterArticle(ctx, d, "a", now); err == nil {
			t.Fatal("expected attach tx error")
		}
	})

	t.Run("enrich.applyError", func(t *testing.T) {
		dbtest.Reset(t, d)
		dbtest.SeedTaxonomy(t, d)
		mk(`INSERT INTO "Source" ("id","name","url","updatedAt") VALUES ('s','S','https://s.example',now())`)
		mk(`INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","updatedAt") VALUES ('sg','T','S',now(),now(),now())`)
		mk(`INSERT INTO "Article" ("id","sourceId","title","body") VALUES ('a','s','T','earthquake struck')`)
		mk(`INSERT INTO "SignalArticle" ("signalId","articleId","relationType") VALUES ('sg','a','PRIMARY')`)
		defer hide(t, d, "SignalTag")()
		if err := EnrichSignal(ctx, d, gw, "sg", now); err == nil {
			t.Fatal("expected applyEnrichment error")
		}
	})

	t.Run("match.createDeliveryError", func(t *testing.T) {
		dbtest.Reset(t, d)
		dbtest.SeedTaxonomy(t, d)
		mk(`INSERT INTO "Signal" ("id","title","summary","severity","confidence","firstSeenAt","lastSeenAt","updatedAt") VALUES ('sg','T','S','HIGH',0.8,now(),now(),now())`)
		mk(`INSERT INTO "SignalTag" ("signalId","tagId","confidence") SELECT 'sg',"id",0.9 FROM "TaxonomyTag" WHERE "code"='DISASTER.FLOOD'`)
		mk(`INSERT INTO "Subscriber" ("id","name","createdAt") VALUES ('__default__','D',now())`)
		mk(`INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config","createdAt") VALUES ('sub','__default__','S','POLLING','{}','{}',now())`)
		defer hide(t, d, "DeliveryEvent")()
		if _, err := MatchSubscriptions(ctx, d, "sg", now); err == nil {
			t.Fatal("expected createDelivery error")
		}
	})
}
