package db_test

import (
	"context"
	"testing"

	"github.com/worldsignal/backend/internal/cuid"
	"github.com/worldsignal/backend/internal/dbtest"
)

func TestListAndGetSource(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()

	// Insert two sources out of priority order.
	id1, id2 := cuid.New(), cuid.New()
	_, err := d.Pool.Exec(ctx,
		`INSERT INTO "Source" ("id","name","url","priority","updatedAt") VALUES ($1,'Zeta','https://z.example/feed',2,now()),($2,'Alpha','https://a.example/feed',1,now())`,
		id1, id2)
	if err != nil {
		t.Fatal(err)
	}

	list, err := d.ListSources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2 sources, got %d", len(list))
	}
	// Ordered priority asc → Alpha (1) before Zeta (2).
	if list[0].Name != "Alpha" || list[1].Name != "Zeta" {
		t.Fatalf("ordering wrong: %s, %s", list[0].Name, list[1].Name)
	}
	// Defaults applied by the DB.
	if list[0].Type != "RSS" || !list[0].Enabled || list[0].Credibility != 0.5 {
		t.Fatalf("defaults wrong: %+v", list[0])
	}
	if list[0].Language == nil || *list[0].Language != "en" {
		t.Fatal("language default should be en")
	}

	got, err := d.GetSource(ctx, id2)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Name != "Alpha" {
		t.Fatalf("GetSource returned %+v", got)
	}

	missing, err := d.GetSource(ctx, "nope")
	if err != nil {
		t.Fatal(err)
	}
	if missing != nil {
		t.Fatal("expected nil for missing source")
	}
}

func TestSeedTaxonomy(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	dbtest.SeedTaxonomy(t, d)

	var count int
	if err := d.Pool.QueryRow(context.Background(), `SELECT count(*) FROM "TaxonomyTag"`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count < 30 {
		t.Fatalf("expected full taxonomy seeded, got %d tags", count)
	}
}
