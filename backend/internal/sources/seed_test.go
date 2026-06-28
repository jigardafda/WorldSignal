package sources

import (
	"context"
	"testing"
	"time"

	"github.com/worldsignal/backend/internal/dbtest"
)

func TestSeedValid(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()

	newest := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	results := []Result{
		{OK: true, HTTPStatus: 200, ResponseMs: 120, ItemCount: 30, NewestItem: &newest, HealthScore: 98,
			Candidate: Candidate{
				Name: "Feed One", FeedURL: "https://one.example/rss", WebsiteURL: "https://one.example",
				Country: "US", Region: "North America", GeographicScope: "NATIONAL", Languages: []string{"en"},
				Category: "General", Industry: "Technology", Publisher: "One", OrgType: "PRIVATE", SourceType: "RSS",
				OfficialFeed: true, Priority: 2, Credibility: 0.8, Tags: []string{"tech", "global"},
				Keywords: []string{"k1"}, Metadata: map[string]any{"discoverySource": "curated"},
			}},
		{OK: true, HTTPStatus: 200, ResponseMs: 80, ItemCount: 10, HealthScore: 90, RedirectedTo: "https://two.example/atom2",
			Candidate: Candidate{Name: "Feed Two", FeedURL: "https://two.example/atom", SourceType: "ATOM", Languages: []string{"fr"}, Tags: []string{}}},
		{OK: false, Error: "http 404", Candidate: Candidate{Name: "Dead", FeedURL: "https://dead.example/rss"}},
	}

	sum, err := SeedValid(ctx, d, results)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Inserted != 2 || sum.Skipped != 1 || sum.Logs != 2 {
		t.Fatalf("seed summary unexpected: %+v", sum)
	}

	// Verify persisted metadata for the first feed.
	var (
		validation, sourceType string
		health                 int
		langs, tags            []string
		official               bool
	)
	err = d.Pool.QueryRow(ctx, `SELECT "validationStatus","sourceType","healthScore","languages","tags","officialFeed" FROM "Source" WHERE "url"=$1`, "https://one.example/rss").
		Scan(&validation, &sourceType, &health, &langs, &tags, &official)
	if err != nil {
		t.Fatal(err)
	}
	if validation != "VALID" || sourceType != "RSS" || health != 98 || len(langs) != 1 || langs[0] != "en" || len(tags) != 2 || !official {
		t.Fatalf("persisted fields wrong: status=%s type=%s health=%d langs=%v tags=%v official=%v", validation, sourceType, health, langs, tags, official)
	}

	// ATOM type maps to legacy "ATOM".
	var legacyType string
	if err := d.Pool.QueryRow(ctx, `SELECT "type" FROM "Source" WHERE "url"=$1`, "https://two.example/atom").Scan(&legacyType); err != nil {
		t.Fatal(err)
	}
	if legacyType != "ATOM" {
		t.Fatalf("expected ATOM legacy type, got %s", legacyType)
	}

	// Re-seeding the same results updates rather than inserts.
	sum2, err := SeedValid(ctx, d, results)
	if err != nil {
		t.Fatal(err)
	}
	if sum2.Updated != 2 || sum2.Inserted != 0 {
		t.Fatalf("re-seed should update: %+v", sum2)
	}
}

func TestNullStr(t *testing.T) {
	if nullStr("") != nil {
		t.Fatal("empty should be nil")
	}
	if nullStr("x") != "x" {
		t.Fatal("non-empty should pass through")
	}
}
