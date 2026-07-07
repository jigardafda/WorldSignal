// Package relevance ranks enriched signals for a subscriber's weighted interest
// graph. It is deterministic and dependency-free so the scoring can be unit
// tested and reused by the API, digest and streaming paths.
//
// Score model: score = (2·match + quality) · recency
//   - match:    Σ weights of interests the signal matches + keyword hits. This is
//     the personal component; it dominates when interests are set.
//   - quality:  the signal's intrinsic newsworthiness (influence/severity/
//     relevance/confidence), 0..1 — so an empty profile still surfaces
//     the most important recent news.
//   - recency:  time decay in [0.3, 1] so fresh events outrank stale ones without
//     old-but-relevant items vanishing entirely.
package relevance

import (
	"math"
	"sort"
	"strings"
)

// Profile is a subscriber's weighted interest graph. Interests keys are
// "dimension:value" — tag:<code>, entity:<name>, country:<ISO2>, region:<name>,
// sentiment:<POSITIVE|NEUTRAL|NEGATIVE>. A larger weight means more important.
// Keywords match the signal's title/summary text.
type Profile struct {
	Interests map[string]float64
	Keywords  []string
}

// Signal is the scorable projection of an enriched signal.
type Signal struct {
	ID         string
	Title      string
	Summary    string
	EventType  string   // primary category code, e.g. DISASTER.EARTHQUAKE
	Tags       []string // all assigned category codes
	Country    string   // ISO 3166-1 alpha-2
	Region     string
	Entities   []string
	Sentiment  string  // POSITIVE | NEUTRAL | NEGATIVE
	Influence  string  // LOW | MEDIUM | HIGH | CRITICAL
	Severity   string  // LOW | MEDIUM | HIGH | CRITICAL
	Relevance  float64 // 0..1
	Confidence float64 // 0..1
	AgeHours   float64 // hours since the signal was last seen / published
}

// Scored is a signal with its computed relevance and the reasons it ranked.
type Scored struct {
	Signal
	Score   float64  `json:"score"`
	Reasons []string `json:"reasons"`
}

// recencyHalfLife controls the decay: a signal this many hours old scores at the
// midpoint between the floor (0.3) and full weight (1.0).
const recencyHalfLife = 48.0

var rankValue = map[string]float64{"LOW": 0.25, "MEDIUM": 0.5, "HIGH": 0.75, "CRITICAL": 1.0}

// Score computes the relevance of a signal for a profile.
func Score(p Profile, s Signal) Scored {
	var match float64
	var reasons []string

	for key, w := range p.Interests {
		if w == 0 {
			continue
		}
		dim, val, ok := splitInterest(key)
		if !ok || !matches(dim, val, s) {
			continue
		}
		match += w
		reasons = append(reasons, key)
	}

	hay := strings.ToLower(s.Title + " " + s.Summary)
	for _, kw := range p.Keywords {
		kw = strings.ToLower(strings.TrimSpace(kw))
		if kw != "" && strings.Contains(hay, kw) {
			match++
			reasons = append(reasons, "keyword:"+kw)
		}
	}

	quality := 0.4*rankValue[s.Influence] + 0.3*rankValue[s.Severity] +
		0.2*clamp01(s.Relevance) + 0.1*clamp01(s.Confidence)

	recency := recencyFactor(s.AgeHours)
	score := (2*match + quality) * recency

	sort.Strings(reasons) // stable, deterministic ordering
	return Scored{Signal: s, Score: score, Reasons: reasons}
}

// Rank scores every signal and returns them sorted by score descending. Ties keep
// input order (stable), so callers can pre-order by recency for deterministic ties.
func Rank(p Profile, sigs []Signal) []Scored {
	out := make([]Scored, len(sigs))
	for i, s := range sigs {
		out[i] = Score(p, s)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

// splitInterest parses a "dimension:value" key.
func splitInterest(key string) (dim, val string, ok bool) {
	i := strings.IndexByte(key, ':')
	if i <= 0 || i == len(key)-1 {
		return "", "", false
	}
	return strings.ToLower(key[:i]), key[i+1:], true
}

// matches reports whether a signal satisfies one interest dimension/value.
func matches(dim, val string, s Signal) bool {
	switch dim {
	case "tag":
		// Exact leaf match, or a domain-level interest (DISASTER) matching any of
		// its leaves (DISASTER.EARTHQUAKE).
		if tagMatch(s.EventType, val) {
			return true
		}
		for _, t := range s.Tags {
			if tagMatch(t, val) {
				return true
			}
		}
	case "country":
		return strings.EqualFold(s.Country, val)
	case "region":
		return strings.EqualFold(s.Region, val)
	case "sentiment":
		return strings.EqualFold(s.Sentiment, val)
	case "entity":
		for _, e := range s.Entities {
			if strings.EqualFold(e, val) {
				return true
			}
		}
	}
	return false
}

// tagMatch is true when want equals code or is the domain prefix of code.
func tagMatch(code, want string) bool {
	if strings.EqualFold(code, want) {
		return true
	}
	// domain-level: want="DISASTER" matches code="DISASTER.EARTHQUAKE"
	return len(code) > len(want) && strings.EqualFold(code[:len(want)], want) && code[len(want)] == '.'
}

func recencyFactor(ageHours float64) float64 {
	if ageHours < 0 {
		ageHours = 0
	}
	// Exponential decay from 1.0 toward the 0.3 floor.
	const floor = 0.3
	return floor + (1-floor)*math.Exp(-ageHours/recencyHalfLife)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
