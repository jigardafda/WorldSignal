// Package attributes defines WorldSignal's closed attribute dictionary: the
// predefined, versioned registry of every structured attribute a Signal can
// carry. Enrichment must map every extracted value through this dictionary —
// arbitrary keys and out-of-vocabulary enum values are rejected — so the data
// stays consistent for filtering, searching, analytics and indexing. New
// attributes are introduced by appending to the registry, never by writing
// values under undefined keys.
package attributes

import (
	"strings"

	"github.com/worldsignal/backend/internal/countries"
	"github.com/worldsignal/backend/internal/taxonomy"
)

// Kind classifies how an attribute's value space behaves.
type Kind string

const (
	// KindEnum is a single value drawn from a closed controlled vocabulary.
	KindEnum Kind = "ENUM"
	// KindTagSet is zero-or-more values from a closed controlled vocabulary.
	KindTagSet Kind = "TAGSET"
	// KindScalar is a single number constrained to [Min, Max].
	KindScalar Kind = "SCALAR"
	// KindText is normalized free text (closed key, open value), e.g. a city name.
	KindText Kind = "TEXT"
	// KindGeoCountry is an ISO 3166-1 country, validated against the country set.
	KindGeoCountry Kind = "GEO_COUNTRY"
)

// Value is one entry in an enum/tagset vocabulary. Aliases are case-insensitive
// synonyms the normalizer also accepts (in addition to the code and label).
type Value struct {
	Code    string
	Label   string
	Aliases []string
}

// Definition describes one attribute in the dictionary.
type Definition struct {
	Key         string
	Label       string
	Kind        Kind
	Description string
	// Min/Max bound a KindScalar value; ignored for other kinds.
	Min, Max float64
	// Indexed marks attributes that back filtering/analytics (informational; used
	// by the schema/index design and surfaced to API consumers).
	Indexed bool

	values []Value
	index  map[string]string // lowercased code|label|alias -> canonical code
}

func def(d Definition, vals ...Value) Definition {
	d.values = vals
	if len(vals) > 0 {
		d.index = make(map[string]string, len(vals)*2)
		for _, v := range vals {
			d.index[strings.ToLower(v.Code)] = v.Code
			d.index[strings.ToLower(v.Label)] = v.Code
			for _, a := range v.Aliases {
				d.index[strings.ToLower(a)] = v.Code
			}
		}
	}
	return d
}

func v(code, label string, aliases ...string) Value {
	return Value{Code: code, Label: label, Aliases: aliases}
}

