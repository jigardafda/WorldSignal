// Package textutil ports backend/src/lib/text.ts. Behavior must match the
// TypeScript implementation exactly so clustering/dedupe stay byte-compatible.
package textutil

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"
)

var stopwords = map[string]struct{}{}

func init() {
	for _, w := range []string{
		"the", "a", "an", "and", "or", "but", "of", "to", "in", "on", "for", "with",
		"at", "by", "from", "as", "is", "are", "was", "were", "be", "been", "has",
		"have", "had", "it", "its", "this", "that", "these", "those", "after", "over",
		"into", "amid", "says", "said", "new", "report", "reports",
	} {
		stopwords[w] = struct{}{}
	}
}

var (
	reStyle      = regexp.MustCompile(`(?is)<style.*?</style>`)
	reScript     = regexp.MustCompile(`(?is)<script.*?</script>`)
	reTag        = regexp.MustCompile(`<[^>]+>`)
	reWhitespace = regexp.MustCompile(`\s+`)
	reNonAlnum   = regexp.MustCompile(`[^a-z0-9\s]`)
	reSentence   = regexp.MustCompile(`[^.!?]+[.!?]+`)
)

// entity replacements applied in the same order as the TS chained .replace calls.
var entityReplacers = []struct {
	re   *regexp.Regexp
	with string
}{
	{regexp.MustCompile(`(?i)&nbsp;`), " "},
	{regexp.MustCompile(`(?i)&amp;`), "&"},
	{regexp.MustCompile(`(?i)&lt;`), "<"},
	{regexp.MustCompile(`(?i)&gt;`), ">"},
	{regexp.MustCompile(`(?i)&#39;|&apos;`), "'"},
	{regexp.MustCompile(`(?i)&quot;`), `"`},
}

// StripHtml mirrors stripHtml(): removes style/script/tags, decodes a small set
// of entities, collapses whitespace and trims.
func StripHtml(html string) string {
	if html == "" {
		return ""
	}
	s := reStyle.ReplaceAllString(html, " ")
	s = reScript.ReplaceAllString(s, " ")
	s = reTag.ReplaceAllString(s, " ")
	for _, r := range entityReplacers {
		s = r.re.ReplaceAllString(s, r.with)
	}
	s = reWhitespace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// NormalizeText mirrors normalizeText(): lowercase, non-alphanumerics → space,
// collapse whitespace, trim.
func NormalizeText(s string) string {
	s = strings.ToLower(s)
	s = reNonAlnum.ReplaceAllString(s, " ")
	s = reWhitespace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// TokenSet mirrors tokenSet(): normalized, length>2, non-stopword tokens.
func TokenSet(text string) map[string]struct{} {
	norm := NormalizeText(text)
	out := map[string]struct{}{}
	if norm == "" {
		return out
	}
	for _, t := range strings.Split(norm, " ") {
		if len(t) > 2 {
			if _, stop := stopwords[t]; !stop {
				out[t] = struct{}{}
			}
		}
	}
	return out
}

// TokenSetString mirrors tokenSetString(): sorted token set joined by spaces.
func TokenSetString(text string) string {
	set := TokenSet(text)
	toks := make([]string, 0, len(set))
	for t := range set {
		toks = append(toks, t)
	}
	sort.Strings(toks)
	return strings.Join(toks, " ")
}

func splitSet(s string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, t := range strings.Split(s, " ") {
		if t != "" {
			out[t] = struct{}{}
		}
	}
	return out
}

// Jaccard mirrors jaccard(): similarity between two space-separated token strings.
func Jaccard(aStr, bStr string) float64 {
	a := splitSet(aStr)
	b := splitSet(bStr)
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for t := range a {
		if _, ok := b[t]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	return float64(inter) / float64(union)
}

// ContentHash mirrors contentHash(): sha256 of normalized title + "\n" + first
// 5000 chars of normalized body. Normalized text is ASCII so byte/rune slicing
// is equivalent to the JS UTF-16 slice for this input.
func ContentHash(title, body string) string {
	nb := NormalizeText(body)
	if len(nb) > 5000 {
		nb = nb[:5000]
	}
	basis := NormalizeText(title) + "\n" + nb
	sum := sha256.Sum256([]byte(basis))
	return hex.EncodeToString(sum[:])
}

// FirstSentences mirrors firstSentences(): first n sentences of stripped text,
// joined and capped at 500 chars.
func FirstSentences(text string, n int) string {
	clean := StripHtml(text)
	sentences := reSentence.FindAllString(clean, -1)
	if len(sentences) == 0 {
		sentences = []string{clean}
	}
	if n < len(sentences) {
		sentences = sentences[:n]
	}
	for i, s := range sentences {
		sentences[i] = strings.TrimSpace(s)
	}
	out := strings.TrimSpace(strings.Join(sentences, " "))
	if len(out) > 500 {
		out = out[:500]
	}
	return out
}
