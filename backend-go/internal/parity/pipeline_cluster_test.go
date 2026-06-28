package parity_test

import (
	"context"
	"testing"
	"time"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
	"github.com/worldsignal/backend/internal/parity"
	"github.com/worldsignal/backend/internal/pipeline"
)

// Snapshots exclude random ids/timestamps (normalized) by selecting only stable
// columns.
const signalSnap = `SELECT row_to_json(t) FROM (SELECT "title","summary","status","severity","confidence","eventType","country","region","sourceCount","metadata" FROM "Signal" ORDER BY "title") t`
const signalArticleSnap = `SELECT row_to_json(t) FROM (SELECT "articleId","relationType","similarityScore" FROM "SignalArticle" ORDER BY "articleId","relationType") t`

func seedClusterNew(t *testing.T, d *db.DB) {
	t.Helper()
	ex := mkExec(t, d)
	ex(`INSERT INTO "Source" ("id","name","url","updatedAt") VALUES ('src_c','C','https://c.example/feed',now())`)
	ex(`INSERT INTO "Article" ("id","sourceId","canonicalUrl","title","body","summary","publishedAt","contentHash","tokenSet")
		VALUES ('art_a','src_c','https://c.example/a','Quake hits Mindanao','A strong earthquake struck Mindanao region.','A strong earthquake struck Mindanao region.','2026-01-02T01:00:00.000Z','h_a','earthquake mindanao region strong struck')`)
}

func seedClusterJoin(t *testing.T, d *db.DB) {
	t.Helper()
	ex := mkExec(t, d)
	ex(`INSERT INTO "Source" ("id","name","url","updatedAt") VALUES ('src_c','C','https://c.example/feed',now())`)
	// Existing recent signal with a highly-overlapping token set (Jaccard >= 0.5).
	ex(`INSERT INTO "Signal" ("id","title","summary","status","firstSeenAt","lastSeenAt","sourceCount","metadata","updatedAt")
		VALUES ('sig_e','Existing quake','prev','UNVERIFIED',now(),now(),1,'{"tokenSet":"earthquake mindanao region struck"}',now())`)
	ex(`INSERT INTO "Article" ("id","sourceId","canonicalUrl","title","body","summary","publishedAt","contentHash","tokenSet")
		VALUES ('art_a','src_c','https://c.example/a','Quake hits Mindanao','A strong earthquake struck Mindanao region.','A strong earthquake struck Mindanao region.','2026-01-02T01:00:00.000Z','h_a','earthquake mindanao region strong struck')`)
}

func mkExec(t *testing.T, d *db.DB) func(string, ...any) {
	return func(sql string, args ...any) {
		if _, err := d.Pool.Exec(context.Background(), sql, args...); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
}

func TestPipelineClusterParity(t *testing.T) {
	if testing.Short() {
		t.Skip("needs DB + node")
	}
	d := dbtest.Connect(t)
	ctx := context.Background()

	scenarios := []struct {
		name string
		seed func(*testing.T, *db.DB)
	}{
		{"new_signal", seedClusterNew},
		{"join_existing", seedClusterJoin},
	}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			// TS run
			dbtest.Reset(t, d)
			sc.seed(t, d)
			if _, err := parity.RunTSStage("cluster", `{"articleId":"art_a"}`, dbtest.URL(), paritySecret); err != nil {
				t.Fatal(err)
			}
			tsSig := snapshot(t, d, signalSnap)
			tsLink := snapshot(t, d, signalArticleSnap)

			// Go run
			dbtest.Reset(t, d)
			sc.seed(t, d)
			if _, err := pipeline.ClusterArticle(ctx, d, "art_a", time.Now()); err != nil {
				t.Fatal(err)
			}
			goSig := snapshot(t, d, signalSnap)
			goLink := snapshot(t, d, signalArticleSnap)

			eqSnapshots(t, sc.name+" signals", tsSig, goSig)
			eqSnapshots(t, sc.name+" links", tsLink, goLink)
		})
	}
}
