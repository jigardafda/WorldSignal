package pipeline

import (
	"context"
	"encoding/json"
	"time"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/llm"
)

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
// aggregation. Confidence blends LLM/heuristic confidence, source independence,
// and average credibility. Mirrors enrichSignal.ts. `now` is injected.
func EnrichSignal(ctx context.Context, d *db.DB, gw llm.Gateway, signalID string, now time.Time) error {
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
	enr := llm.EnrichArticle(ctx, gw, llm.EnrichInput{Title: rep.Title, Body: body, Publisher: rep.SourceName})

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
	meta["distinctSources"] = distinctSources

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
	})
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
