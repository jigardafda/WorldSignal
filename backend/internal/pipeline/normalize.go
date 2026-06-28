// Package pipeline ports the WorldSignal processing stages (fetch → normalize →
// cluster → enrich → match → send) from backend/src/pipeline.
package pipeline

import (
	"context"
	"strings"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/textutil"
	"github.com/worldsignal/backend/internal/urlutil"
)

// NormalizeRawItem turns a RawItem into a normalized Article, applying exact
// dedupe. Returns the new article id, or "" if it was a duplicate/failed.
// Mirrors normalizeRawItem in normalize.ts.
func NormalizeRawItem(ctx context.Context, d *db.DB, rawItemID string) (string, error) {
	raw, err := d.GetRawItem(ctx, rawItemID)
	if err != nil || raw == nil {
		return "", err
	}
	if raw.Status == "PARSED" || raw.Status == "DUPLICATE" {
		return d.ArticleIDByRawItem(ctx, rawItemID)
	}

	title := strings.TrimSpace(deref(raw.RawTitle))
	body := strings.TrimSpace(deref(raw.RawContent))
	if title == "" {
		return "", d.SetRawItemStatus(ctx, rawItemID, "FAILED")
	}

	var canonical *string
	if c, ok := urlutil.Canonicalize(deref(raw.RawURL)); ok {
		canonical = &c
	}
	hash := textutil.ContentHash(title, body)

	dup, err := d.FindDuplicateArticle(ctx, hash, canonical)
	if err != nil {
		return "", err
	}
	if dup != "" {
		return "", d.SetRawItemStatus(ctx, rawItemID, "DUPLICATE")
	}

	var bodyPtr, summaryPtr *string
	if body != "" {
		bodyPtr = &body
		sum := sliceRunes(body, 280)
		summaryPtr = &sum
	}

	id, err := d.CreateArticle(ctx, db.NewArticle{
		RawItemID: raw.ID, SourceID: raw.SourceID, CanonicalURL: canonical,
		Title: title, Body: bodyPtr, Summary: summaryPtr, PublishedAt: raw.PublishedAt,
		ContentHash: hash, TokenSet: textutil.TokenSetString(title + " " + body),
	})
	if err != nil {
		return "", err
	}
	if err := d.SetRawItemStatus(ctx, rawItemID, "PARSED"); err != nil {
		return "", err
	}
	return id, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// sliceRunes returns the first n runes (approximating JS String.slice for BMP).
func sliceRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		r = r[:n]
	}
	return string(r)
}