// registry is the closed, ordered attribute dictionary. Append new attributes
// here; never remove or repurpose an existing key.
var registry = []Definition{
	def(Definition{Key: "country", Label: "Country", Kind: KindGeoCountry, Indexed: true,
		Description: "ISO 3166-1 alpha-2 country the signal primarily concerns."}),
	def(Definition{Key: "region", Label: "Region / State", Kind: KindText, Indexed: true,
		Description: "Sub-national region, state or province."}),
	def(Definition{Key: "city", Label: "City", Kind: KindText, Indexed: true,
		Description: "City or town."}),
	def(Definition{Key: "locality", Label: "Locality", Kind: KindText,
		Description: "Neighbourhood, district or other fine-grained locality."}),
	def(Definition{Key: "geoScope", Label: "Geographic Scope", Kind: KindEnum, Indexed: true,
		Description: "Geographic reach of the signal."},
		v("GLOBAL", "Global", "world", "worldwide", "international"),
		v("MULTINATIONAL", "Multinational", "multi-country", "regional bloc", "cross-border"),
		v("NATIONAL", "National", "country", "countrywide", "nationwide"),
		v("REGIONAL", "Regional", "state", "province", "subnational"),
		v("LOCAL", "Local", "city", "municipal", "town"),
	),
	def(Definition{Key: "sentiment", Label: "Sentiment", Kind: KindEnum, Indexed: true,
		Description: "Overall sentiment of the event."},
		v("POSITIVE", "Positive", "good", "favorable", "favourable", "optimistic"),
		v("NEGATIVE", "Negative", "bad", "adverse", "pessimistic"),
		v("NEUTRAL", "Neutral", "factual", "balanced"),
		v("MIXED", "Mixed", "ambivalent", "both"),
	),
	def(Definition{Key: "sentimentScore", Label: "Sentiment Score", Kind: KindScalar, Min: -1, Max: 1,
		Description: "Signed sentiment polarity from -1 (very negative) to 1 (very positive)."}),
	def(Definition{Key: "influence", Label: "Influence", Kind: KindEnum, Indexed: true,
		Description: "Expected breadth and depth of the event's impact."},
		v("NEGLIGIBLE", "Negligible", "trivial", "minimal"),
		v("LOW", "Low", "minor", "limited"),
		v("MEDIUM", "Medium", "moderate"),
		v("HIGH", "High", "major", "significant"),
		v("CRITICAL", "Critical", "severe", "systemic"),
	),
	def(Definition{Key: "severity", Label: "Severity", Kind: KindEnum, Indexed: true,
		Description: "Severity of the event (reuses the Signal severity scale)."},
		v("LOW", "Low", "minor"),
		v("MEDIUM", "Medium", "moderate"),
		v("HIGH", "High", "major", "severe"),
		v("CRITICAL", "Critical", "catastrophic", "extreme"),
	),
	def(Definition{Key: "confidence", Label: "Confidence", Kind: KindScalar, Min: 0, Max: 1, Indexed: true,
		Description: "Confidence the signal is real and accurately characterized (0..1)."}),
	def(Definition{Key: "relevance", Label: "Relevance", Kind: KindScalar, Min: 0, Max: 1, Indexed: true,
		Description: "General newsworthiness / relevance of the signal (0..1)."}),
	def(Definition{Key: "industry", Label: "Industry", Kind: KindTagSet, Indexed: true,
		Description: "Industries/verticals the signal affects."}, industryValues...),
	def(Definition{Key: "category", Label: "Category / Topic", Kind: KindTagSet, Indexed: true,
		Description: "Topic categories from the closed WorldSignal taxonomy."}, categoryValues...),
	def(Definition{Key: "entityType", Label: "Entity Type", Kind: KindEnum,
		Description: "Type of an affected/involved entity."},
		v("PERSON", "Person", "individual", "people"),
		v("ORGANIZATION", "Organization", "organisation", "company", "corporation", "firm", "business"),
		v("GOVERNMENT", "Government", "agency", "ministry", "regulator", "state body", "public sector"),
		v("LOCATION", "Location", "place", "geography"),
		v("FACILITY", "Facility", "infrastructure", "plant", "building"),
		v("PRODUCT", "Product", "service", "brand"),
		v("EVENT", "Event", "incident"),
		v("OTHER", "Other", "misc", "uncategorized", "uncategorised"),
	),
	def(Definition{Key: "entity", Label: "Affected Entity", Kind: KindText,
		Description: "Name of an affected/involved entity (typed via entityType)."}),
}

