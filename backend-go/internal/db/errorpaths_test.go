package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
)

// closed returns a DB whose pool is closed, so every query errors — exercising
// the error-return branches across the data layer.
func closed(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Connect(context.Background(), dbtest.URL())
	if err != nil {
		t.Skip("no DB")
	}
	d.Close()
	return d
}

func TestDBErrorPaths(t *testing.T) {
	d := closed(t)
	ctx := context.Background()
	now := time.Now()
	url := "https://x.example"

	mustErr := func(name string, err error) {
		if err == nil {
			t.Fatalf("%s: expected error on closed pool", name)
		}
	}

	_, err := d.ListSources(ctx)
	mustErr("ListSources", err)
	_, err = d.GetSource(ctx, "x")
	mustErr("GetSource", err)
	_, err = d.GetStats(ctx)
	mustErr("GetStats", err)
	_, err = d.ListSignals(ctx, db.SignalFilter{})
	mustErr("ListSignals", err)
	_, err = d.GetSignal(ctx, "x")
	mustErr("GetSignal", err)
	_, err = d.ListSubscriptions(ctx)
	mustErr("ListSubscriptions", err)
	_, err = d.ListSubscriptionsBasic(ctx)
	mustErr("ListSubscriptionsBasic", err)
	_, err = d.ListDeliveries(ctx, 10)
	mustErr("ListDeliveries", err)
	_, err = d.CreateSource(ctx, db.CreateSourceInput{Name: "n", URL: url})
	mustErr("CreateSource", err)
	_, err = d.SetSourceEnabled(ctx, "x", true)
	mustErr("SetSourceEnabled", err)
	_, err = d.UpdateSource(ctx, "x", db.SourcePatch{})
	mustErr("UpdateSource", err)
	mustErr("UpsertDefaultSubscriber", d.UpsertDefaultSubscriber(ctx))
	_, err = d.CreateSubscription(ctx, db.CreateSubscriptionInput{Name: "n"})
	mustErr("CreateSubscription", err)
	_, err = d.GetRawItem(ctx, "x")
	mustErr("GetRawItem", err)
	mustErr("SetRawItemStatus", d.SetRawItemStatus(ctx, "x", "PARSED"))
	_, err = d.ArticleIDByRawItem(ctx, "x")
	mustErr("ArticleIDByRawItem", err)
	_, err = d.FindDuplicateArticle(ctx, "h", &url)
	mustErr("FindDuplicateArticle", err)
	_, err = d.FindDuplicateArticle(ctx, "h", nil)
	mustErr("FindDuplicateArticle nil url", err)
	_, err = d.CreateArticle(ctx, db.NewArticle{})
	mustErr("CreateArticle", err)
	_, err = d.GetClusterArticle(ctx, "x")
	mustErr("GetClusterArticle", err)
	_, err = d.ExistingSignalForArticle(ctx, "x")
	mustErr("ExistingSignalForArticle", err)
	_, err = d.RecentSignalCandidates(ctx, now)
	mustErr("RecentSignalCandidates", err)
	mustErr("AttachArticleToSignal", d.AttachArticleToSignal(ctx, "s", "a", 0.5, now))
	_, err = d.CreateSignalFromArticle(ctx, &db.ClusterArticle{ID: "a"}, now)
	mustErr("CreateSignalFromArticle", err)
	_, err = d.LoadSignalForEnrich(ctx, "x")
	mustErr("LoadSignalForEnrich", err)
	_, err = d.TagIDsByCodes(ctx, []string{"A"})
	mustErr("TagIDsByCodes", err)
	mustErr("ApplyEnrichment", d.ApplyEnrichment(ctx, "x", db.EnrichmentUpdate{PublishedAt: now, Metadata: map[string]any{}}))
	_, err = d.LoadSignalForMatch(ctx, "x")
	mustErr("LoadSignalForMatch", err)
	_, err = d.EnabledSubscriptions(ctx)
	mustErr("EnabledSubscriptions", err)
	_, err = d.CreateDeliveryIfNew(ctx, "s", "sg", "POLLING", []byte("{}"))
	mustErr("CreateDeliveryIfNew", err)
	_, err = d.LoadDeliveryForSend(ctx, "x")
	mustErr("LoadDeliveryForSend", err)
	mustErr("IncrementDeliveryAttempts", d.IncrementDeliveryAttempts(ctx, "x"))
	mustErr("MarkDeliverySent", d.MarkDeliverySent(ctx, "x", now))
	mustErr("MarkDeliveryFailed", d.MarkDeliveryFailed(ctx, "x", "FAILED", now, "m"))
	_, err = d.GetSourceForFetch(ctx, "x")
	mustErr("GetSourceForFetch", err)
	_, err = d.RawItemExists(ctx, "s", "g")
	mustErr("RawItemExists", err)
	_, err = d.CreateRawItem(ctx, db.NewRawItem{SourceID: "s"})
	mustErr("CreateRawItem", err)
	mustErr("MarkSourceFetchSuccess", d.MarkSourceFetchSuccess(ctx, "x", now))
	mustErr("MarkSourceFetchFailure", d.MarkSourceFetchFailure(ctx, "x", now))
}

