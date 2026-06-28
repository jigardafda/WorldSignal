package sources

import (
	"fmt"
	"strings"
)

// edition is a Google News locale edition (hl=language, gl=country).
type edition struct {
	country string // display name
	cc      string // ISO 3166-1 alpha-2 (gl)
	lang    string // hl, e.g. "en-US" or "pt-BR"
	langs   []string
	region  string
}

// gnewsTopic maps a Google News topic section to our category/industry.
type gnewsTopic struct {
	topic    string
	category string
	industry string
	tags     []string
}

var gnewsTopics = []gnewsTopic{
	{"WORLD", "World", "", []string{"international", "breaking-news"}},
	{"NATION", "National", "", []string{"national", "politics"}},
	{"BUSINESS", "Business", "Finance", []string{"business", "economy", "financial"}},
	{"TECHNOLOGY", "Technology", "Technology", []string{"technology", "consumer"}},
	{"SCIENCE", "Science", "Science", []string{"science", "research", "scientific"}},
	{"HEALTH", "Health", "Healthcare", []string{"health", "public-health"}},
	{"SPORTS", "Sports", "Sports", []string{"sports"}},
	{"ENTERTAINMENT", "Entertainment", "Media", []string{"entertainment", "consumer"}},
}

// editions enumerates Google News locale editions across every region and all
// 30 required languages. Each yields a validated top-stories feed plus topic
// sections; non-existent sections fail validation and are dropped.
var editions = []edition{
	// ── North America ──
	{"United States", "US", "en-US", []string{"en"}, "North America"},
	{"Canada", "CA", "en-CA", []string{"en"}, "North America"},
	{"Canada (Français)", "CA", "fr-CA", []string{"fr"}, "North America"},
	{"Mexico", "MX", "es-419", []string{"es"}, "North America"},
	// ── Central America & Caribbean ──
	{"Cuba", "CU", "es-419", []string{"es"}, "Caribbean"},
	{"Costa Rica", "CR", "es-419", []string{"es"}, "Central America"},
	{"Guatemala", "GT", "es-419", []string{"es"}, "Central America"},
	{"Panama", "PA", "es-419", []string{"es"}, "Central America"},
	// ── South America ──
	{"Brazil", "BR", "pt-BR", []string{"pt"}, "South America"},
	{"Argentina", "AR", "es-419", []string{"es"}, "South America"},
	{"Chile", "CL", "es-419", []string{"es"}, "South America"},
	{"Colombia", "CO", "es-419", []string{"es"}, "South America"},
	{"Peru", "PE", "es-419", []string{"es"}, "South America"},
	{"Venezuela", "VE", "es-419", []string{"es"}, "South America"},
	{"Ecuador", "EC", "es-419", []string{"es"}, "South America"},
	{"Bolivia", "BO", "es-419", []string{"es"}, "South America"},
	{"Uruguay", "UY", "es-419", []string{"es"}, "South America"},
	{"Paraguay", "PY", "es-419", []string{"es"}, "South America"},
	// ── Europe (Western) ──
	{"United Kingdom", "GB", "en-GB", []string{"en"}, "Europe"},
	{"Ireland", "IE", "en-IE", []string{"en"}, "Europe"},
	{"France", "FR", "fr-FR", []string{"fr"}, "Europe"},
	{"Belgium (FR)", "BE", "fr-BE", []string{"fr"}, "Europe"},
	{"Belgium (NL)", "BE", "nl-BE", []string{"nl"}, "Europe"},
	{"Netherlands", "NL", "nl-NL", []string{"nl"}, "Europe"},
	{"Germany", "DE", "de-DE", []string{"de"}, "Europe"},
	{"Austria", "AT", "de-AT", []string{"de"}, "Europe"},
	{"Switzerland (DE)", "CH", "de-CH", []string{"de"}, "Europe"},
	{"Switzerland (FR)", "CH", "fr-CH", []string{"fr"}, "Europe"},
	{"Italy", "IT", "it-IT", []string{"it"}, "Europe"},
	{"Spain", "ES", "es-ES", []string{"es"}, "Europe"},
	{"Portugal", "PT", "pt-PT", []string{"pt"}, "Europe"},
	// ── Europe (Nordic) ──
	{"Sweden", "SE", "sv-SE", []string{"sv"}, "Europe"},
	{"Norway", "NO", "no-NO", []string{"no"}, "Europe"},
	{"Denmark", "DK", "da-DK", []string{"da"}, "Europe"},
	{"Finland", "FI", "fi-FI", []string{"fi"}, "Europe"},
	// ── Europe (Central & Eastern) ──
	{"Poland", "PL", "pl-PL", []string{"pl"}, "Europe"},
	{"Czechia", "CZ", "cs-CZ", []string{"cs"}, "Europe"},
	{"Slovakia", "SK", "sk-SK", []string{"sk"}, "Europe"},
	{"Hungary", "HU", "hu-HU", []string{"hu"}, "Europe"},
	{"Romania", "RO", "ro-RO", []string{"ro"}, "Europe"},
	{"Bulgaria", "BG", "bg-BG", []string{"bg"}, "Europe"},
	{"Greece", "GR", "el-GR", []string{"el"}, "Europe"},
	{"Ukraine (UK)", "UA", "uk-UA", []string{"uk"}, "Europe"},
	{"Ukraine (RU)", "UA", "ru-UA", []string{"ru"}, "Europe"},
	{"Russia", "RU", "ru-RU", []string{"ru"}, "Europe"},
	{"Serbia", "RS", "sr-RS", []string{"sr"}, "Europe"},
	{"Croatia", "HR", "hr-HR", []string{"hr"}, "Europe"},
	{"Slovenia", "SI", "sl-SI", []string{"sl"}, "Europe"},
	{"Lithuania", "LT", "lt-LT", []string{"lt"}, "Europe"},
	{"Latvia", "LV", "lv-LV", []string{"lv"}, "Europe"},
	{"Estonia", "EE", "et-EE", []string{"et"}, "Europe"},
	// ── Middle East ──
	{"Israel (HE)", "IL", "he-IL", []string{"he"}, "Middle East"},
	{"Saudi Arabia", "SA", "ar", []string{"ar"}, "Middle East"},
	{"UAE", "AE", "ar", []string{"ar"}, "Middle East"},
	{"Egypt", "EG", "ar", []string{"ar"}, "Middle East"},
	{"Lebanon", "LB", "ar", []string{"ar"}, "Middle East"},
	{"Türkiye", "TR", "tr-TR", []string{"tr"}, "Middle East"},
	{"Iran", "IR", "fa-IR", []string{"fa"}, "Middle East"},
	// ── Africa ──
	{"Nigeria", "NG", "en-NG", []string{"en"}, "Africa"},
	{"South Africa", "ZA", "en-ZA", []string{"en"}, "Africa"},
	{"Kenya", "KE", "en-KE", []string{"en", "sw"}, "Africa"},
	{"Tanzania (SW)", "TZ", "sw-TZ", []string{"sw"}, "Africa"},
	{"Ghana", "GH", "en-GH", []string{"en"}, "Africa"},
	{"Morocco (FR)", "MA", "fr-MA", []string{"fr", "ar"}, "Africa"},
	{"Senegal (FR)", "SN", "fr-SN", []string{"fr"}, "Africa"},
	{"Ethiopia", "ET", "en-ET", []string{"en"}, "Africa"},
	{"Uganda", "UG", "en-UG", []string{"en"}, "Africa"},
	{"Zimbabwe", "ZW", "en-ZW", []string{"en"}, "Africa"},
	// ── South Asia ──
	{"India (EN)", "IN", "en-IN", []string{"en"}, "South Asia"},
	{"India (हिन्दी)", "IN", "hi-IN", []string{"hi"}, "South Asia"},
	{"India (বাংলা)", "IN", "bn-IN", []string{"bn"}, "South Asia"},
	{"India (தமிழ்)", "IN", "ta-IN", []string{"ta"}, "South Asia"},
	{"India (తెలుగు)", "IN", "te-IN", []string{"te"}, "South Asia"},
	{"India (मराठी)", "IN", "mr-IN", []string{"mr"}, "South Asia"},
	{"India (ગુજરાતી)", "IN", "gu-IN", []string{"gu"}, "South Asia"},
	{"India (മലയാളം)", "IN", "ml-IN", []string{"ml"}, "South Asia"},
	{"India (ಕನ್ನಡ)", "IN", "kn-IN", []string{"kn"}, "South Asia"},
	{"Pakistan (EN)", "PK", "en-PK", []string{"en"}, "South Asia"},
	{"Pakistan (اردو)", "PK", "ur-PK", []string{"ur"}, "South Asia"},
	{"Bangladesh", "BD", "bn-BD", []string{"bn"}, "South Asia"},
	{"Sri Lanka", "LK", "en-LK", []string{"en"}, "South Asia"},
	{"Nepal", "NP", "en-NP", []string{"en"}, "South Asia"},
	// ── Southeast Asia ──
	{"Indonesia", "ID", "id-ID", []string{"id"}, "Southeast Asia"},
	{"Malaysia", "MY", "ms-MY", []string{"ms", "en"}, "Southeast Asia"},
	{"Philippines", "PH", "en-PH", []string{"en"}, "Southeast Asia"},
	{"Singapore", "SG", "en-SG", []string{"en"}, "Southeast Asia"},
	{"Thailand", "TH", "th-TH", []string{"th"}, "Southeast Asia"},
	{"Vietnam", "VN", "vi-VN", []string{"vi"}, "Southeast Asia"},
	// ── East Asia ──
	{"Japan", "JP", "ja-JP", []string{"ja"}, "East Asia"},
	{"South Korea", "KR", "ko-KR", []string{"ko"}, "East Asia"},
	{"China", "CN", "zh-CN", []string{"zh"}, "East Asia"},
	{"Taiwan", "TW", "zh-TW", []string{"zh"}, "East Asia"},
	{"Hong Kong", "HK", "zh-HK", []string{"zh"}, "East Asia"},
	// ── Oceania ──
	{"Australia", "AU", "en-AU", []string{"en"}, "Oceania"},
	{"New Zealand", "NZ", "en-NZ", []string{"en"}, "Oceania"},
}

