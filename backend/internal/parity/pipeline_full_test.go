package parity_test

import (
	"context"
	"testing"
	"time"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
	"github.com/worldsignal/backend/internal/llm"
	"github.com/worldsignal/backend/internal/parity"
	"github.com/worldsignal/backend/internal/pipeline"
)

const (
	articleFullSnap      = `SELECT row_to_json(t) FROM (SELECT "sourceId","canonicalUrl","title","body","summary","contentHash","tokenSet" FROM "Article" ORDER BY "title") t`
	signalArticleRelSnap = `SELECT row_to_json(t) FROM (SELECT "relationType","similarityScore" FROM "SignalArticle" ORDER BY "relationType","similarityScore") t`
	deliveryFullSnap     = `SELECT row_to_json(t) FROM (SELECT "channel","status","attempts","payload" FROM "DeliveryEvent" ORDER BY "payload"->'data'->>'title') t`
	rawFullSnap          = `SELECT row_to_json(t) FROM (SELECT "sourceGuid","rawUrl","rawTitle","rawContent","status" FROM "RawItem" ORDER BY "sourceGuid") t`
)

func seedFullPipeline(t *testing.T, d *db.DB, feedURL string) {
	ex := mkExec(t, d)
	ex(`INSERT INTO "Source" ("id","name","url","updatedAt") VALUES ('src_p','Pipe',$1,now())`, feedURL)
	ex(`INSERT INTO "Subscriber" ("id","name","createdAt") VALUES ('__default__','Default Subscriber',now())`)
	ex(`INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config","createdAt") VALUES ('sub_p','__default__','All','POLLING','{}','{}',now())`)
}

func runGoPipeline(t *testing.T, d *db.DB) {
	ctx := context.Background()
	now := time.Now()
	gw := llm.NewOpenAIGateway("", "gpt-4o-mini") // disabled → heuristic
	rawIDs, err := pipeline.FetchSource(ctx, d, "src_p", now, 3, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	for _, rid := range rawIDs {
		aid, err := pipeline.NormalizeRawItem(ctx, d, rid)
		if err != nil {
			t.Fatal(err)
		}
		if aid == "" {
			continue
		}
		cl, err := pipeline.ClusterArticle(ctx, d, aid, now)
		if err != nil {
			t.Fatal(err)
		}
		if cl == nil {
			continue
		}
		if err := pipeline.EnrichSignal(ctx, d, gw, cl.SignalID, now); err != nil {
			t.Fatal(err)
		}
		dids, err := pipeline.MatchSubscriptions(ctx, d, cl.SignalID, now)
		if err != nil {
			t.Fatal(err)
		}
		for _, did := range dids {
			if err := pipeline.SendDelivery(ctx, d, nil, paritySecret, did, false, now); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestPipelineFullShadowRun(t *testing.T) {
	if testing.Short() {
		t.Skip("needs DB + node")
	}
	d := dbtest.Connect(t)
	url := feedServer(t)

	// TS full pipeline.
	dbtest.Reset(t, d)
	dbtest.SeedTaxonomy(t, d)
	seedFullPipeline(t, d, url)
	if _, err := parity.RunTSStage("pipeline", `{"sourceId":"src_p"}`, dbtest.URL(), paritySecret); err != nil {
		t.Fatal(err)
	}
	tsSnaps := map[string][]string{
		"raw":      snapshot(t, d, rawFullSnap),
		"article":  snapshot(t, d, articleFullSnap),
		"signal":   snapshot(t, d, signalSnap),
		"link":     snapshot(t, d, signalArticleRelSnap),
		"tag":      snapshot(t, d, signalTagSnap),
		"delivery": snapshot(t, d, deliveryFullSnap),
	}

	// Go full pipeline.
	dbtest.Reset(t, d)
	dbtest.SeedTaxonomy(t, d)
	seedFullPipeline(t, d, url)
	runGoPipeline(t, d)
	goSnaps := map[string][]string{
		"raw":      snapshot(t, d, rawFullSnap),
		"article":  snapshot(t, d, articleFullSnap),
		"signal":   snapshot(t, d, signalSnap),
		"link":     snapshot(t, d, signalArticleRelSnap),
		"tag":      snapshot(t, d, signalTagSnap),
		"delivery": snapshot(t, d, deliveryFullSnap),
	}

	for _, k := range []string{"raw", "article", "signal", "link", "tag", "delivery"} {
		eqSnapshots(t, "full:"+k, tsSnaps[k], goSnaps[k])
	}
}
