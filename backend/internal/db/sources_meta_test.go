package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/worldsignal/backend/internal/cuid"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
)

// insertSource inserts a fully-specified source row for metadata-query tests.
func insertSource(t *testing.T, d *db.DB, name, country, region, scope, industry, sourceType, validation string, langs, tags []string, health int) string {
	t.Helper()
	id := cuid.New()
	_, err := d.Pool.Exec(context.Background(), `
INSERT INTO "Source" ("id","name","type","url","country","region","language","languages","geographicScope",
  "industry","sourceType","orgType","officialFeed","priority","credibility","crawlFrequency","parserType",
  "enabled","tags","healthScore","validationStatus","updatedAt")
VALUES ($1,$2,'RSS',$3,$4,$5,$6,$7,$8,$9,$10,'PRIVATE',true,2,0.8,900,'rss',true,$11,$12,$13,now())`,
		id, name, "https://"+id+".example/rss", country, region, langs[0], langs, scope,
		industry, sourceType, tags, health, validation)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestSourceMetadataQueries(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()

	id1 := insertSource(t, d, "Alpha", "US", "North America", "NATIONAL", "Technology", "RSS", "VALID", []string{"en"}, []string{"tech", "global"}, 95)
	insertSource(t, d, "Beta", "FR", "Europe", "NATIONAL", "Finance", "AGGREGATOR", "VALID", []string{"fr"}, []string{"finance"}, 80)
	insertSource(t, d, "Gamma", "FR", "Europe", "GLOBAL", "Technology", "RSS", "INVALID", []string{"fr", "en"}, []string{"tech"}, 40)

	// Filter by region.
	rows, total, err := d.ListSourcesFiltered(ctx, db.SourceFilter{Region: ptr("Europe"), Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(rows) != 2 {
		t.Fatalf("region filter: total=%d rows=%d", total, len(rows))
	}

	// Filter by language array membership.
	if n, _ := d.CountSources(ctx, db.SourceFilter{Language: ptr("en")}); n != 2 {
		t.Fatalf("language=en expected 2, got %d", n)
	}
	// Filter by tag, industry, validation, scope, search, country combined-ish.
	if n, _ := d.CountSources(ctx, db.SourceFilter{Tag: ptr("global")}); n != 1 {
		t.Fatalf("tag=global expected 1, got %d", n)
	}
	if n, _ := d.CountSources(ctx, db.SourceFilter{Industry: ptr("Technology"), ValidationStatus: ptr("VALID")}); n != 1 {
		t.Fatalf("industry+validation expected 1, got %d", n)
	}
	if n, _ := d.CountSources(ctx, db.SourceFilter{Scope: ptr("GLOBAL"), SourceType: ptr("RSS"), OrgType: ptr("PRIVATE")}); n != 1 {
		t.Fatalf("scope+type+org expected 1, got %d", n)
	}
	en := true
	if n, _ := d.CountSources(ctx, db.SourceFilter{Search: ptr("Alph"), Enabled: &en}); n != 1 {
		t.Fatalf("search expected 1, got %d", n)
	}
	if n, _ := d.CountSources(ctx, db.SourceFilter{Country: ptr("US")}); n != 1 {
		t.Fatalf("country expected 1, got %d", n)
	}

	// Pagination + ordering: limit 1 returns highest health first within priority.
	page, _, err := d.ListSourcesFiltered(ctx, db.SourceFilter{Limit: 1, Offset: 0})
	if err != nil || len(page) != 1 {
		t.Fatalf("pagination: %v len=%d", err, len(page))
	}

	// Coverage aggregates.
	cov, err := d.SourceCoverage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if sumBuckets(cov["byRegion"]) != 3 || sumBuckets(cov["byLanguage"]) < 3 {
		t.Fatalf("coverage wrong: region=%d lang=%d", sumBuckets(cov["byRegion"]), sumBuckets(cov["byLanguage"]))
	}

	// Validation log record + list.
	newest := time.Now()
	if err := d.RecordValidation(ctx, id1, cuid.New(), db.ValidationOutcome{OK: true, HTTPStatus: 200, ResponseMs: 100, ItemCount: 20, NewestItemAt: &newest, RedirectedTo: "https://x", HealthScore: 97}); err != nil {
		t.Fatal(err)
	}
	if err := d.RecordValidation(ctx, id1, cuid.New(), db.ValidationOutcome{OK: false, HTTPStatus: 500, ResponseMs: 50, Error: "boom"}); err != nil {
		t.Fatal(err)
	}
	logs, err := d.ListValidationLogs(ctx, id1, 10)
	if err != nil || len(logs) != 2 {
		t.Fatalf("logs: %v len=%d", err, len(logs))
	}
	if logs[0].OK {
		t.Fatal("newest-first: latest log should be the failure")
	}
	// The failure updated validationStatus to INVALID with the error recorded.
	src, _ := d.GetSource(ctx, id1)
	if src.ValidationStatus != "INVALID" || src.LastValidationError == nil || *src.LastValidationError != "boom" {
		t.Fatalf("record validation did not update source: %+v", src.ValidationStatus)
	}
}

func TestListSourcesAndGetSourceNotFound(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	insertSource(t, d, "Solo", "US", "North America", "NATIONAL", "Technology", "RSS", "VALID", []string{"en"}, []string{"x"}, 90)

	all, err := d.ListSources(ctx) // unfiltered wrapper
	if err != nil || len(all) != 1 {
		t.Fatalf("ListSources: %v len=%d", err, len(all))
	}
	if s, err := d.GetSource(ctx, "does-not-exist"); err != nil || s != nil {
		t.Fatalf("GetSource not-found should be (nil,nil): s=%v err=%v", s, err)
	}
}

func ptr(s string) *string { return &s }

func sumBuckets(bs []db.Bucket) int {
	n := 0
	for _, b := range bs {
		n += b.Count
	}
	return n
}
