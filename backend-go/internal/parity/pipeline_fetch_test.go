package parity_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
	"github.com/worldsignal/backend/internal/ingestion"
	"github.com/worldsignal/backend/internal/parity"
	"github.com/worldsignal/backend/internal/pipeline"
)

const fixtureFeed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/" xmlns:dc="http://purl.org/dc/elements/1.1/">
<channel>
<title>Fixture Feed</title>
<link>https://fixture.example</link>
<description>test</description>
<item>
  <title>Quake hits region</title>
  <link>https://fixture.example/a?utm_source=x</link>
  <guid>guid-a</guid>
  <pubDate>Mon, 02 Jan 2026 01:00:00 GMT</pubDate>
  <dc:creator>Jane Doe</dc:creator>
  <content:encoded><![CDATA[<p>A strong earthquake &amp; aftershocks struck the region.</p>]]></content:encoded>
  <description>short desc</description>
</item>
<item>
  <title>Markets rally</title>
  <link>https://fixture.example/b</link>
  <guid>guid-b</guid>
  <pubDate>Mon, 02 Jan 2026 02:00:00 GMT</pubDate>
  <description>Stocks climbed &lt;b&gt;today&lt;/b&gt;.</description>
</item>
</channel>
</rss>`

const rawItemFetchSnap = `SELECT row_to_json(t) FROM (SELECT "sourceId","sourceGuid","rawUrl","rawTitle","rawContent","status" FROM "RawItem" ORDER BY "sourceGuid") t`
const sourceBookkeepSnap = `SELECT row_to_json(t) FROM (SELECT "failureCount" FROM "Source" WHERE "id"='src_f') t`

// extractRSSFields pulls the parity-relevant string fields from the TS rss stage
// output, ignoring author/publishedAt/rawPayload.
func extractRSSFields(t *testing.T, raw []byte) []map[string]string {
	t.Helper()
	var items []struct {
		SourceGuid *string `json:"sourceGuid"`
		URL        *string `json:"url"`
		Title      string  `json:"title"`
		Content    string  `json:"content"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("parse rss output: %v\nraw: %s", err, raw)
	}
	out := make([]map[string]string, len(items))
	for i, it := range items {
		out[i] = map[string]string{
			"sourceGuid": derefS(it.SourceGuid), "url": derefS(it.URL),
			"title": it.Title, "content": it.Content,
		}
	}
	return out
}

func feedServer(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(fixtureFeed))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func seedFetchSource(t *testing.T, d *db.DB, feedURL string) {
	mkExec(t, d)(`INSERT INTO "Source" ("id","name","url","updatedAt") VALUES ('src_f','Fixture',$1,now())`, feedURL)
}

func TestPipelineRSSExtractionParity(t *testing.T) {
	if testing.Short() {
		t.Skip("needs node")
	}
	url := feedServer(t)

	tsOut, err := parity.RunTSStage("rss", `{"url":"`+url+`"}`, dbtest.URL(), paritySecret)
	if err != nil {
		t.Fatal(err)
	}
	// Compare only the fields stored downstream (rawPayload is parser-internal).
	tsItems := extractRSSFields(t, tsOut)

	goItems, err := ingestion.FetchRSSSource(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	if len(tsItems) != len(goItems) {
		t.Fatalf("item count TS=%d Go=%d", len(tsItems), len(goItems))
	}
	for i, g := range goItems {
		ts := tsItems[i]
		if ts["title"] != g.Title || ts["content"] != g.Content ||
			ts["sourceGuid"] != derefS(g.SourceGuid) || ts["url"] != derefS(g.URL) {
			t.Fatalf("rss item %d differs:\nTS: %v\nGo: title=%q content=%q guid=%q url=%q",
				i, ts, g.Title, g.Content, derefS(g.SourceGuid), derefS(g.URL))
		}
	}
}

func TestPipelineFetchParity(t *testing.T) {
	if testing.Short() {
		t.Skip("needs DB + node")
	}
	d := dbtest.Connect(t)
	url := feedServer(t)

	dbtest.Reset(t, d)
	seedFetchSource(t, d, url)
	if _, err := parity.RunTSStage("fetch", `{"sourceId":"src_f"}`, dbtest.URL(), paritySecret); err != nil {
		t.Fatal(err)
	}
	tsRaw := snapshot(t, d, rawItemFetchSnap)
	tsSrc := snapshot(t, d, sourceBookkeepSnap)

	dbtest.Reset(t, d)
	seedFetchSource(t, d, url)
	if _, err := pipeline.FetchSource(context.Background(), d, "src_f", time.Now()); err != nil {
		t.Fatal(err)
	}
	goRaw := snapshot(t, d, rawItemFetchSnap)
	goSrc := snapshot(t, d, sourceBookkeepSnap)

	eqSnapshots(t, "fetch rawitems", tsRaw, goRaw)
	eqSnapshots(t, "fetch source bookkeeping", tsSrc, goSrc)
}
