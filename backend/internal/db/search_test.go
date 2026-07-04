package db_test

import (
	"context"
	"testing"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
)

func TestSignalFullTextSearch(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	ex := func(q string, a ...any) {
		if _, err := d.Pool.Exec(ctx, q, a...); err != nil {
			t.Fatalf("exec: %v", err)
		}
	}
	// 'a' matches "earthquake" in the title (weight A); 'b' only in the summary.
	ex(`INSERT INTO "Signal" ("id","title","summary","whatHappened","firstSeenAt","lastSeenAt","updatedAt")
	    VALUES ('a','Major earthquake strikes coastal city','A powerful quake caused damage','buildings collapsed', now(), now(), now())`)
	ex(`INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","updatedAt")
	    VALUES ('b','Economic growth report','Markets reacted to the earthquake of policy', now(), now(), now())`)
	ex(`INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","updatedAt")
	    VALUES ('c','Sports roundup','Local team wins the cup', now(), now(), now())`)

	search := func(q string) []string {
		s := q
		rows, err := d.ListSignals(ctx, db.SignalFilter{Search: &s})
		if err != nil {
			t.Fatalf("search %q: %v", q, err)
		}
		ids := make([]string, len(rows))
		for i, r := range rows {
			ids[i] = r.ID
		}
		return ids
	}

	got := search("earthquake")
	if len(got) != 2 {
		t.Fatalf("expected 2 matches for earthquake, got %v", got)
	}
	// Title match must rank above summary-only match.
	if got[0] != "a" || got[1] != "b" {
		t.Fatalf("expected ranked [a b], got %v", got)
	}
	// websearch phrase / multi-word AND semantics.
	if ids := search("coastal city"); len(ids) != 1 || ids[0] != "a" {
		t.Fatalf(`"coastal city" should match only a, got %v`, ids)
	}
	// Substring fallback: a partial word FTS wouldn't stem still matches via ILIKE.
	if ids := search("quak"); len(ids) == 0 {
		t.Fatal("partial 'quak' should match via the substring fallback")
	}
	// Blank/whitespace search is ignored (returns everything).
	blank := "   "
	if rows, err := d.ListSignals(ctx, db.SignalFilter{Search: &blank}); err != nil || len(rows) != 3 {
		t.Fatalf("blank search should not filter: %d %v", len(rows), err)
	}
	// Count honors the same predicate.
	q := "earthquake"
	if n, err := d.CountSignals(ctx, db.SignalFilter{Search: &q}); err != nil || n != 2 {
		t.Fatalf("count: %d %v", n, err)
	}
}

func TestArticleFullTextSearch(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	ex := func(q string, a ...any) {
		if _, err := d.Pool.Exec(ctx, q, a...); err != nil {
			t.Fatalf("exec: %v", err)
		}
	}
	ex(`INSERT INTO "Source" ("id","name","url","updatedAt") VALUES ('s','S','https://s.example',now())`)
	ex(`INSERT INTO "Article" ("id","sourceId","title","summary","body","fetchedAt") VALUES ('a1','s','Flood warning issued','','Heavy rain expected',now())`)
	ex(`INSERT INTO "Article" ("id","sourceId","title","summary","body","fetchedAt") VALUES ('a2','s','Weather note','','a passing flood of tourists',now())`)

	q := "flood"
	rows, total, err := d.ListArticles(ctx, db.ListFilter{Search: &q})
	if err != nil || total != 2 || len(rows) != 2 {
		t.Fatalf("article FTS: total=%d err=%v", total, err)
	}
	if rows[0].ID != "a1" {
		t.Fatalf("title match should rank first, got %s", rows[0].ID)
	}
}

func TestSearchEntities(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	ex := func(q string, a ...any) {
		if _, err := d.Pool.Exec(ctx, q, a...); err != nil {
			t.Fatalf("exec: %v", err)
		}
	}
	ex(`INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","updatedAt") VALUES ('s1','A','',now(),now(),now())`)
	ex(`INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","updatedAt") VALUES ('s2','B','',now(),now(),now())`)
	// "Red Cross" appears on two signals → highest count; typed entities.
	ex(`INSERT INTO "SignalAttribute" ("signalId","key","valueCode","valueText","confidence") VALUES
	    ('s1','entity','ORG','Red Cross',1),
	    ('s2','entity','ORG','Red Cross',1),
	    ('s1','entity','LOCATION','Coastal City',1),
	    ('s1','industry','','ENERGY',1)`)

	all, err := d.SearchEntities(ctx, db.EntityFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 distinct entities, got %d: %+v", len(all), all)
	}
	if all[0].Name != "Red Cross" || all[0].SignalCount != 2 || all[0].Type != "ORG" {
		t.Fatalf("top entity: %+v", all[0])
	}
	// Name search.
	hit, _ := d.SearchEntities(ctx, db.EntityFilter{Search: strPtr("coast")})
	if len(hit) != 1 || hit[0].Name != "Coastal City" {
		t.Fatalf("name search: %+v", hit)
	}
	// Type filter.
	orgs, _ := d.SearchEntities(ctx, db.EntityFilter{Type: strPtr("ORG")})
	if len(orgs) != 1 || orgs[0].Name != "Red Cross" {
		t.Fatalf("type filter: %+v", orgs)
	}

	// Signals filterable by entity name.
	name := "Red Cross"
	rows, err := d.ListSignals(ctx, db.SignalFilter{Entity: &name})
	if err != nil || len(rows) != 2 {
		t.Fatalf("entity filter on signals: %d %v", len(rows), err)
	}
}

func TestSearchEntitiesError(t *testing.T) {
	d := closed(t)
	if _, err := d.SearchEntities(context.Background(), db.EntityFilter{}); err == nil {
		t.Fatal("expected error on closed pool")
	}
}
