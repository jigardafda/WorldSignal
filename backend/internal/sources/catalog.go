package sources

// All returns the full de-duplicated candidate catalog: curated direct feeds,
// the Google News country/language/topic matrix, and industry search feeds.
func All() []Candidate {
	var out []Candidate
	out = append(out, CuratedCandidates()...)
	out = append(out, GNewsCandidates()...)
	out = append(out, IndustryCandidates()...)
	return dedup(out)
}

// Stats summarizes a candidate set for reporting.
type Stats struct {
	Total      int
	ByScope    map[string]int
	ByRegion   map[string]int
	ByCountry  map[string]int
	ByLanguage map[string]int
	ByIndustry map[string]int
	BySource   map[string]int // discoverySource
}

// Summarize tallies a candidate set across coverage dimensions.
func Summarize(cands []Candidate) Stats {
	s := Stats{
		Total: len(cands), ByScope: map[string]int{}, ByRegion: map[string]int{},
		ByCountry: map[string]int{}, ByLanguage: map[string]int{},
		ByIndustry: map[string]int{}, BySource: map[string]int{},
	}
	for _, c := range cands {
		s.ByScope[c.GeographicScope]++
		s.ByRegion[c.Region]++
		if c.Country != "" {
			s.ByCountry[c.Country]++
		}
		for _, l := range c.Languages {
			s.ByLanguage[l]++
		}
		if c.Industry != "" {
			s.ByIndustry[c.Industry]++
		}
		if c.Metadata != nil {
			if ds, ok := c.Metadata["discoverySource"].(string); ok {
				s.BySource[ds]++
			}
		}
	}
	return s
}
