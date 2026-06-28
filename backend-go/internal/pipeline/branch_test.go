package pipeline

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
	"github.com/worldsignal/backend/internal/llm"
)

func TestPureHelpers(t *testing.T) {
	if clamp01(-1) != 0 || clamp01(2) != 1 || clamp01(0.5) != 0.5 {
		t.Fatal("clamp01")
	}
	if runeLen(nil) != 0 {
		t.Fatal("runeLen nil")
	}
	s := "abc"
	if runeLen(&s) != 3 {
		t.Fatal("runeLen")
	}
	if derefStr(nil) != "" || derefStr(&s) != "abc" {
		t.Fatal("derefStr")
	}
	if !contains([]string{"a", "b"}, "b") || contains([]string{"a"}, "z") {
		t.Fatal("contains")
	}
	if !hasPrefix("DISASTER.X", "DISASTER.") || hasPrefix("X", "DISASTER.") {
		t.Fatal("hasPrefix")
	}
	if sliceRunes("héllo", 2) != "hé" || sliceRunes("ab", 5) != "ab" {
		t.Fatal("sliceRunes")
	}
	if nilIfEmpty("") != nil || *nilIfEmpty("x") != "x" {
		t.Fatal("nilIfEmpty")
	}
}

func TestPickRepresentativeLongestBodyWhenNoPrimary(t *testing.T) {
	short, long := "short", "a much longer body here"
	links := []db.EnrichLink{
		{RelationType: "SUPPORTING", Body: &short, Title: "s"},
		{RelationType: "SUPPORTING", Body: &long, Title: "l"},
	}
	if pickRepresentative(links).Title != "l" {
		t.Fatal("should pick longest body")
	}
}

func conn(t *testing.T) *db.DB {
	t.Helper()
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	return d
}

func closedDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Connect(context.Background(), dbtest.URL())
	if err != nil {
		t.Skip("no DB")
	}
	d.Close()
	return d
}

func TestFetchSourceEdges(t *testing.T) {
	d := conn(t)
	ctx := context.Background()
	now := time.Now()

	// Missing source → no-op.
	if ids, err := FetchSource(ctx, d, "nope", now); err != nil || ids != nil {
		t.Fatalf("missing source: %v %v", ids, err)
	}
	// Disabled source → no-op.
	mustExec(t, d, `INSERT INTO "Source" ("id","name","url","enabled","updatedAt") VALUES ('off','O','https://off.example',false,now())`)
	if ids, _ := FetchSource(ctx, d, "off", now); ids != nil {
		t.Fatal("disabled source should be skipped")
	}
	// Fetch failure → failureCount incremented, returns nil.
	mustExec(t, d, `INSERT INTO "Source" ("id","name","url","enabled","updatedAt") VALUES ('bad','B','http://127.0.0.1:0/',true,now())`)
	if ids, err := FetchSource(ctx, d, "bad", now); err != nil || ids != nil {
		t.Fatalf("fetch failure: %v %v", ids, err)
	}
	var fc int
	mustScan(t, d, `SELECT "failureCount" FROM "Source" WHERE "id"='bad'`, &fc)
	if fc != 1 {
		t.Fatalf("failureCount %d", fc)
	}
}

func TestNormalizeEdges(t *testing.T) {
	d := conn(t)
	ctx := context.Background()
	// Missing raw item → "".
	if id, err := NormalizeRawItem(ctx, d, "nope"); err != nil || id != "" {
		t.Fatalf("missing raw: %q %v", id, err)
	}
	// Already PARSED → returns linked article id.
	mustExec(t, d, `INSERT INTO "Source" ("id","name","url","updatedAt") VALUES ('s','S','https://s.example',now())`)
	mustExec(t, d, `INSERT INTO "RawItem" ("id","sourceId","rawTitle","rawContent","status") VALUES ('r','s','T','B','PARSED')`)
	mustExec(t, d, `INSERT INTO "Article" ("id","rawItemId","sourceId","title") VALUES ('art','r','s','T')`)
	if id, err := NormalizeRawItem(ctx, d, "r"); err != nil || id != "art" {
		t.Fatalf("parsed raw should return article: %q %v", id, err)
	}
	// NULL rawTitle → deref nil branch → empty title → marked FAILED.
	mustExec(t, d, `INSERT INTO "RawItem" ("id","sourceId","rawContent","status") VALUES ('rn','s','body','PENDING')`)
	if id, err := NormalizeRawItem(ctx, d, "rn"); err != nil || id != "" {
		t.Fatalf("null title should fail: %q %v", id, err)
	}
	var st string
	mustScan(t, d, `SELECT status FROM "RawItem" WHERE id='rn'`, &st)
	if st != "FAILED" {
		t.Fatalf("expected FAILED, got %s", st)
	}
}

func TestClusterEdges(t *testing.T) {
	d := conn(t)
	ctx := context.Background()
	now := time.Now()
	// Missing article → nil.
	if r, err := ClusterArticle(ctx, d, "nope", now); err != nil || r != nil {
		t.Fatalf("missing article: %v %v", r, err)
	}
	// Existing link → idempotent.
	mustExec(t, d, `INSERT INTO "Source" ("id","name","url","updatedAt") VALUES ('s','S','https://s.example',now())`)
	mustExec(t, d, `INSERT INTO "Article" ("id","sourceId","title","tokenSet") VALUES ('a','s','T','quake region')`)
	mustExec(t, d, `INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","updatedAt") VALUES ('sig','T','S',now(),now(),now())`)
	mustExec(t, d, `INSERT INTO "SignalArticle" ("signalId","articleId","relationType") VALUES ('sig','a','PRIMARY')`)
	r, err := ClusterArticle(ctx, d, "a", now)
	if err != nil || r == nil || r.SignalID != "sig" || r.IsNew {
		t.Fatalf("idempotent cluster: %+v %v", r, err)
	}
}

