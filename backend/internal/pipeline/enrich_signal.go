package pipeline

import (
	"context"
	"encoding/json"
	"time"

	"github.com/worldsignal/backend/internal/crawl"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/llm"
)

// PageCrawler fetches richer context from a source article's web page. The
// concrete *crawl.Crawler implements it; tests may pass nil to skip crawling.
type PageCrawler interface {
	Fetch(ctx context.Context, url string) crawl.Result
}

func clamp01(n float64) float64 {
	if n < 0 {
		return 0
	}
	if n > 1 {
		return 1
	}
	return n
}

// EnrichSignal enriches a signal from its representative article + source
// aggregation. Beyond the narrative fields it crawls the representative article's
// web page (best-effort, via cr) for richer context and extracts the full
// dictionary-constrained attribute set (geo, sentiment, influence, relevance,
// industries, categories, entities). Confidence blends LLM/heuristic confidence,
// source independence, and average credibility. `now` is injected; `cr` may be
// nil to skip crawling. Re-running on each newly linked article continuously
// deepens the signal's attributes.
func EnrichSignal(ctx context.Context, d *db.DB, gw llm.Gateway, cr PageCrawler, signalID string, now time.Time) error {
	sig, err := d.LoadSignalForEnrich(ctx, signalID)
	if err != nil || sig == nil {
		return err
	}

	// Representative: PRIMARY link, else the article with the longest body.
	rep := pickRepresentative(sig.Links)

	body := derefStr(rep.Body)
	if body == "" {
		body = derefStr(rep.Summary)
	}
	if body == "" {
		body = rep.Title
	}

	// Best-effort crawl of the source page for richer context.
	pageText := ""
	if cr != nil && rep.CanonicalURL != nil && *rep.CanonicalURL != "" {
		if res := cr.Fetch(ctx, *rep.CanonicalURL); res.OK() {
			pageText = res.Text
		}
	}

	// The crawled page (when available) is richer than the feed snippet.
	enrichBody := body
	if pageText != "" {
		enrichBody = pageText
	}
	enr := llm.EnrichArticle(ctx, gw, llm.EnrichInput{Title: rep.Title, Body: enrichBody, Publisher: rep.SourceName})
	attrs := llm.ExtractAttributes(ctx, gw, llm.AttrInput{Title: rep.Title, Body: body, Publisher: rep.SourceName, Context: pageText})

	// Source aggregation.
	var credSum float64
	distinct := map[string]struct{}{}
	for _, l := range sig.Links {
		credSum += l.Credibility
		distinct[l.SourceID] = struct{}{}
	}
	avgCred := credSum / float64(len(sig.Links))
	distinctSources := len(distinct)
	independence := float64(distinctSources) / 5
	if independence > 1 {
		independence = 1
	}
	confidence := clamp01(0.4*enr.Confidence + 0.3*independence + 0.3*avgCred)

	status := "UNVERIFIED"
	if distinctSources >= 3 {
		status = "CONFIRMED"
	} else if distinctSources == 2 {
		status = "DEVELOPING"
	}

	var eventType *string
	if len(enr.Tags) > 0 {
		c := enr.Tags[0].Code
		eventType = &c
	}

	codes := make([]string, len(enr.Tags))
	for i, tg := range enr.Tags {
		codes[i] = tg.Code
	}
	codeToID, err := d.TagIDsByCodes(ctx, codes)
	if err != nil {
		return err
	}
	var tagAssignments []db.TagAssignment
	for _, tg := range enr.Tags {
		if id, ok := codeToID[tg.Code]; ok {
			tagAssignments = append(tagAssignments, db.TagAssignment{TagID: id, Confidence: tg.Confidence})
		}
	}

	publishedAt := now
	if sig.PublishedAt != nil {
		publishedAt = *sig.PublishedAt
	}

	meta := map[string]any{}
	if len(sig.Metadata) > 0 {
		_ = json.Unmarshal(sig.Metadata, &meta)
	}
	meta["enrichmentSource"] = enr.Source
	meta["attributeSource"] = attrs.Source
	meta["distinctSources"] = distinctSources
	if pageText != "" {
		meta["crawled"] = true
	}

	// Mirror the taxonomy categories into the unified attribute dictionary so
	// every dimension is queryable consistently, alongside industries/entities.
	attrRows := buildAttributeRows(enr.Tags, attrs)

	return d.ApplyEnrichment(ctx, signalID, db.EnrichmentUpdate{
		Title:        enr.Title,
		Summary:      enr.Summary,
		WhatHappened: nilIfEmpty(enr.WhatHappened),
		WhyItMatters: nilIfEmpty(enr.WhyItMatters),
		Severity:     enr.Severity,
		Confidence:   confidence,
		Status:       status,
		EventType:    eventType,
		PublishedAt:  publishedAt,
		Metadata:     meta,
		Tags:         tagAssignments,

		Country:        nilIfEmpty(attrs.Country),
		Region:         nilIfEmpty(attrs.Region),
		City:           nilIfEmpty(attrs.City),
		Locality:       nilIfEmpty(attrs.Locality),
		GeoScope:       nilIfEmpty(attrs.GeoScope),
		Sentiment:      nilIfEmpty(attrs.Sentiment),
		SentimentScore: &attrs.SentimentScore,
		Influence:      nilIfEmpty(attrs.Influence),
		Relevance:      &attrs.Relevance,
		Attributes:     attrRows,
	})
}

// buildAttributeRows flattens the extracted categories, industries and entities
// into normalized SignalAttribute rows. Values are already dictionary-valid.
func buildAttributeRows(tags []llm.TagConf, attrs llm.AttributeResult) []db.SignalAttr {
	var rows []db.SignalAttr
	for _, tg := range tags {
		rows = append(rows, db.SignalAttr{Key: "category", ValueCode: tg.Code, Confidence: tg.Confidence})
	}
	for _, ind := range attrs.Industries {
		rows = append(rows, db.SignalAttr{Key: "industry", ValueCode: ind, Confidence: 1})
	}
	for _, e := range attrs.Entities {
		rows = append(rows, db.SignalAttr{Key: "entity", ValueCode: e.Type, ValueText: e.Name, Confidence: e.Confidence})
	}
	return rows
}

func pickRepresentative(links []db.EnrichLink) db.EnrichLink {
	for _, l := range links {
		if l.RelationType == "PRIMARY" {
			return l
		}
	}
	// Longest body wins; ties keep the earliest (input is addedAt-ordered).
	best := links[0]
	bestLen := runeLen(best.Body)
	for _, l := range links[1:] {
		if n := runeLen(l.Body); n > bestLen {
			best = l
			bestLen = n
		}
	}
	return best
}

func runeLen(s *string) int {
	if s == nil {
		return 0
	}
	return len([]rune(*s))
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
