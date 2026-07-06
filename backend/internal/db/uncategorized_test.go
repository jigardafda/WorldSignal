package db_test

import (
	"context"
	"testing"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
)

// TestUncategorizedSignalIDs verifies the backfill query selects exactly the
// signals whose primary category is missing or in the GENERAL domain, newest
// first, and excludes properly categorized ones.
func TestUncategorizedSignalIDs(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()

	exec := func(sql string, args ...any) {
		if _, err := d.Pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("exec %q: %v", sql, err)
		}
	}
	ins := func(id, eventType string, secondsAgo int) {
		if eventType == "" {
			exec(`INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","sourceCount","metadata","updatedAt")
			       VALUES ($1,'t','s',now(),now() - make_interval(secs => $2),1,'{}',now())`, id, secondsAgo)
			return
		}
		exec(`INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","eventType","sourceCount","metadata","updatedAt")
		       VALUES ($1,'t','s',now(),now() - make_interval(secs => $2),$3,1,'{}',now())`, id, secondsAgo, eventType)
	}

	ins("null_cat", "", 10)                    // NULL eventType → uncategorized
	ins("general", "GENERAL.OTHER", 30)        // GENERAL domain → uncategorized
	ins("disaster", "DISASTER.EARTHQUAKE", 20) // categorized → excluded
	ins("politics", "POLITICS.ELECTIONS", 5)   // categorized → excluded

	ids, err := d.UncategorizedSignalIDs(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Only the two uncategorized signals, ordered by lastSeenAt DESC
	// (null_cat is 10s ago, general is 30s ago → null_cat first).
	want := []string{"null_cat", "general"}
	if len(ids) != len(want) {
		t.Fatalf("got %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("order/content mismatch: got %v, want %v", ids, want)
		}
	}

	// limit is honored.
	limited, err := d.UncategorizedSignalIDs(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(limited) != 1 || limited[0] != "null_cat" {
		t.Fatalf("limit=1 got %v, want [null_cat]", limited)
	}
}

// TestUncategorizedSignalTexts checks the backfill input query returns the
// GENERAL/uncategorized signals with their title, summary and the longest linked
// article body, newest first.
func TestUncategorizedSignalTexts(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		if _, err := d.Pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("exec %q: %v", sql, err)
		}
	}
	exec(`INSERT INTO "Source" ("id","name","url","credibility","updatedAt") VALUES ('s','S','https://s.example',0.8,now())`)
	// A GENERAL signal with two linked articles — the longer body must win.
	exec(`INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","eventType","sourceCount","metadata","updatedAt") VALUES ('g','Quake hits','short sum',now(),now()-make_interval(secs=>10),'GENERAL.OTHER',1,'{}',now())`)
	exec(`INSERT INTO "Article" ("id","sourceId","title","body") VALUES ('a1','s','t','tiny')`)
	exec(`INSERT INTO "Article" ("id","sourceId","title","body") VALUES ('a2','s','t','a much longer article body with detail')`)
	exec(`INSERT INTO "SignalArticle" ("signalId","articleId","relationType","addedAt") VALUES ('g','a1','PRIMARY',now())`)
	exec(`INSERT INTO "SignalArticle" ("signalId","articleId","relationType","addedAt") VALUES ('g','a2','SUPPORTING',now())`)
	// A categorized signal — must be excluded.
	exec(`INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","eventType","sourceCount","metadata","updatedAt") VALUES ('c','x','y',now(),now(),'DISASTER.FLOOD',1,'{}',now())`)

	got, err := d.UncategorizedSignalTexts(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "g" {
		t.Fatalf("want [g], got %+v", got)
	}
	if got[0].Title != "Quake hits" || got[0].Summary != "short sum" {
		t.Fatalf("title/summary wrong: %+v", got[0])
	}
	if got[0].Body != "a much longer article body with detail" {
		t.Fatalf("expected the longest article body, got %q", got[0].Body)
	}

	// limit > 0 branch is honored.
	limited, err := d.UncategorizedSignalTexts(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(limited) != 1 || limited[0].ID != "g" {
		t.Fatalf("limit=1 got %+v", limited)
	}
}

// TestSetSignalCategory checks the in-place recategorization: eventType is set to
// the primary tag, category attribute rows are replaced, and non-category
// attributes are left untouched.
func TestSetSignalCategory(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		if _, err := d.Pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("exec %q: %v", sql, err)
		}
	}
	exec(`INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","eventType","sourceCount","metadata","updatedAt") VALUES ('g','t','s',now(),now(),'GENERAL.OTHER',1,'{}',now())`)
	// Existing rows: a stale category (to be replaced) and an industry (to be kept).
	exec(`INSERT INTO "SignalAttribute" ("signalId","key","valueCode","valueText","valueNum","confidence") VALUES ('g','category','GENERAL.OTHER','',NULL,0.3)`)
	exec(`INSERT INTO "SignalAttribute" ("signalId","key","valueCode","valueText","valueNum","confidence") VALUES ('g','industry','BANKING','',NULL,1)`)

	err := d.SetSignalCategory(ctx, "g", []db.CategoryTag{
		{Code: "TRANSPORT.RAIL", Confidence: 0.9},
		{Code: "TRANSPORT.OTHER", Confidence: 0.5},
	})
	if err != nil {
		t.Fatal(err)
	}

	var eventType string
	if err := d.Pool.QueryRow(ctx, `SELECT "eventType" FROM "Signal" WHERE id='g'`).Scan(&eventType); err != nil {
		t.Fatal(err)
	}
	if eventType != "TRANSPORT.RAIL" {
		t.Fatalf("eventType = %q, want TRANSPORT.RAIL", eventType)
	}

	attrs, err := d.SignalAttributes(ctx, "g")
	if err != nil {
		t.Fatal(err)
	}
	var cats []string
	industryKept := false
	for _, a := range attrs {
		if a.Key == "category" {
			cats = append(cats, a.ValueCode)
		}
		if a.Key == "industry" && a.ValueCode == "BANKING" {
			industryKept = true
		}
	}
	if len(cats) != 2 {
		t.Fatalf("want 2 category rows, got %v", cats)
	}
	if !industryKept {
		t.Fatal("non-category attribute (industry=BANKING) should be preserved")
	}

	// No-op on empty tags.
	if err := d.SetSignalCategory(ctx, "g", nil); err != nil {
		t.Fatalf("empty tags should be a no-op, got %v", err)
	}
}