func TestTagIDsByCodesEmpty(t *testing.T) {
	d := dbtest.Connect(t)
	m, err := d.TagIDsByCodes(context.Background(), nil)
	if err != nil || len(m) != 0 {
		t.Fatalf("empty codes: %v %v", m, err)
	}
}

func TestGetClusterArticleNullTokenSet(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	if _, err := d.Pool.Exec(ctx, `INSERT INTO "Source" ("id","name","url","updatedAt") VALUES ('s','S','https://s.example',now())`); err != nil {
		t.Fatal(err)
	}
	// tokenSet NULL → deref's nil branch.
	if _, err := d.Pool.Exec(ctx, `INSERT INTO "Article" ("id","sourceId","title") VALUES ('a','s','T')`); err != nil {
		t.Fatal(err)
	}
	a, err := d.GetClusterArticle(ctx, "a")
	if err != nil || a == nil || a.TokenSet != "" {
		t.Fatalf("null tokenSet: %+v %v", a, err)
	}
}

func TestCreateDeliveryConflict(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	ex := func(q string, a ...any) {
		if _, err := d.Pool.Exec(ctx, q, a...); err != nil {
			t.Fatal(err)
		}
	}
	ex(`INSERT INTO "Subscriber" ("id","name","createdAt") VALUES ('__default__','D',now())`)
	ex(`INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config","createdAt") VALUES ('sub','__default__','S','POLLING','{}','{}',now())`)
	ex(`INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","updatedAt") VALUES ('sg','T','S',now(),now(),now())`)

	id1, err := d.CreateDeliveryIfNew(ctx, "sub", "sg", "POLLING", []byte(`{}`))
	if err != nil || id1 == "" {
		t.Fatalf("first create: %q %v", id1, err)
	}
	id2, err := d.CreateDeliveryIfNew(ctx, "sub", "sg", "POLLING", []byte(`{}`))
	if err != nil || id2 != "" {
		t.Fatalf("duplicate should be skipped: %q %v", id2, err)
	}
}

func TestApplyEnrichmentMetaMarshalError(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	err := d.ApplyEnrichment(context.Background(), "sg", db.EnrichmentUpdate{
		PublishedAt: time.Now(),
		Metadata:    map[string]any{"bad": make(chan int)}, // unmarshalable
	})
	if err == nil {
		t.Fatal("expected metadata marshal error")
	}
}

func TestCreateRawItemConflict(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	if _, err := d.Pool.Exec(ctx, `INSERT INTO "Source" ("id","name","url","updatedAt") VALUES ('s','S','https://s.example',now())`); err != nil {
		t.Fatal(err)
	}
	g := "guid-1"
	in := db.NewRawItem{SourceID: "s", SourceGuid: &g, RawTitle: "T", RawContent: "B"}
	id1, err := d.CreateRawItem(ctx, in)
	if err != nil || id1 == "" {
		t.Fatalf("first insert: %q %v", id1, err)
	}
	id2, err := d.CreateRawItem(ctx, in) // same (sourceId, sourceGuid)
	if err != nil || id2 != "" {
		t.Fatalf("duplicate raw item should be skipped: %q %v", id2, err)
	}
}

func TestArticleIDByRawItemMissing(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	id, err := d.ArticleIDByRawItem(context.Background(), "nope")
	if err != nil || id != "" {
		t.Fatalf("missing raw → empty: %q %v", id, err)
	}
}

func TestConnectBadURL(t *testing.T) {
	if _, err := db.Connect(context.Background(), "postg://bad::url"); err == nil {
		t.Fatal("expected connect error for bad URL")
	}
}

func TestConnectPingFailure(t *testing.T) {
	// Valid DSN syntax but an unreachable port → pool opens lazily, Ping fails.
	if _, err := db.Connect(context.Background(), "postgres://u:p@127.0.0.1:1/db?connect_timeout=1"); err == nil {
		t.Fatal("expected ping failure for unreachable server")
	}
}