// gnewsURL builds a Google News RSS URL for a given edition and path.
func gnewsURL(path string, e edition) string {
	hl := e.lang
	gl := e.cc
	ceid := gl + ":" + langPrefix(hl)
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	return fmt.Sprintf("https://news.google.com/rss%s%shl=%s&gl=%s&ceid=%s", path, sep, hl, gl, ceid)
}

// langPrefix returns the bare language part of an hl value ("pt-BR" → "pt").
func langPrefix(hl string) string {
	if i := strings.IndexByte(hl, '-'); i > 0 {
		return hl[:i]
	}
	return hl
}

// GNewsCandidates expands the edition matrix into top-stories + topic feeds.
func GNewsCandidates() []Candidate {
	var out []Candidate
	for _, e := range editions {
		// Top stories.
		out = append(out, Candidate{
			Name:            "Google News — " + e.country + " (Top stories)",
			FeedURL:         gnewsURL("", e),
			WebsiteURL:      "https://news.google.com",
			Country:         e.cc,
			Region:          e.region,
			GeographicScope: "NATIONAL",
			Languages:       e.langs,
			Category:        "General",
			Publisher:       "Google News",
			OrgType:         "PRIVATE",
			SourceType:      "AGGREGATOR",
			OfficialFeed:    false,
			Priority:        2,
			Credibility:     0.7,
			Tags:            []string{"aggregator", "breaking-news", "national", "international"},
			Metadata:        map[string]any{"discoverySource": "google-news-edition", "edition": e.lang},
		})
		// Topic sections.
		for _, t := range gnewsTopics {
			tags := append([]string{"aggregator", "national"}, t.tags...)
			out = append(out, Candidate{
				Name:            "Google News — " + e.country + " (" + t.category + ")",
				FeedURL:         gnewsURL("/headlines/section/topic/"+t.topic, e),
				WebsiteURL:      "https://news.google.com",
				Country:         e.cc,
				Region:          e.region,
				GeographicScope: "NATIONAL",
				Languages:       e.langs,
				Category:        t.category,
				Industry:        t.industry,
				Publisher:       "Google News",
				OrgType:         "PRIVATE",
				SourceType:      "AGGREGATOR",
				OfficialFeed:    false,
				Priority:        3,
				Credibility:     0.68,
				Tags:            tags,
				Metadata:        map[string]any{"discoverySource": "google-news-topic", "topic": t.topic, "edition": e.lang},
			})
		}
	}
	return out
}
