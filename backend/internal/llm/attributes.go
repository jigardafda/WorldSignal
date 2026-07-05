package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/worldsignal/backend/internal/attributes"
	"github.com/worldsignal/backend/internal/textutil"
)

// Entity is an affected/involved entity extracted from a signal.
type Entity struct {
	Name       string  `json:"name"`
	Type       string  `json:"type"` // attributes "entityType" code
	Confidence float64 `json:"confidence"`
}

// AttributeResult is the dictionary-constrained structured extraction that
// complements EnrichArticle's narrative fields. Every value here has already
// been normalized through the attributes dictionary; unknown enum values are
// dropped (left empty) rather than stored. Scalars are clamped to range.
type AttributeResult struct {
	Country        string   `json:"country"` // ISO 3166-1 alpha-2 or ""
	Region         string   `json:"region"`
	City           string   `json:"city"`
	Locality       string   `json:"locality"`
	GeoScope       string   `json:"geoScope"`
	Sentiment      string   `json:"sentiment"`
	SentimentScore float64  `json:"sentimentScore"`
	Influence      string   `json:"influence"`
	Relevance      float64  `json:"relevance"`
	Industries     []string `json:"industries"`
	Entities       []Entity `json:"entities"`
	Source         string   `json:"source"` // "llm" | "heuristic"
}

// AttrInput is the article (plus optional crawled page context) to extract from.
type AttrInput struct {
	Title     string
	Body      string
	Publisher string
	Context   string // richer text crawled from the source page (best-effort)
}

// ExtractAttributes runs the LLM extractor when the gateway is enabled, else a
// deterministic heuristic. The result is always dictionary-valid.
func ExtractAttributes(ctx context.Context, gw Gateway, in AttrInput) AttributeResult {
	if gw != nil && gw.Enabled() {
		if r := runAttrLLM(ctx, gw, in); r != nil {
			return *r
		}
	}
	return heuristicAttributes(in)
}

// allowedList renders an attribute's vocabulary as "CODE (Label)" lines for the
// prompt, so the model is constrained to the closed dictionary.
func allowedList(key string) string {
	d, ok := attributes.Lookup(key)
	if !ok {
		return ""
	}
	var b strings.Builder
	for i, v := range d.Values() {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(v.Code)
	}
	return b.String()
}

type attrRaw struct {
	Country        *string  `json:"country"`
	Region         *string  `json:"region"`
	City           *string  `json:"city"`
	Locality       *string  `json:"locality"`
	GeoScope       *string  `json:"geoScope"`
	Sentiment      *string  `json:"sentiment"`
	SentimentScore *float64 `json:"sentimentScore"`
	Influence      *string  `json:"influence"`
	Relevance      *float64 `json:"relevance"`
	Industries     []string `json:"industries"`
	Entities       []struct {
		Name *string `json:"name"`
		Type *string `json:"type"`
	} `json:"entities"`
}

