package parity_test

import (
	"context"
	"testing"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
	"github.com/worldsignal/backend/internal/parity"
	"github.com/worldsignal/backend/internal/pipeline"
)

const paritySecret = "parity-secret"

// snapshot returns normalized row_to_json rows for an ORDER-BY query.
func snapshot(t *testing.T, d *db.DB, query string) []string {
	t.Helper()
	rows, err := d.Pool.Query(context.Background(), query)
	if err != nil {
		t.Fatalf("snapshot query: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		out = append(out, normalizeJSON(t, raw))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func eqSnapshots(t *testing.T, name string, ts, go_ []string) {
	t.Helper()
	if len(ts) != len(go_) {
		t.Fatalf("%s: row count TS=%d Go=%d\nTS=%v\nGo=%v", name, len(ts), len(go_), ts, go_)
	}
	for i := range ts {
		if ts[i] != go_[i] {
			t.Fatalf("%s row %d differs:\nTS:  %s\nGo:  %s", name, i, ts[i], go_[i])
		}
	}
}

const articleSnap = `SELECT row_to_json(t) FROM (SELECT * FROM "Article" ORDER BY "rawItemId","title") t`
const rawItemSnap = `SELECT row_to_json(t) FROM (SELECT "id","sourceId","sourceGuid","rawUrl","rawTitle","rawContent","contentHash","status" FROM "RawItem" ORDER BY "id") t`

// seedNormalizeInput inserts a Source and PENDING RawItems for normalize tests.
func seedNormalizeInput(t *testing.T, d *db.DB) {
	t.Helper()
	ctx := context.Background()
	ex := func(sql string, args ...any) {
		if _, err := d.Pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	ex(`INSERT INTO "Source" ("id","name","url","updatedAt") VALUES ('src_n','Norm Source','https://norm.example/feed',now())`)
	// A normal item.
	ex(`INSERT INTO "RawItem" ("id","sourceId","sourceGuid","rawUrl","rawTitle","rawContent","status") VALUES
		('raw_1','src_n','g1','https://norm.example/a?utm_source=x','Quake Hits Region','<p>A strong earthquake struck the region today.</p>','PENDING')`)
	// An item with no title (should FAIL).
	ex(`INSERT INTO "RawItem" ("id","sourceId","sourceGuid","rawUrl","rawTitle","rawContent","status") VALUES
		('raw_2','src_n','g2','https://norm.example/b','','body only','PENDING')`)
	// A duplicate (same content as raw_1 after normalization).
	ex(`INSERT INTO "RawItem" ("id","sourceId","sourceGuid","rawUrl","rawTitle","rawContent","status") VALUES
		('raw_3','src_n','g3','https://other.example/c','Quake Hits Region','A strong earthquake struck the region today.','PENDING')`)
}

func TestPipelineNormalizeParity(t *testing.T) {
	if testing.Short() {
		t.Skip("needs DB + node")
	}
	d := dbtest.Connect(t)
	ctx := context.Background()

	runTS := func() ([]string, []string) {
		dbtest.Reset(t, d)
		seedNormalizeInput(t, d)
		for _, id := range []string{"raw_1", "raw_2", "raw_3"} {
			if _, err := parity.RunTSStage("normalize", `{"rawItemId":"`+id+`"}`, dbtest.URL(), paritySecret); err != nil {
				t.Fatal(err)
			}
		}
		return snapshot(t, d, articleSnap), snapshot(t, d, rawItemSnap)
	}
	runGo := func() ([]string, []string) {
		dbtest.Reset(t, d)
		seedNormalizeInput(t, d)
		for _, id := range []string{"raw_1", "raw_2", "raw_3"} {
			if _, err := pipeline.NormalizeRawItem(ctx, d, id); err != nil {
				t.Fatal(err)
			}
		}
		return snapshot(t, d, articleSnap), snapshot(t, d, rawItemSnap)
	}

	tsArt, tsRaw := runTS()
	goArt, goRaw := runGo()
	eqSnapshots(t, "articles", tsArt, goArt)
	eqSnapshots(t, "rawitems", tsRaw, goRaw)
}