func TestEnrichSignalNoLinks(t *testing.T) {
	d := conn(t)
	ctx := context.Background()
	gw := llm.NewOpenAIGateway("", "")
	mustExec(t, d, `INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","updatedAt") VALUES ('s','T','S',now(),now(),now())`)
	if err := EnrichSignal(ctx, d, gw, "s", time.Now()); err != nil {
		t.Fatalf("enrich no links should be no-op: %v", err)
	}
}

func TestMatchAndSendEdges(t *testing.T) {
	d := conn(t)
	ctx := context.Background()
	now := time.Now()

	// Missing signal → nil result, no error.
	if ids, err := MatchSubscriptions(ctx, d, "nope", now); err != nil || ids != nil {
		t.Fatalf("missing signal match: %v %v", ids, err)
	}
	// Missing delivery → no-op.
	if err := SendDelivery(ctx, d, nil, "s", "nope", false, now); err != nil {
		t.Fatalf("missing delivery: %v", err)
	}
	// Already-SENT delivery → early return.
	mustExec(t, d, `INSERT INTO "Subscriber" ("id","name","createdAt") VALUES ('__default__','D',now())`)
	mustExec(t, d, `INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config","createdAt") VALUES ('sub','__default__','S','POLLING','{}','{}',now())`)
	mustExec(t, d, `INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","updatedAt") VALUES ('sg','T','S',now(),now(),now())`)
	mustExec(t, d, `INSERT INTO "DeliveryEvent" ("id","subscriptionId","signalId","channel","status","payload","createdAt") VALUES ('del','sub','sg','POLLING','SENT','{}',now())`)
	if err := SendDelivery(ctx, d, nil, "s", "del", false, now); err != nil {
		t.Fatalf("already-sent: %v", err)
	}
}

func TestFetchSourceDedupSkip(t *testing.T) {
	d := conn(t)
	ctx := context.Background()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>f</title>` +
			`<item><title>One</title><link>https://x/a</link><guid>g1</guid></item></channel></rss>`))
	}))
	defer srv.Close()
	mustExec(t, d, `INSERT INTO "Source" ("id","name","url","enabled","updatedAt") VALUES ('s','S',$1,true,now())`, srv.URL)

	first, err := FetchSource(ctx, d, "s", time.Now())
	if err != nil || len(first) != 1 {
		t.Fatalf("first fetch: %v %v", first, err)
	}
	// Second fetch: the guid already exists → dedup skip branch.
	second, err := FetchSource(ctx, d, "s", time.Now())
	if err != nil || len(second) != 0 {
		t.Fatalf("second fetch should dedup: %v %v", second, err)
	}
}

func TestClosedDBErrors(t *testing.T) {
	d := closedDB(t)
	ctx := context.Background()
	now := time.Now()
	gw := llm.NewOpenAIGateway("", "")
	if _, err := FetchSource(ctx, d, "x", now); err == nil {
		t.Fatal("FetchSource should error on closed DB")
	}
	if _, err := NormalizeRawItem(ctx, d, "x"); err == nil {
		t.Fatal("Normalize should error")
	}
	if _, err := ClusterArticle(ctx, d, "x", now); err == nil {
		t.Fatal("Cluster should error")
	}
	if err := EnrichSignal(ctx, d, gw, "x", now); err == nil {
		t.Fatal("Enrich should error")
	}
	if _, err := MatchSubscriptions(ctx, d, "x", now); err == nil {
		t.Fatal("Match should error")
	}
	if err := SendDelivery(ctx, d, nil, "s", "x", false, now); err == nil {
		t.Fatal("Send should error")
	}
}

func TestFetchSourceRawItemInsertError(t *testing.T) {
	d := conn(t)
	ctx := context.Background()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>f</title>` +
			`<item><title>T</title><link>https://x/a</link><guid>g</guid></item></channel></rss>`))
	}))
	defer srv.Close()
	mustExec(t, d, `INSERT INTO "Source" ("id","name","url","enabled","updatedAt") VALUES ('s','S',$1,true,now())`, srv.URL)
	// Hide RawItem so the existence check / insert fails after a successful fetch.
	if _, err := d.Pool.Exec(ctx, `ALTER TABLE "RawItem" RENAME TO "RawItem__h"`); err != nil {
		t.Fatal(err)
	}
	defer d.Pool.Exec(ctx, `ALTER TABLE "RawItem__h" RENAME TO "RawItem"`)
	if _, err := FetchSource(ctx, d, "s", time.Now()); err == nil {
		t.Fatal("expected error when RawItem table is hidden")
	}
}

func mustExec(t *testing.T, d *db.DB, sql string, args ...any) {
	t.Helper()
	if _, err := d.Pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec: %v", err)
	}
}

func mustScan(t *testing.T, d *db.DB, sql string, dest ...any) {
	t.Helper()
	if err := d.Pool.QueryRow(context.Background(), sql).Scan(dest...); err != nil {
		t.Fatalf("scan: %v", err)
	}
}
