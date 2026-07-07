package db_test

import (
	"context"
	"testing"

	"github.com/worldsignal/backend/internal/dbtest"
)

// TestRelevanceFeed exercises the Phase-1 backfill/feed DB layer end to end
// against the test database: candidate loading, interest persistence, ranked feed
// ordering, and feedback logging.
func TestRelevanceFeed(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		if _, err := d.Pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("exec %q: %v", sql, err)
		}
	}

	// A subscriber + subscription (the profile) and two enriched signals: one that
	// matches a DISASTER interest, one that does not.
	exec(`INSERT INTO "Subscriber" ("id","name","status","createdAt") VALUES ('sub1','Acme','ACTIVE',now())`)
	exec(`INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config","enabled","createdAt")
	       VALUES ('p1','sub1','For You','WEBHOOK','{}','{}',true,now())`)

	exec(`INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","eventType","severity","influence","relevance","confidence","sourceCount","metadata","updatedAt")
	       VALUES ('quake','Big earthquake','A quake struck.',now(),now(),'DISASTER.EARTHQUAKE','HIGH','HIGH',0.8,0.9,1,'{}',now())`)
	exec(`INSERT INTO "SignalAttribute" ("signalId","key","valueCode","valueText","confidence") VALUES ('quake','category','DISASTER.EARTHQUAKE','',0.9)`)
	exec(`INSERT INTO "SignalAttribute" ("signalId","key","valueCode","valueText","confidence") VALUES ('quake','entity','','NDMA',0.8)`)

	exec(`INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","eventType","severity","influence","relevance","confidence","sourceCount","metadata","updatedAt")
	       VALUES ('match','Cup final result','Team wins.',now(),now(),'SPORTS.RESULT','LOW','LOW',0.3,0.5,1,'{}',now())`)
	exec(`INSERT INTO "SignalAttribute" ("signalId","key","valueCode","valueText","confidence") VALUES ('match','category','SPORTS.RESULT','',0.9)`)

	// Candidate loading pulls both with their tags/entities.
	cands, err := d.CandidateSignals(ctx, 72, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 2 {
		t.Fatalf("want 2 candidates, got %d", len(cands))
	}

	// Set a DISASTER interest, then the feed must rank the quake first.
	if err := d.SetSubscriptionInterests(ctx, "p1", map[string]float64{"tag:DISASTER": 5}); err != nil {
		t.Fatal(err)
	}
	profile, err := d.LoadProfile(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if profile.Interests["tag:DISASTER"] != 5 {
		t.Fatalf("interest not persisted: %+v", profile.Interests)
	}

	feed, err := d.RankedFeed(ctx, "p1", 72, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(feed) == 0 || feed[0].ID != "quake" {
		t.Fatalf("expected quake ranked first for a DISASTER profile, got %+v", feed)
	}
	if len(feed[0].Reasons) == 0 {
		t.Fatal("top result should carry a reason")
	}

	// Feedback is recorded and idempotent.
	if err := d.RecordFeedback(ctx, "p1", "quake", "UP"); err != nil {
		t.Fatal(err)
	}
	if err := d.RecordFeedback(ctx, "p1", "quake", "UP"); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM "SignalFeedback" WHERE "subscriptionId"='p1'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("feedback should upsert to 1 row, got %d", n)
	}
}

// TestRelevanceFeedErrorPaths covers the query/tx error branches of the feed DB
// layer via a canceled context, deterministically.
func TestRelevanceFeedErrorPaths(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	cctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := d.CandidateSignals(cctx, 72, 10); err == nil {
		t.Error("CandidateSignals should error on a canceled context")
	}
	if _, err := d.LoadProfile(cctx, "x"); err == nil {
		t.Error("LoadProfile should error on a canceled context")
	}
	if err := d.SetSubscriptionInterests(cctx, "x", map[string]float64{"tag:DISASTER": 1}); err == nil {
		t.Error("SetSubscriptionInterests should error on a canceled context")
	}
	if _, err := d.RankedFeed(cctx, "x", 72, 10); err == nil {
		t.Error("RankedFeed should error on a canceled context")
	}
	// SetSubscriptionInterests coerces nil to an empty map (no error shape change).
	dbtest.Reset(t, d)
	if _, err := d.Pool.Exec(context.Background(), `INSERT INTO "Subscriber" ("id","name","status","createdAt") VALUES ('s0','n','ACTIVE',now())`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(context.Background(), `INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config","enabled","createdAt") VALUES ('p0','s0','n','WEBHOOK','{}','{}',true,now())`); err != nil {
		t.Fatal(err)
	}
	if err := d.SetSubscriptionInterests(context.Background(), "p0", nil); err != nil {
		t.Fatalf("nil interests should be accepted: %v", err)
	}
}

func TestRankedFeedLargeLimit(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	if _, err := d.Pool.Exec(ctx, `INSERT INTO "Subscriber" ("id","name","status","createdAt") VALUES ('sl','n','ACTIVE',now())`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(ctx, `INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config","enabled","createdAt") VALUES ('pl','sl','n','WEBHOOK','{}','{}',true,now())`); err != nil {
		t.Fatal(err)
	}
	// A large limit exercises the maxCandidates upper clamp (>2000 -> 2000).
	if _, err := d.RankedFeed(ctx, "pl", 72, 500); err != nil {
		t.Fatalf("RankedFeed large limit: %v", err)
	}
}

func TestCandidateDefaultsAndProfileKeyword(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	// Default and clamped limit/sinceHours branches.
	if _, err := d.CandidateSignals(ctx, 0, 0); err != nil {
		t.Fatalf("default params: %v", err)
	}
	if _, err := d.CandidateSignals(ctx, 72, 5000); err != nil {
		t.Fatalf("clamped limit: %v", err)
	}
	// LoadProfile lifts the filter's keyword into the profile keywords.
	if _, err := d.Pool.Exec(ctx, `INSERT INTO "Subscriber" ("id","name","status","createdAt") VALUES ('sk','n','ACTIVE',now())`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(ctx, `INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config","enabled","createdAt") VALUES ('pk','sk','n','WEBHOOK','{"keyword":"nike"}','{}',true,now())`); err != nil {
		t.Fatal(err)
	}
	p, err := d.LoadProfile(ctx, "pk")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Keywords) != 1 || p.Keywords[0] != "nike" {
		t.Fatalf("filter keyword should become a profile keyword: %+v", p.Keywords)
	}
}
