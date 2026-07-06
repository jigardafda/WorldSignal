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

// stemToken applies light, symmetric singularization to a single token: it maps
// simple English plurals to a shared stem so "floods" matches keyword "flood".
// Because the SAME transform is applied to both the article text and the
// keywords, matches survive even when the stem is not a real word (e.g.
// "series"→"serie") — only symmetry matters. The "ss" guard protects words like
// "business"/"press" from losing their tail.
func stemToken(w string) string {
	if len(w) <= 3 || !strings.HasSuffix(w, "s") || strings.HasSuffix(w, "ss") {
		return w
	}
	switch {
	case strings.HasSuffix(w, "ies"): // companies→company, countries→country
		return w[:len(w)-3] + "y"
	case strings.HasSuffix(w, "es"): // cases→cas, launches→launch(e)
		return w[:len(w)-2]
	default:
		return w[:len(w)-1] // floods→flood, markets→market
	}
}

// stemPhrase normalizes text (lowercase, punctuation→space) and stems every
// token, so both the haystack and each keyword are compared in the same space.
func stemPhrase(s string) string {
	fields := strings.Fields(textutil.NormalizeText(s))
	for i, f := range fields {
		fields[i] = stemToken(f)
	}
	return strings.Join(fields, " ")
}

// containsWord reports whether stemmed keyword kw appears in stemmed haystack hay
// on word boundaries (both are space-delimited stemPhrase output). This avoids
// false substring hits like "war" ⊂ "warm" or "ai" ⊂ "said" that plagued the old
// strings.Contains matcher, while still matching plurals via the shared stem.
func containsWord(hay, kw string) bool {
	if kw == "" {
		return false
	}
	return strings.Contains(" "+hay+" ", " "+kw+" ")
}

// isOtherLeaf reports whether a code is a domain-level catch-all like
// POLITICS.OTHER (but not GENERAL.OTHER, the true last resort).
func isOtherLeaf(code string) bool {
	return strings.HasSuffix(code, ".OTHER") && code != taxonomy.FallbackCode
}

// ClassifyText assigns up to 3 taxonomy tags to article text using bounded
// keyword matching. Specific leaves always outrank their domain's `.OTHER`
// catch-all (which is scored lower and only surfaces when no specific leaf in
// the domain matched), and `.OTHER` catch-alls in turn outrank nothing —
// GENERAL.OTHER is returned only when the text matches no domain at all.
func ClassifyText(title, body string) []TagConf {
	haystack := stemPhrase(title + " " + body)
	var scored []TagConf
	for _, tag := range taxonomy.LeafTags() {
		hits := 0
		for _, kw := range tag.Keywords {
			if containsWord(haystack, stemPhrase(kw)) {
				hits++
			}
		}
		if hits == 0 {
			continue
		}
		var conf float64
		if isOtherLeaf(tag.Code) {
			// Domain catch-all: kept below any specific leaf's floor (0.65) so a
			// specific match always wins, yet above GENERAL.OTHER.
			conf = 0.45 + float64(hits)*0.03
			if conf > 0.6 {
				conf = 0.6
			}
		} else {
			conf = 0.65 + float64(hits)*0.12
			if conf > 0.95 {
				conf = 0.95
			}
		}
		scored = append(scored, TagConf{Code: tag.Code, Confidence: conf})
	}
	// Stable sort by confidence desc preserves taxonomy order on ties.
	sort.SliceStable(scored, func(i, j int) bool { return scored[i].Confidence > scored[j].Confidence })

	// Drop a domain's `.OTHER` catch-all when a specific leaf from the SAME domain
	// already matched — the specific tag subsumes it.
	matchedDomain := map[string]bool{}
	for _, t := range scored {
		if !isOtherLeaf(t.Code) {
			matchedDomain[domainOf(t.Code)] = true
		}
	}
	var tags []TagConf
	for _, t := range scored {
		if isOtherLeaf(t.Code) && matchedDomain[domainOf(t.Code)] {
			continue
		}
		tags = append(tags, t)
		if len(tags) == 3 {
			break
		}
	}
	if len(tags) == 0 {
		tags = []TagConf{{Code: taxonomy.FallbackCode, Confidence: 0.3}}
	}
	return tags
}

// domainOf returns the domain prefix of a taxonomy code (the part before ".").
func domainOf(code string) string {
	if i := strings.IndexByte(code, '.'); i >= 0 {
		return code[:i]
	}
	return code
}

func heuristic(in EnrichInput) EnrichmentResult {
	haystack := textutil.NormalizeText(in.Title + " " + in.Body)
	tags := ClassifyText(in.Title, in.Body)

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