func runAttrLLM(ctx context.Context, gw Gateway, in AttrInput) *AttributeResult {
	system := strings.Join([]string{
		"You extract structured, standardized metadata from a news event.",
		"Return JSON only. Do not invent facts not supported by the text.",
		"Use ONLY the allowed enum codes provided; if none fit, omit the field.",
		"For country use the ISO 3166-1 alpha-2 code (e.g. US, IN, GB) or null for global events.",
		"region/city/locality are free text place names (or null).",
		"",
		"geoScope allowed: " + allowedList("geoScope"),
		"sentiment allowed: " + allowedList("sentiment"),
		"influence allowed: " + allowedList("influence"),
		"industries allowed (array, choose all that apply): " + allowedList("industry"),
		"entity types allowed: " + allowedList("entityType"),
	}, "\n")

	body := in.Body
	if in.Context != "" {
		body = in.Context // crawled page text is richer than the feed snippet
	}
	if len(body) > 6000 {
		body = strings.ToValidUTF8(body[:6000], "") // drop a rune split by the byte cut
	}
	publisher := in.Publisher
	if publisher == "" {
		publisher = "unknown"
	}
	user := strings.Join([]string{
		"Produce JSON with keys: country, region, city, locality,",
		"geoScope, sentiment, sentimentScore (-1..1), influence,",
		"relevance (0..1), industries (array of codes),",
		"entities (array of {name, type}).",
		"",
		"PUBLISHER: " + publisher,
		"TITLE: " + in.Title,
		"BODY: " + body,
	}, "\n")

	raw, err := gw.JSONCompletion(ctx, system, user, 700)
	if err != nil || raw == nil {
		return nil
	}
	var p attrRaw
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil
	}

	r := AttributeResult{Source: "llm"}
	if p.Country != nil {
		if code, ok := attributes.NormalizeCountry(*p.Country); ok {
			r.Country = code
		}
	}
	r.Region = trimStr(p.Region)
	r.City = trimStr(p.City)
	r.Locality = trimStr(p.Locality)
	r.GeoScope = normEnum("geoScope", p.GeoScope)
	r.Sentiment = normEnum("sentiment", p.Sentiment)
	r.Influence = normEnum("influence", p.Influence)
	if p.SentimentScore != nil {
		d, _ := attributes.Lookup("sentimentScore")
		r.SentimentScore = d.ClampScalar(*p.SentimentScore)
	}
	if p.Relevance != nil {
		d, _ := attributes.Lookup("relevance")
		r.Relevance = d.ClampScalar(*p.Relevance)
	} else {
		r.Relevance = 0.5
	}
	r.Industries = normIndustries(p.Industries)

	etDef, _ := attributes.Lookup("entityType")
	seenEntity := map[string]bool{}
	for _, e := range p.Entities {
		name := trimStr(e.Name)
		if name == "" {
			continue
		}
		typ := "OTHER"
		if e.Type != nil {
			if code, ok := etDef.Normalize(*e.Type); ok {
				typ = code
			}
		}
		key := strings.ToLower(name) + "|" + typ
		if seenEntity[key] {
			continue
		}
		seenEntity[key] = true
		r.Entities = append(r.Entities, Entity{Name: name, Type: typ, Confidence: 0.8})
	}
	return &r
}

func normEnum(key string, raw *string) string {
	if raw == nil {
		return ""
	}
	d, ok := attributes.Lookup(key)
	if !ok {
		return ""
	}
	if code, ok := d.Normalize(*raw); ok {
		return code
	}
	return ""
}

func normIndustries(raw []string) []string {
	d, _ := attributes.Lookup("industry")
	var out []string
	seen := map[string]bool{}
	for _, s := range raw {
		if code, ok := d.Normalize(s); ok && !seen[code] {
			seen[code] = true
			out = append(out, code)
		}
	}
	return out
}

// heuristicAttributes derives deterministic attributes without an LLM: industry
// tags via dictionary alias matching, and sentiment from severity cues. Geo is
// left to the article/source metadata the pipeline already carries.
func heuristicAttributes(in AttrInput) AttributeResult {
	hay := textutil.NormalizeText(in.Title + " " + in.Body + " " + in.Context)
	r := AttributeResult{Source: "heuristic", Sentiment: "NEUTRAL", SentimentScore: 0, Relevance: 0.5}
	if severityRe.MatchString(hay) {
		r.Sentiment = "NEGATIVE"
		r.SentimentScore = -0.4
		r.Influence = "HIGH"
	}
	r.Industries = matchIndustries(hay)
	return r
}

// matchIndustries scans normalized text for any industry code/label/alias.
func matchIndustries(hay string) []string {
	d, _ := attributes.Lookup("industry")
	var out []string
	seen := map[string]bool{}
	for _, val := range d.Values() {
		terms := append([]string{val.Label}, val.Aliases...)
		for _, term := range terms {
			t := textutil.NormalizeText(term)
			if t == "" {
				continue
			}
			if strings.Contains(" "+hay+" ", " "+t+" ") && !seen[val.Code] {
				seen[val.Code] = true
				out = append(out, val.Code)
				break
			}
		}
	}
	return out
}

func trimStr(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

// AttributesPromptDigest is exposed for diagnostics/tests: a stable digest of the
// allowed enum codes baked into the prompt, so drift in the dictionary is visible.
func AttributesPromptDigest() string {
	return fmt.Sprintf("geoScope=%s|sentiment=%s|influence=%s|entityType=%s",
		allowedList("geoScope"), allowedList("sentiment"), allowedList("influence"), allowedList("entityType"))
}
