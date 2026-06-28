// Package ingestion ports backend/src/ingestion/rss.ts: fetch and parse an
// RSS/Atom feed into discovered items. Field extraction mirrors the rss-parser
// priorities the TS code relies on.
package ingestion

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/worldsignal/backend/internal/textutil"
)

// DiscoveredItem mirrors the TS DiscoveredItem (sans rawPayload, which is parser
// -internal provenance and not part of the parity contract).
type DiscoveredItem struct {
	SourceGuid  *string
	URL         *string
	Title       string
	Content     string
	Author      *string
	PublishedAt *time.Time
}

const userAgent = "WorldSignalBot/0.1 (+https://worldsignal.example/bot)"

// FetchRSSSource fetches and parses a feed. Mirrors fetchRssSource in rss.ts.
func FetchRSSSource(ctx context.Context, url string) ([]DiscoveredItem, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	feed, err := gofeed.NewParser().Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	var items []DiscoveredItem
	for _, it := range feed.Items {
		title := strings.TrimSpace(it.Title)
		if title == "" {
			continue
		}
		// content:encoded || content || description (stripped).
		content := textutil.StripHtml(firstNonEmpty(it.Content, it.Description))
		di := DiscoveredItem{
			SourceGuid:  ptrIf(firstNonEmpty(it.GUID, it.Link)),
			URL:         ptrIf(firstNonEmpty(it.Link)),
			Title:       title,
			Content:     content,
			Author:      ptrIf(author(it)),
			PublishedAt: it.PublishedParsed,
		}
		items = append(items, di)
	}
	return items, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func ptrIf(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func author(it *gofeed.Item) string {
	if len(it.Authors) > 0 && it.Authors[0] != nil {
		return it.Authors[0].Name
	}
	if it.Author != nil {
		return it.Author.Name
	}
	return ""
}
