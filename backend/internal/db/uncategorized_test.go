package db_test

import (
	"context"
	"testing"

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
