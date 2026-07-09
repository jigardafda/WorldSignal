package llm

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"github.com/worldsignal/backend/internal/taxonomy"
)

// ProfileDraft is a proposed relevance profile generated from a brand document —
// the AI-native alternative to filling the creation form by hand. Interests is
// the weighted graph (dimension:value -> weight); Reasons carries the provenance
// shown in the UI ("from your document" / "inferred").
type ProfileDraft struct {
	Name        string             `json:"name"`
	Summary     string             `json:"summary"`
	Interests   map[string]float64 `json:"interests"`
	MinScore    float64            `json:"minScore"`
	MinSeverity string             `json:"minSeverity"`
	Reasons     []DraftReason      `json:"reasons"`
	Source      string             `json:"source"` // "llm" | "heuristic"
}

// DraftReason explains one interest: which key, why, and where it came from.
type DraftReason struct {
	Key    string `json:"key"`
	Why    string `json:"why"`
	Origin string `json:"origin"` // "doc" | "web" | "inferred"
}

// DraftProfileFromDocument reads a brand document (media kit, brief, product page,
// contract, or crawled site text) and proposes a ranked profile. It uses the LLM
// when the gateway is enabled — constrained to the taxonomy, exactly like
// enrichment — and falls back to a deterministic heuristic otherwise so onboarding
// always produces something usable.
func DraftProfileFromDocument(ctx context.Context, gw Gateway, docText string) ProfileDraft {
	if gw != nil && gw.Enabled() {
		if d := draftLLM(ctx, gw, docText); d != nil {
			return *d
		}
	}
	return draftHeuristic(docText)
}

type draftRaw struct {
	Name        *string            `json:"name"`
	Summary     *string            `json:"summary"`
	Interests   map[string]float64 `json:"interests"`
	MinScore    *float64           `json:"minScore"`
	MinSeverity *string            `json:"minSeverity"`
	Reasons     []DraftReason      `json:"reasons"`
}

func draftLLM(ctx context.Context, gw Gateway, docText string) *ProfileDraft {
	system := strings.Join([]string{
		"You configure a real-time monitoring profile for a brand from a document.",
		"Return JSON only. Extract what the brand should watch and how much each matters.",
		"Interests keys are 'dimension:value' where dimension is one of:",
		"  entity:<name>     — the brand, its competitors, sponsored people/teams",
		"  tag:<CODE>        — a topic; CODE MUST come from the taxonomy below",
		"  country:<ISO2>    — a market (ISO 3166-1 alpha-2)",
		"  keyword:<word>    — a distinctive term to track",
		"  sentiment:<POSITIVE|NEGATIVE>",
		"Weights are 1..5 (5 = most important). Infer competitors and key markets",
		"even if not named explicitly. Suggest minScore (0..10) and minSeverity",
		"(LOW|MEDIUM|HIGH|CRITICAL) from the brand's likely intent.",
		"For each interest add a reason: {key, why, origin} where origin is",
		"'doc' (stated in the document), 'web' (a competitor/market you inferred",
		"about the brand), or 'inferred' (a topic mapping).",
		"",
		"Taxonomy (use these tag CODEs):",
		buildTaxonomyGuide(),
	}, "\n")
	body := docText
	if len(body) > 8000 {
		body = strings.ToValidUTF8(body[:8000], "")
	}
	user := strings.Join([]string{
		"Produce JSON: {name, summary, interests:{\"dim:value\":weight},",
		"minScore, minSeverity, reasons:[{key, why, origin}]}. Max 12 interests.",
		"",
		"DOCUMENT:",
		body,
	}, "\n")

	raw, err := gw.JSONCompletion(ctx, system, user, 900)
	if err != nil || raw == nil {
		return nil
	}
	var p draftRaw
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil
	}
	interests := map[string]float64{}
	for k, w := range p.Interests {
		if k = validInterestKey(k); k != "" && w > 0 {
			if w > 5 {
				w = 5
			}
			interests[k] = w
		}
	}
	if len(interests) == 0 {
		return nil // nothing usable — fall back to the heuristic
	}
	d := ProfileDraft{
		Name:        strDefault(p.Name, "Custom profile"),
		Summary:     strDefault(p.Summary, ""),
		Interests:   interests,
		MinScore:    6.5,
		MinSeverity: "MEDIUM",
		Reasons:     filterReasons(p.Reasons, interests),
		Source:      "llm",
	}
	if p.MinScore != nil && *p.MinScore >= 0 && *p.MinScore <= 10 {
		d.MinScore = *p.MinScore
	}
	if p.MinSeverity != nil && validSeverity[*p.MinSeverity] {
		d.MinSeverity = *p.MinSeverity
	}
	return &d
}

