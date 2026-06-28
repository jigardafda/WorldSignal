// Package sources builds and validates the global news/knowledge source catalog.
//
// A Candidate is a proposed feed with rich metadata. The Validator fetches each
// candidate, parses it, checks freshness, and produces a Result; only candidates
// that pass validation are seeded into the database. See GOAL_GLOBAL_SOURCES.md.
package sources

import (
	"net/url"
	"strings"
)

// Candidate is a proposed source with the full metadata set from the spec.
type Candidate struct {
	Name            string         `json:"name"`
	FeedURL         string         `json:"feedUrl"`
	WebsiteURL      string         `json:"websiteUrl,omitempty"`
	Country         string         `json:"country,omitempty"` // ISO 3166-1 alpha-2 (or "" for global)
	Region          string         `json:"region,omitempty"`
	GeographicScope string         `json:"geographicScope,omitempty"` // GLOBAL|CONTINENTAL|REGIONAL|NATIONAL|STATE|CITY
	Languages       []string       `json:"languages,omitempty"`
	Category        string         `json:"category,omitempty"`
	Industry        string         `json:"industry,omitempty"`
	Subcategory     string         `json:"subcategory,omitempty"`
	Publisher       string         `json:"publisher,omitempty"`
	OrgType         string         `json:"orgType,omitempty"`    // GOVERNMENT|PUBLIC|PRIVATE|INDEPENDENT
	SourceType      string         `json:"sourceType,omitempty"` // RSS|ATOM|AGGREGATOR|GOVERNMENT_FEED|SECURITY_ADVISORY|RESEARCH_FEED|PRESS_RELEASE
	OfficialFeed    bool           `json:"officialFeed"`
	Priority        int            `json:"priority"`
	Credibility     float64        `json:"credibility"`
	Tags            []string       `json:"tags,omitempty"`
	Keywords        []string       `json:"keywords,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

// normalizedURL is the dedup key: lowercased, trailing slash trimmed.
func normalizedURL(raw string) string {
	return strings.TrimRight(strings.ToLower(strings.TrimSpace(raw)), "/")
}

// host returns the lowercased host of a URL, or "" if unparseable.
func host(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimPrefix(u.Host, "www."))
}

// dedup removes candidates sharing a normalized feed URL (first wins).
func dedup(in []Candidate) []Candidate {
	seen := make(map[string]bool, len(in))
	out := make([]Candidate, 0, len(in))
	for _, c := range in {
		k := normalizedURL(c.FeedURL)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, c)
	}
	return out
}
