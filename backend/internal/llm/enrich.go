// Package llm ports backend/src/llm: a provider gateway and the article
// enrichment with a deterministic heuristic fallback used when no API key is set.
package llm

import (
	"context"
	"regexp"
	"sort"
	"strings"

	"github.com/worldsignal/backend/internal/taxonomy"
	"github.com/worldsignal/backend/internal/textutil"
)

// TagConf is a tag code with a confidence.
type TagConf struct {
	Code       string  `json:"code"`
	Confidence float64 `json:"confidence"`
}

// EnrichmentResult mirrors the TS EnrichmentResult. The narrative fields are
// always English; when the source article is in another language the LLM
// translates them and reports the detected source Language (ISO 639-1).
type EnrichmentResult struct {
	Title        string    `json:"title"`
	Summary      string    `json:"summary"`
	WhatHappened string    `json:"whatHappened"`
	WhyItMatters string    `json:"whyItMatters"`
	Severity     string    `json:"severity"`
	Confidence   float64   `json:"confidence"`
	Language     string    `json:"language"` // detected source language, ISO 639-1
	Translated   bool      `json:"translated"`
	Tags         []TagConf `json:"tags"`
	Source       string    `json:"source"`
}

// EnrichInput is the article to enrich.
type EnrichInput struct {
	Title     string
	Body      string
	Publisher string
}

var severityRe = regexp.MustCompile(`earthquake|flood|cyclone|war|attack|outbreak|breach|killed|dead|critical`)

// EnrichArticle runs the LLM if the gateway is enabled, else the heuristic.
// Mirrors enrichArticle in enrich.ts.
func EnrichArticle(ctx context.Context, gw Gateway, in EnrichInput) EnrichmentResult {
	if gw != nil && gw.Enabled() {
		if r := runLLM(ctx, gw, in); r != nil {
			return *r
		}
	}
	return heuristic(in)
}

func heuristic(in EnrichInput) EnrichmentResult {
	haystack := textutil.NormalizeText(in.Title + " " + in.Body)
	var scored []TagConf
	for _, tag := range taxonomy.LeafTags() {
		hits := 0
		for _, kw := range tag.Keywords {
			if strings.Contains(haystack, kw) {
				hits++
			}
		}
		if hits > 0 {
			conf := 0.5 + float64(hits)*0.15
			if conf > 0.9 {
				conf = 0.9
			}
			scored = append(scored, TagConf{Code: tag.Code, Confidence: conf})
		}
	}
	// Stable sort by confidence desc preserves taxonomy order on ties (JS sort is stable).
	sort.SliceStable(scored, func(i, j int) bool { return scored[i].Confidence > scored[j].Confidence })
	tags := scored
	if len(tags) > 3 {
		tags = tags[:3]
	}
	if len(tags) == 0 {
		tags = []TagConf{{Code: taxonomy.FallbackCode, Confidence: 0.3}}
	}

	severity := "MEDIUM"
	if severityRe.MatchString(haystack) {
		severity = "HIGH"
	}

	summary := textutil.FirstSentences(in.Body, 2)
	if summary == "" {
		summary = in.Title
	}

	return EnrichmentResult{
		Title:        in.Title,
		Summary:      summary,
		WhatHappened: textutil.FirstSentences(in.Body, 1),
		WhyItMatters: "",
		Severity:     severity,
		Confidence:   0.45,
		Tags:         tags,
		Source:       "heuristic",
	}
}
