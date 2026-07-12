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
	if ids, err := FetchSource(ctx, d, "nope", now, 3, time.Hour); err != nil || ids != nil {
		t.Fatalf("missing source: %v %v", ids, err)
	}
	// Disabled source → no-op.
	mustExec(t, d, `INSERT INTO "Source" ("id","name","url","enabled","updatedAt") VALUES ('off','O','https://off.example',false,now())`)
	if ids, _ := FetchSource(ctx, d, "off", now, 3, time.Hour); ids != nil {
		t.Fatal("disabled source should be skipped")
	}
	// Fetch failure → failureCount incremented, returns nil.
	mustExec(t, d, `INSERT INTO "Source" ("id","name","url","enabled","updatedAt") VALUES ('bad','B','http://127.0.0.1:0/',true,now())`)
	if ids, err := FetchSource(ctx, d, "bad", now, 3, time.Hour); err != nil || ids != nil {
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
	// Title but NULL url and empty body → canonical nil + nil summary/body branches.
	mustExec(t, d, `INSERT INTO "RawItem" ("id","sourceId","rawTitle","rawContent","status") VALUES ('rm','s','Headline only','','PENDING')`)
	aid, err := NormalizeRawItem(ctx, d, "rm")
	if err != nil || aid == "" {
		t.Fatalf("minimal article: %q %v", aid, err)
	}
	var body, summary, canonical *string
	mustScan(t, d, `SELECT body, summary, "canonicalUrl" FROM "Article" WHERE "rawItemId"='rm'`, &body, &summary, &canonical)
	if body != nil || summary != nil || canonical != nil {
		t.Fatalf("expected nil body/summary/canonical, got %v %v %v", body, summary, canonical)
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

func TestClusterNewSignalMinimalArticle(t *testing.T) {
	d := conn(t)
	ctx := context.Background()
	// Article with no summary and no publishedAt → CreateSignalFromArticle uses
	// the title as summary and `now` as firstSeen.
	mustExec(t, d, `INSERT INTO "Source" ("id","name","url","updatedAt") VALUES ('s','S','https://s.example',now())`)
	mustExec(t, d, `INSERT INTO "Article" ("id","sourceId","title","tokenSet") VALUES ('a','s','Lone headline','lone headline tokens')`)
	res, err := ClusterArticle(ctx, d, "a", time.Now())
	if err != nil || res == nil || !res.IsNew {
		t.Fatalf("expected new signal: %+v %v", res, err)
	}
	var summary string
	mustScan(t, d, `SELECT summary FROM "Signal" LIMIT 1`, &summary)
	if summary != "Lone headline" {
		t.Fatalf("summary should default to title, got %q", summary)
	}
}

func TestClusterJoinsExistingSignal(t *testing.T) {
	d := conn(t)
	ctx := context.Background()
	now := time.Now()
	mustExec(t, d, `INSERT INTO "Source" ("id","name","url","updatedAt") VALUES ('s','S','https://s.example',now())`)
	mustExec(t, d, `INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","sourceCount","metadata","updatedAt") VALUES ('sig','T','S',now(),now(),1,'{"tokenSet":"earthquake mindanao region struck"}',now())`)
	mustExec(t, d, `INSERT INTO "Article" ("id","sourceId","title","tokenSet") VALUES ('a','s','T','earthquake mindanao region strong struck')`)

	res, err := ClusterArticle(ctx, d, "a", now)
	if err != nil || res == nil || res.IsNew || res.SignalID != "sig" {
		t.Fatalf("expected join to existing signal: %+v %v", res, err)
	}
	var count int
	mustScan(t, d, `SELECT "sourceCount" FROM "Signal" WHERE id='sig'`, &count)
	if count != 2 {
		t.Fatalf("sourceCount should increment to 2, got %d", count)
	}
	var rel string
	mustScan(t, d, `SELECT "relationType" FROM "SignalArticle" WHERE "signalId"='sig' AND "articleId"='a'`, &rel)
	if rel != "SUPPORTING" {
		t.Fatalf("expected SUPPORTING link, got %s", rel)
	}
}

func TestEnrichSignalNoLinks(t *testing.T) {
	d := conn(t)
	ctx := context.Background()
	gw := llm.NewOpenAIGateway("", "")
	mustExec(t, d, `INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","updatedAt") VALUES ('s','T','S',now(),now(),now())`)
	if err := EnrichSignal(ctx, d, gw, nil, "s", time.Now()); err != nil {
		t.Fatalf("enrich no links should be no-op: %v", err)
	}
}

func TestEnrichSignalSuccess(t *testing.T) {
	d := conn(t)
	ctx := context.Background()
	dbtest.SeedTaxonomy(t, d)
	gw := llm.NewOpenAIGateway("", "") // heuristic
	mustExec(t, d, `INSERT INTO "Source" ("id","name","url","credibility","updatedAt") VALUES ('s1','Primary','https://s1.example',0.9,now())`)
	mustExec(t, d, `INSERT INTO "Source" ("id","name","url","credibility","updatedAt") VALUES ('s2','Second','https://s2.example',0.5,now())`)
	mustExec(t, d, `INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","sourceCount","metadata","updatedAt") VALUES ('sg','old','old',now(),now(),2,'{"tokenSet":"x"}',now())`)
	// PRIMARY article has NULL body → enrich falls back to its summary.
	mustExec(t, d, `INSERT INTO "Article" ("id","sourceId","title","summary","publishedAt") VALUES ('a1','s1','Quake','A major earthquake struck the coast.',now())`)
	mustExec(t, d, `INSERT INTO "Article" ("id","sourceId","title","body") VALUES ('a2','s2','Quake follow','More on the earthquake.')`)
	mustExec(t, d, `INSERT INTO "SignalArticle" ("signalId","articleId","relationType","addedAt") VALUES ('sg','a1','PRIMARY',now())`)
	mustExec(t, d, `INSERT INTO "SignalArticle" ("signalId","articleId","relationType","addedAt") VALUES ('sg','a2','SUPPORTING',now())`)

	if err := EnrichSignal(ctx, d, gw, nil, "sg", time.Now()); err != nil {
		t.Fatal(err)
	}
	var status, eventType string
	mustScan(t, d, `SELECT status, "eventType" FROM "Signal" WHERE id='sg'`, &status, &eventType)
	if status != "DEVELOPING" { // 2 distinct sources
		t.Fatalf("status %s", status)
	}
	if eventType == "" {
		t.Fatal("eventType should be set from top tag")
	}
	var tagCount int
	mustScan(t, d, `SELECT count(*) FROM "SignalTag" WHERE "signalId"='sg'`, &tagCount)
	if tagCount == 0 {
		t.Fatal("expected tags written")
	}
}

func TestMatchAndSendEdges(t *testing.T) {
	d := conn(t)
	ctx := context.Background()
	now := time.Now()

	// Missing signal → nil result, no error.
	if ids, err := MatchSubscriptions(ctx, d, "nope", now, nil); err != nil || ids != nil {
		t.Fatalf("missing signal match: %v %v", ids, err)
	}
	// Missing delivery → no-op.
	if err := SendDelivery(ctx, d, nil, "s", "nope", false, now); err != nil {
		t.Fatalf("missing delivery: %v", err)
	}
	// Already-SENT delivery → early return.
	mustExec(t, d, `INSERT INTO "Subscription" ("id","name","channel","filter","config","createdAt") VALUES ('sub','S','POLLING','{}','{}',now())`)
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

	first, err := FetchSource(ctx, d, "s", time.Now(), 3, time.Hour)
	if err != nil || len(first) != 1 {
		t.Fatalf("first fetch: %v %v", first, err)
	}
	// Second fetch: the guid already exists → dedup skip branch.
	second, err := FetchSource(ctx, d, "s", time.Now(), 3, time.Hour)
	if err != nil || len(second) != 0 {
		t.Fatalf("second fetch should dedup: %v %v", second, err)
	}
}

func TestClosedDBErrors(t *testing.T) {
	d := closedDB(t)
	ctx := context.Background()
	now := time.Now()
	gw := llm.NewOpenAIGateway("", "")
	if _, err := FetchSource(ctx, d, "x", now, 3, time.Hour); err == nil {
		t.Fatal("FetchSource should error on closed DB")
	}
	if _, err := NormalizeRawItem(ctx, d, "x"); err == nil {
		t.Fatal("Normalize should error")
	}
	if _, err := ClusterArticle(ctx, d, "x", now); err == nil {
		t.Fatal("Cluster should error")
	}
	if err := EnrichSignal(ctx, d, gw, nil, "x", now); err == nil {
		t.Fatal("Enrich should error")
	}
	if _, err := MatchSubscriptions(ctx, d, "x", now, nil); err == nil {
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
	if _, err := FetchSource(ctx, d, "s", time.Now(), 3, time.Hour); err == nil {
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