// validInterestKey keeps only well-formed keys, and for tag: keys requires the
// code (or its domain) to be in the taxonomy.
func validInterestKey(key string) string {
	i := strings.IndexByte(key, ':')
	if i <= 0 || i == len(key)-1 {
		return ""
	}
	dim, val := strings.ToLower(key[:i]), key[i+1:]
	switch dim {
	case "entity", "country", "keyword":
		return dim + ":" + strings.TrimSpace(val)
	case "sentiment":
		up := strings.ToUpper(strings.TrimSpace(val))
		if up == "POSITIVE" || up == "NEGATIVE" || up == "NEUTRAL" {
			return "sentiment:" + up
		}
	case "tag":
		up := strings.ToUpper(strings.TrimSpace(val))
		if _, ok := taxonomy.ValidCodes[up]; ok {
			return "tag:" + up
		}
	}
	return ""
}

func filterReasons(rs []DraftReason, interests map[string]float64) []DraftReason {
	var out []DraftReason
	for _, r := range rs {
		if _, ok := interests[r.Key]; ok {
			if r.Origin == "" {
				r.Origin = "inferred"
			}
			out = append(out, r)
		}
	}
	return out
}

// properNoun matches a capitalized word or short phrase. Sentence punctuation is
// excluded from the token so "India. This" splits into "India" and "This".
var properNoun = regexp.MustCompile(`\b([A-Z][a-zA-Z0-9&\-]+(?:\s+[A-Z][a-zA-Z0-9&\-]+){0,2})\b`)

// draftHeuristic derives a usable profile without an LLM: topics via the taxonomy
// classifier, entities via proper-noun frequency, and sensible default gates.
func draftHeuristic(docText string) ProfileDraft {
	interests := map[string]float64{}
	var reasons []DraftReason

	// Topics: the strongest categories the document is about.
	for i, tag := range ClassifyText(docText, "") {
		if tag.Code == taxonomy.FallbackCode {
			continue
		}
		w := 4.0 - float64(i) // 4,3,2...
		if w < 2 {
			w = 2
		}
		key := "tag:" + domainOf(tag.Code)
		if _, seen := interests[key]; !seen {
			interests[key] = w
			reasons = append(reasons, DraftReason{Key: key, Why: "A dominant topic in the document.", Origin: "inferred"})
		}
	}

	// Entities: the most frequent proper nouns (likely the brand + key names).
	freq := map[string]int{}
	for _, m := range properNoun.FindAllString(docText, -1) {
		m = strings.TrimSpace(m)
		if len(m) > 2 && !commonWord(m) {
			freq[m]++
		}
	}
	type nf struct {
		n string
		f int
	}
	var ranked []nf
	for n, f := range freq {
		ranked = append(ranked, nf{n, f})
	}
	// Frequency desc, then name asc so ties are deterministic (map iteration order
	// is not) — the same document always yields the same draft.
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].f != ranked[j].f {
			return ranked[i].f > ranked[j].f
		}
		return ranked[i].n < ranked[j].n
	})
	for i, e := range ranked {
		if i >= 4 {
			break
		}
		w := 5.0 - float64(i)
		key := "entity:" + e.n
		interests[key] = w
		reasons = append(reasons, DraftReason{Key: key, Why: "A prominent name in the document.", Origin: "doc"})
	}

	name := "Custom profile"
	if len(ranked) > 0 {
		name = ranked[0].n + " watch"
	}
	return ProfileDraft{
		Name: name, Summary: "Drafted from the document's topics and key names.",
		Interests: interests, MinScore: 6.0, MinSeverity: "MEDIUM",
		Reasons: reasons, Source: "heuristic",
	}
}

var stopNouns = map[string]bool{
	"The": true, "This": true, "That": true, "We": true, "Our": true, "You": true,
	"Your": true, "It": true, "A": true, "An": true, "In": true, "On": true, "For": true,
	"And": true, "But": true, "With": true, "From": true, "To": true, "As": true,
}

func commonWord(s string) bool {
	if stopNouns[s] {
		return true
	}
	// single very short token
	return len(s) <= 2
}
