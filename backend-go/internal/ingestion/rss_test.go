package ingestion

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const feed = `<?xml version="1.0"?>
<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel><title>F</title><link>https://f.example</link><description>d</description>
<item>
  <title>Quake hits</title>
  <link>https://f.example/a</link>
  <guid>g-a</guid>
  <content:encoded><![CDATA[<p>Strong &amp; deep quake.</p>]]></content:encoded>
</item>
<item>
  <title>No content item</title>
  <link>https://f.example/b</link>
  <description>Falls back to &lt;b&gt;description&lt;/b&gt;.</description>
</item>
<item><title></title><link>https://f.example/c</link></item>
</channel></rss>`

func TestFetchRSSSource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(feed))
	}))
	defer srv.Close()

	items, err := FetchRSSSource(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 { // empty-title item skipped
		t.Fatalf("want 2 items, got %d", len(items))
	}
	if items[0].Title != "Quake hits" || items[0].Content != "Strong & deep quake." {
		t.Fatalf("item0 wrong: %+v", items[0])
	}
	if items[0].SourceGuid == nil || *items[0].SourceGuid != "g-a" {
		t.Fatalf("guid wrong: %+v", items[0].SourceGuid)
	}
	// Second item falls back to description and strips tags.
	// </b> becomes a space, matching the TS stripHtml behavior.
	if items[1].Content != "Falls back to description ." {
		t.Fatalf("item1 content %q", items[1].Content)
	}
	// No guid → falls back to link.
	if items[1].SourceGuid == nil || *items[1].SourceGuid != "https://f.example/b" {
		t.Fatalf("guid fallback wrong: %v", items[1].SourceGuid)
	}
}

func TestFetchRSSSourceBadFeed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("this is not a feed"))
	}))
	defer srv.Close()
	if _, err := FetchRSSSource(context.Background(), srv.URL); err == nil {
		t.Fatal("expected parse error for non-feed body")
	}
}

func TestFetchRSSSourceBadURL(t *testing.T) {
	if _, err := FetchRSSSource(context.Background(), "://nope"); err == nil {
		t.Fatal("expected error for bad URL")
	}
}

func TestFetchRSSSourceConnRefused(t *testing.T) {
	if _, err := FetchRSSSource(context.Background(), "http://127.0.0.1:0/"); err == nil {
		t.Fatal("expected connection error")
	}
}
