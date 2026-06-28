// Package taxonomy ports backend/src/taxonomy/taxonomy.ts. The closed vocabulary
// the LLM and heuristic classifier are constrained to. JSON serialization is
// byte-compatible with the TS object-literal order: domains emit {code,label,
// children}; leaves emit {code,label,keywords} (keywords present even when empty).
package taxonomy

import (
	"bytes"

	"github.com/worldsignal/backend/internal/jsonx"
)

// Node is a taxonomy entry. Keywords is non-nil exactly for leaf nodes (matching
// the TS data where every leaf sets `keywords` and domains never do); Children is
// non-nil for domains.
type Node struct {
	Code     string
	Label    string
	Keywords []string
	Children []Node
}

// MarshalJSON emits keys in the fixed order code,label,keywords?,children? — a
// node never has both keywords and children, so this matches both TS shapes.
func (n Node) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	code, _ := jsonx.Marshal(n.Code)
	label, _ := jsonx.Marshal(n.Label)
	b.WriteString(`"code":`)
	b.Write(code)
	b.WriteString(`,"label":`)
	b.Write(label)
	if n.Keywords != nil {
		kw, err := jsonx.Marshal(n.Keywords)
		if err != nil {
			return nil, err
		}
		b.WriteString(`,"keywords":`)
		b.Write(kw)
	}
	if n.Children != nil {
		ch, err := jsonx.Marshal(n.Children)
		if err != nil {
			return nil, err
		}
		b.WriteString(`,"children":`)
		b.Write(ch)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

func leaf(code, label string, keywords ...string) Node {
	if keywords == nil {
		keywords = []string{}
	}
	return Node{Code: code, Label: label, Keywords: keywords}
}

func domain(code, label string, children ...Node) Node {
	return Node{Code: code, Label: label, Children: children}
}

// Taxonomy is the closed WorldSignal taxonomy tree.
var Taxonomy = []Node{
	domain("POLITICS", "Politics",
		leaf("POLITICS.ELECTIONS", "Elections", "election", "ballot", "vote", "poll", "candidate", "campaign"),
		leaf("POLITICS.POLICY", "Policy", "policy", "legislation", "bill", "reform", "parliament", "congress"),
		leaf("POLITICS.DIPLOMACY", "Diplomacy", "diplomacy", "summit", "treaty", "sanction", "embassy", "foreign minister"),
	),
	domain("ECONOMY", "Economy",
		leaf("ECONOMY.INFLATION", "Inflation", "inflation", "cpi", "consumer price", "cost of living"),
		leaf("ECONOMY.INTEREST_RATES", "Interest Rates", "interest rate", "central bank", "rate hike", "basis points", "federal reserve", "rbi"),
		leaf("ECONOMY.MARKETS", "Markets", "stock market", "shares", "index", "nasdaq", "nifty", "sensex", "commodities", "crypto", "bitcoin"),
		leaf("ECONOMY.JOBS", "Jobs & Employment", "unemployment", "jobs report", "hiring", "labor market", "payroll"),
	),
	domain("BUSINESS", "Business",
		leaf("BUSINESS.EARNINGS", "Earnings", "earnings", "quarterly results", "revenue", "profit", "guidance"),
		leaf("BUSINESS.MA", "Mergers & Acquisitions", "acquisition", "merger", "takeover", "acquires", "buyout"),
		leaf("BUSINESS.FUNDING", "Funding", "funding", "series a", "series b", "raised", "venture", "valuation", "ipo"),
		leaf("BUSINESS.LAYOFFS", "Layoffs", "layoff", "job cuts", "restructuring", "redundancies"),
	),
	domain("TECHNOLOGY", "Technology",
		leaf("TECHNOLOGY.AI", "Artificial Intelligence", "artificial intelligence", "ai model", "machine learning", "llm", "openai", "chatbot", "neural"),
		leaf("TECHNOLOGY.CYBERSECURITY", "Cybersecurity", "data breach", "ransomware", "vulnerability", "hacked", "cyberattack", "malware", "cve"),
		leaf("TECHNOLOGY.PRODUCT", "Product Launch", "launches", "unveils", "release", "new device", "announced"),
	),
	domain("DISASTER", "Disaster",
		leaf("DISASTER.EARTHQUAKE", "Earthquake", "earthquake", "magnitude", "seismic", "tremor", "aftershock"),
		leaf("DISASTER.FLOOD", "Flood", "flood", "flooding", "inundation", "deluge"),
		leaf("DISASTER.CYCLONE", "Cyclone / Storm", "cyclone", "hurricane", "typhoon", "storm", "tornado"),
		leaf("DISASTER.WILDFIRE", "Wildfire", "wildfire", "bushfire", "forest fire", "blaze"),
	),
	domain("PUBLIC_HEALTH", "Public Health",
		leaf("PUBLIC_HEALTH.OUTBREAK", "Disease Outbreak", "outbreak", "epidemic", "pandemic", "virus", "infection", "cases surge"),
		leaf("PUBLIC_HEALTH.DRUG", "Drug / Treatment", "drug approval", "vaccine", "clinical trial", "fda approval", "treatment"),
	),
	domain("LEGAL", "Legal",
		leaf("LEGAL.COURT_RULING", "Court Ruling", "court", "ruling", "verdict", "supreme court", "judge", "lawsuit"),
		leaf("LEGAL.REGULATION", "Regulation", "regulation", "regulator", "compliance", "antitrust", "ban", "fine"),
	),
	domain("CONFLICT", "Conflict & Security",
		leaf("CONFLICT.WAR", "War / Armed Conflict", "war", "military", "airstrike", "troops", "ceasefire", "invasion", "missile"),
		leaf("CONFLICT.TERRORISM", "Terrorism", "terror", "bombing", "attack", "explosion", "shooting"),
	),
	domain("SPORTS", "Sports",
		leaf("SPORTS.RESULT", "Match Result", "wins", "defeats", "final", "tournament", "championship", "match"),
		leaf("SPORTS.TRANSFER", "Transfer", "transfer", "signs", "signing", "contract", "deal"),
	),
	domain("GENERAL", "General",
		leaf("GENERAL.OTHER", "Other / Uncategorized"),
	),
}

// FallbackCode is used when nothing else matches.
const FallbackCode = "GENERAL.OTHER"

// Flatten returns every node (domains + leaves), depth-first, matching flattenTaxonomy.
func Flatten(nodes []Node) []Node {
	var out []Node
	for _, n := range nodes {
		out = append(out, n)
		if n.Children != nil {
			out = append(out, Flatten(n.Children)...)
		}
	}
	return out
}

// LeafTags returns only assignable leaf nodes (no children).
func LeafTags() []Node {
	var out []Node
	for _, n := range Flatten(Taxonomy) {
		if n.Children == nil {
			out = append(out, n)
		}
	}
	return out
}

// ValidCodes is the set of every code (domains + leaves), used to validate LLM output.
var ValidCodes = func() map[string]struct{} {
	m := map[string]struct{}{}
	for _, n := range Flatten(Taxonomy) {
		m[n.Code] = struct{}{}
	}
	return m
}()
