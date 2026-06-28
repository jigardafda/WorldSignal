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

const signalTagSnap = `SELECT row_to_json(t) FROM (SELECT tt."code", st."confidence" FROM "SignalTag" st JOIN "TaxonomyTag" tt ON tt."id"=st."tagId" ORDER BY tt."code") t`

func seedEnrichInput(t *testing.T, d *db.DB) {
	t.Helper()
	ex := mkExec(t, d)
	ex(`INSERT INTO "Source" ("id","name","url","credibility","updatedAt") VALUES ('src_e1','Primary Pub','https://e1.example/feed',0.9,now())`)
	ex(`INSERT INTO "Source" ("id","name","url","credibility","updatedAt") VALUES ('src_e2','Second Pub','https://e2.example/feed',0.5,now())`)
	ex(`INSERT INTO "Article" ("id","sourceId","canonicalUrl","title","body","summary","publishedAt","contentHash","tokenSet")
		VALUES ('art_e1','src_e1','https://e1.example/a','Cyclone triggers floods','Cyclone made landfall causing severe flooding and killed many people.','sum1','2026-01-02T00:00:00.000Z','he1','cyclone flooding killed landfall')`)
	ex(`INSERT INTO "Article" ("id","sourceId","canonicalUrl","title","body","summary","publishedAt","contentHash","tokenSet")
		VALUES ('art_e2','src_e2','https://e2.example/b','Floods spread along coast','Flooding spread across the coast.','sum2','2026-01-02T00:10:00.000Z','he2','coast flooding spread')`)
	ex(`INSERT INTO "Signal" ("id","title","summary","status","firstSeenAt","lastSeenAt","sourceCount","metadata","updatedAt")
		VALUES ('sig_e','Cyclone triggers floods','sum1','UNVERIFIED','2026-01-02T00:00:00.000Z','2026-01-02T00:10:00.000Z',2,'{"tokenSet":"cyclone flooding killed landfall"}',now())`)
	ex(`INSERT INTO "SignalArticle" ("signalId","articleId","relationType","similarityScore","addedAt") VALUES ('sig_e','art_e1','PRIMARY',1,'2026-01-02T00:00:00.000Z')`)
	ex(`INSERT INTO "SignalArticle" ("signalId","articleId","relationType","similarityScore","addedAt") VALUES ('sig_e','art_e2','SUPPORTING',0.6,'2026-01-02T00:10:00.000Z')`)
}

func TestPipelineEnrichParity(t *testing.T) {
	if testing.Short() {
		t.Skip("needs DB + node")
	}
	d := dbtest.Connect(t)
	ctx := context.Background()
	gw := llm.NewOpenAIGateway("", "gpt-4o-mini") // disabled → heuristic

	// TS
	dbtest.Reset(t, d)
	dbtest.SeedTaxonomy(t, d)
	seedEnrichInput(t, d)
	if _, err := parity.RunTSStage("enrich", `{"signalId":"sig_e"}`, dbtest.URL(), paritySecret); err != nil {
		t.Fatal(err)
	}
	tsSig := snapshot(t, d, signalSnap)
	tsTags := snapshot(t, d, signalTagSnap)

	// Go
	dbtest.Reset(t, d)
	dbtest.SeedTaxonomy(t, d)
	seedEnrichInput(t, d)
	if err := pipeline.EnrichSignal(ctx, d, gw, "sig_e", time.Now()); err != nil {
		t.Fatal(err)
	}
	goSig := snapshot(t, d, signalSnap)
	goTags := snapshot(t, d, signalTagSnap)

	eqSnapshots(t, "enrich signal", tsSig, goSig)
	eqSnapshots(t, "enrich tags", tsTags, goTags)
}