// industryValues is the closed industry vocabulary.
var industryValues = []Value{
	v("TECHNOLOGY", "Technology", "tech", "it"),
	v("ARTIFICIAL_INTELLIGENCE", "Artificial Intelligence", "ai", "machine learning", "ml"),
	v("SOFTWARE", "Software", "saas", "software engineering"),
	v("CYBERSECURITY", "Cybersecurity", "security", "infosec"),
	v("CLOUD", "Cloud Computing", "cloud"),
	v("SEMICONDUCTORS", "Semiconductors", "semiconductor", "chips", "chip"),
	v("TELECOM", "Telecommunications", "telecom", "telco", "5g"),
	v("MANUFACTURING", "Manufacturing", "industrial", "factory"),
	v("AUTOMOTIVE", "Automotive", "auto", "carmaker", "cars", "ev", "electric vehicles"),
	v("AEROSPACE", "Aerospace", "space"),
	v("AVIATION", "Aviation", "airline", "airlines"),
	v("MARITIME", "Maritime", "shipping", "freight"),
	v("DEFENSE", "Defense", "defence", "military"),
	v("BANKING", "Banking", "bank", "banks"),
	v("FINANCE", "Finance", "financial", "financial services"),
	v("FINTECH", "FinTech", "digital payments", "payments"),
	v("INSURANCE", "Insurance", "insurer"),
	v("RETAIL", "Retail", "consumer goods"),
	v("ECOMMERCE", "E-commerce", "ecommerce", "online retail"),
	v("LOGISTICS", "Logistics", "supply chain", "supply-chain"),
	v("ENERGY", "Energy", "oil", "gas", "oil & gas", "power", "utilities"),
	v("RENEWABLES", "Renewable Energy", "renewables", "solar", "wind", "clean energy"),
	v("HEALTHCARE", "Healthcare", "health", "medical", "hospitals"),
	v("PHARMACEUTICALS", "Pharmaceuticals", "pharma", "drugs"),
	v("BIOTECH", "Biotechnology", "biotech", "life sciences"),
	v("AGRICULTURE", "Agriculture", "farming", "agri", "agritech"),
	v("FOOD_BEVERAGE", "Food & Beverage", "food", "beverage", "fmcg"),
	v("REAL_ESTATE", "Real Estate", "property", "realty"),
	v("CONSTRUCTION", "Construction", "infrastructure"),
	v("MINING", "Mining", "metals", "minerals"),
	v("CHEMICALS", "Chemicals", "chemical"),
	v("MEDIA", "Media", "press", "publishing", "broadcast"),
	v("ENTERTAINMENT", "Entertainment", "film", "music", "gaming", "sports business"),
	v("EDUCATION", "Education", "edtech", "schools", "universities"),
	v("HOSPITALITY", "Hospitality", "travel", "tourism", "hotels"),
	v("TRANSPORTATION", "Transportation", "transport", "rail", "mobility"),
	v("GOVERNMENT", "Government & Public Sector", "public sector", "gov"),
	v("NONPROFIT", "Nonprofit / NGO", "ngo", "non-profit", "charity"),
	v("OTHER", "Other", "general", "misc"),
}

// categoryValues mirrors the closed taxonomy (single source of truth) so topic
// categories are part of the same dictionary.
var categoryValues = func() []Value {
	nodes := taxonomy.Flatten(taxonomy.Taxonomy)
	out := make([]Value, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, Value{Code: n.Code, Label: n.Label})
	}
	return out
}()

var byKey = func() map[string]Definition {
	m := make(map[string]Definition, len(registry))
	for _, d := range registry {
		m[d.Key] = d
	}
	return m
}()

// Definitions returns the closed attribute dictionary in registration order.
func Definitions() []Definition { return registry }

// Lookup returns the definition for a key.
func Lookup(key string) (Definition, bool) {
	d, ok := byKey[key]
	return d, ok
}

// Values returns the controlled vocabulary for an enum/tagset attribute.
func (d Definition) Values() []Value { return d.values }

// Codes returns just the canonical codes of an enum/tagset attribute, in order.
func (d Definition) Codes() []string {
	out := make([]string, len(d.values))
	for i, val := range d.values {
		out[i] = val.Code
	}
	return out
}

// Normalize maps a raw extracted value to a canonical vocabulary code for
// enum/tagset attributes, accepting the code, label or any alias
// (case-insensitive, whitespace-trimmed). Returns ok=false for out-of-vocabulary
// values. For non-enum kinds it always returns ok=false (use the kind-specific
// helpers instead).
func (d Definition) Normalize(raw string) (string, bool) {
	if d.index == nil {
		return "", false
	}
	code, ok := d.index[strings.ToLower(strings.TrimSpace(raw))]
	return code, ok
}

// ClampScalar bounds v to the definition's [Min, Max] range.
func (d Definition) ClampScalar(n float64) float64 {
	if n < d.Min {
		return d.Min
	}
	if n > d.Max {
		return d.Max
	}
	return n
}

// NormalizeCountry resolves a raw country (ISO alpha-2 code or country name) to
// its canonical uppercase ISO alpha-2 code, or ok=false if unknown.
func NormalizeCountry(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	if c, ok := countries.Get(raw); ok { // case-insensitive code lookup
		return c.Code, true
	}
	lower := strings.ToLower(raw)
	for _, c := range countries.All() {
		if strings.ToLower(c.Name) == lower {
			return c.Code, true
		}
	}
	return "", false
}
