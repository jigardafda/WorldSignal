package parity_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
	"github.com/worldsignal/backend/internal/httpapi"
	"github.com/worldsignal/backend/internal/parity"
)

// goServer starts the Go HTTP handler in-process against the shared DB.
func goServer(t *testing.T, d *db.DB) *parity.Server {
	t.Helper()
	srv := &httpapi.Server{DB: d, Enqueue: &noopEnqueuer{}, SigningSecret: "parity-secret"}
	ht := httptest.NewServer(srv.Handler())
	t.Cleanup(ht.Close)
	return &parity.Server{BaseURL: ht.URL}
}

type noopEnqueuer struct{ last string }

func (n *noopEnqueuer) EnqueueFetchSource(id string) error  { n.last = id; return nil }
func (n *noopEnqueuer) EnqueueSendDelivery(id string) error { n.last = id; return nil }

// insertFixtures populates a deterministic dataset (fixed ids + timestamps) so
// both backends read identical rows. Returns key ids.
func insertFixtures(t *testing.T, d *db.DB) {
	t.Helper()
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		if _, err := d.Pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture exec failed: %v\nSQL: %s", err, sql)
		}
	}

	// Sources: one fully populated, one with nulls; fixed timestamps.
	exec(`INSERT INTO "Source"
		("id","name","type","url","country","region","language","category","priority","credibility","crawlFrequency","parserType","enabled","config","lastFetchedAt","lastSuccessAt","lastFailureAt","failureCount","createdAt","updatedAt")
		VALUES
		('src_alpha','Alpha News','RSS','https://alpha.example/feed','US','NA','en','general',1,0.9,300,'rss',true,'{"k":"v"}','2026-01-02T03:04:05.000Z','2026-01-02T03:04:05.000Z',NULL,0,'2026-01-01T00:00:00.000Z','2026-01-01T00:00:00.000Z')`)
	exec(`INSERT INTO "Source"
		("id","name","type","url","country","region","language","category","priority","credibility","crawlFrequency","parserType","enabled","config","lastFetchedAt","lastSuccessAt","lastFailureAt","failureCount","createdAt","updatedAt")
		VALUES
		('src_zeta','Zeta Wire','ATOM','https://zeta.example/feed',NULL,NULL,NULL,NULL,3,0.5,900,'rss',false,NULL,NULL,NULL,'2026-01-03T03:04:05.000Z',2,'2026-01-01T00:00:00.000Z','2026-01-01T00:00:00.000Z')`)

	// Articles (need a source).
	exec(`INSERT INTO "Article" ("id","sourceId","canonicalUrl","title","body","summary","publishedAt","contentHash","tokenSet")
		VALUES ('art_1','src_alpha','https://alpha.example/a','Quake hits region','A strong earthquake struck the region.','A strong earthquake struck the region.','2026-01-02T01:00:00.000Z','hash1','earthquake region strong struck')`)
	exec(`INSERT INTO "Article" ("id","sourceId","canonicalUrl","title","body","summary","publishedAt","contentHash","tokenSet")
		VALUES ('art_2','src_alpha','https://alpha.example/b','Markets rally','Stocks climbed today.','Stocks climbed today.','2026-01-02T02:00:00.000Z','hash2','climbed markets stocks today')`)

	// Subscriber + subscriptions (one webhook, one polling).
	exec(`INSERT INTO "Subscriber" ("id","name","status","createdAt") VALUES ('__default__','Default Subscriber','ACTIVE','2026-01-01T00:00:00.000Z')`)
	exec(`INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config","enabled","createdAt")
		VALUES ('sub_poll','__default__','All signals (polling)','POLLING','{}','{}',true,'2026-01-01T00:00:00.000Z')`)
	exec(`INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config","enabled","createdAt")
		VALUES ('sub_hook','__default__','Disasters webhook','WEBHOOK','{"tags":["DISASTER"],"minConfidence":0.5}','{"url":"https://hook.example/in"}',true,'2026-01-01T00:00:01.000Z')`)

	// Signals with tags + article links.
	exec(`INSERT INTO "Signal"
		("id","title","summary","whatHappened","whyItMatters","status","severity","confidence","eventType","country","region","firstSeenAt","lastSeenAt","publishedAt","createdAt","updatedAt","sourceCount","metadata")
		VALUES
		('sig_1','Earthquake strikes region','A strong earthquake struck.','A strong earthquake struck.','Thousands affected.','CONFIRMED','HIGH',0.82,'DISASTER.EARTHQUAKE','US','NA','2026-01-02T01:05:00.000Z','2026-01-02T01:30:00.000Z','2026-01-02T01:05:00.000Z','2026-01-02T01:05:00.000Z','2026-01-02T01:30:00.000Z',3,'{"enrichmentSource":"heuristic","distinctSources":3}')`)
	exec(`INSERT INTO "Signal"
		("id","title","summary","whatHappened","whyItMatters","status","severity","confidence","eventType","country","region","firstSeenAt","lastSeenAt","publishedAt","createdAt","updatedAt","sourceCount","metadata")
		VALUES
		('sig_2','Markets rally on data','Stocks climbed.',NULL,NULL,'UNVERIFIED','MEDIUM',0.45,'ECONOMY.MARKETS',NULL,NULL,'2026-01-02T02:05:00.000Z','2026-01-02T02:10:00.000Z',NULL,'2026-01-02T02:05:00.000Z','2026-01-02T02:10:00.000Z',1,NULL)`)

	// A third signal with MULTIPLE tags and MULTIPLE sources, to stress relation
	// ordering parity.
	exec(`INSERT INTO "Signal"
		("id","title","summary","whatHappened","whyItMatters","status","severity","confidence","eventType","country","region","firstSeenAt","lastSeenAt","publishedAt","createdAt","updatedAt","sourceCount","metadata")
		VALUES
		('sig_3','Cyclone triggers floods','Storm and flooding.','Storm made landfall.','Evacuations underway.','DEVELOPING','CRITICAL',0.71,'DISASTER.CYCLONE','IN','AS','2026-01-02T00:05:00.000Z','2026-01-02T00:30:00.000Z','2026-01-02T00:05:00.000Z','2026-01-02T00:05:00.000Z','2026-01-02T00:30:00.000Z',2,'{"enrichmentSource":"heuristic","distinctSources":2}')`)
	exec(`INSERT INTO "Article" ("id","sourceId","canonicalUrl","title","body","summary","publishedAt","contentHash","tokenSet")
		VALUES ('art_3','src_alpha','https://alpha.example/c','Cyclone landfall','Cyclone made landfall causing floods.','Cyclone made landfall causing floods.','2026-01-02T00:00:00.000Z','hash3','causing cyclone floods landfall made')`)
	exec(`INSERT INTO "Article" ("id","sourceId","canonicalUrl","title","body","summary","publishedAt","contentHash","tokenSet")
		VALUES ('art_4','src_zeta','https://zeta.example/c','Floods spread','Flooding spread across the coast.','Flooding spread across the coast.','2026-01-02T00:10:00.000Z','hash4','across coast flooding spread')`)

	// Link articles to signals.
	exec(`INSERT INTO "SignalArticle" ("signalId","articleId","relationType","similarityScore","addedAt") VALUES ('sig_1','art_1','PRIMARY',1,'2026-01-02T01:05:00.000Z')`)
	exec(`INSERT INTO "SignalArticle" ("signalId","articleId","relationType","similarityScore","addedAt") VALUES ('sig_2','art_2','PRIMARY',1,'2026-01-02T02:05:00.000Z')`)
	exec(`INSERT INTO "SignalArticle" ("signalId","articleId","relationType","similarityScore","addedAt") VALUES ('sig_3','art_3','PRIMARY',1,'2026-01-02T00:05:00.000Z')`)
	exec(`INSERT INTO "SignalArticle" ("signalId","articleId","relationType","similarityScore","addedAt") VALUES ('sig_3','art_4','SUPPORTING',0.6,'2026-01-02T00:20:00.000Z')`)

	// Tags: resolve taxonomy tag ids by code.
	tagID := func(code string) string {
		var id string
		if err := d.Pool.QueryRow(ctx, `SELECT "id" FROM "TaxonomyTag" WHERE "code"=$1`, code).Scan(&id); err != nil {
			t.Fatalf("tag %s: %v", code, err)
		}
		return id
	}
	exec(`INSERT INTO "SignalTag" ("signalId","tagId","confidence") VALUES ('sig_1',$1,0.9)`, tagID("DISASTER.EARTHQUAKE"))
	exec(`INSERT INTO "SignalTag" ("signalId","tagId","confidence") VALUES ('sig_2',$1,0.6)`, tagID("ECONOMY.MARKETS"))
	// Insert sig_3's tags in a deliberately non-sorted order to expose ordering bugs.
	exec(`INSERT INTO "SignalTag" ("signalId","tagId","confidence") VALUES ('sig_3',$1,0.7)`, tagID("DISASTER.FLOOD"))
	exec(`INSERT INTO "SignalTag" ("signalId","tagId","confidence") VALUES ('sig_3',$1,0.8)`, tagID("DISASTER.CYCLONE"))

	// Deliveries: one SENT, one PENDING.
	exec(`INSERT INTO "DeliveryEvent" ("id","subscriptionId","signalId","channel","status","payload","attempts","deliveredAt","createdAt")
		VALUES ('del_1','sub_poll','sig_1','POLLING','SENT','{"event_id":"evt_sig_1_sub_poll"}',1,'2026-01-02T01:31:00.000Z','2026-01-02T01:31:00.000Z')`)
	exec(`INSERT INTO "DeliveryEvent" ("id","subscriptionId","signalId","channel","status","payload","attempts","createdAt")
		VALUES ('del_2','sub_hook','sig_1','WEBHOOK','PENDING','{"event_id":"evt_sig_1_sub_hook"}',0,'2026-01-02T01:31:02.000Z')`)
}

// seededTS boots the TS server and seeds the shared DB once for read-parity tests.
func seededTS(t *testing.T, port int) (*db.DB, *parity.Server, *parity.Server) {
	t.Helper()
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	dbtest.SeedTaxonomy(t, d)
	insertFixtures(t, d)

	ts, err := parity.StartTS(port, dbtest.URL())
	if err != nil {
		t.Fatalf("start TS: %v", err)
	}
	t.Cleanup(ts.Stop)
	return d, ts, goServer(t, d)
}
