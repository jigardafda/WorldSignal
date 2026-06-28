package sources

import (
	"net/url"
	"strings"
)

// industry is one industry/topic vertical to cover via Google News search.
type industry struct {
	name  string // industry label
	query string // search query
	tags  []string
}

// industries enumerates the spec's industry/topic verticals (§3, §4, §8).
var industries = []industry{
	{"Technology", "technology", []string{"technology"}},
	{"Artificial Intelligence", "\"artificial intelligence\" OR \"machine learning\"", []string{"ai", "technology"}},
	{"Software Engineering", "\"software engineering\" OR programming", []string{"software", "technology"}},
	{"Cybersecurity", "cybersecurity OR \"data breach\"", []string{"security", "cybersecurity"}},
	{"Cloud Computing", "\"cloud computing\" OR AWS OR Azure", []string{"cloud", "technology"}},
	{"DevOps", "DevOps OR Kubernetes", []string{"devops", "software"}},
	{"Robotics", "robotics OR automation", []string{"robotics", "technology"}},
	{"Semiconductors", "semiconductor OR chips OR TSMC", []string{"semiconductor", "technology", "supply-chain"}},
	{"Manufacturing", "manufacturing industry", []string{"manufacturing", "industrial"}},
	{"Automotive", "automotive industry OR carmaker", []string{"automotive"}},
	{"Electric Vehicles", "\"electric vehicle\" OR EV", []string{"automotive", "energy", "climate"}},
	{"Aerospace", "aerospace OR aviation industry", []string{"aerospace", "aviation"}},
	{"Aviation", "aviation OR airline", []string{"aviation"}},
	{"Maritime", "maritime OR shipping OR \"cargo ship\"", []string{"maritime", "logistics"}},
	{"Defense", "defense OR military procurement", []string{"defense", "military"}},
	{"Banking", "banking OR \"central bank\"", []string{"banking", "financial"}},
	{"FinTech", "fintech OR \"digital payments\"", []string{"fintech", "financial", "technology"}},
	{"Insurance", "insurance industry", []string{"insurance", "financial"}},
	{"Retail", "retail industry", []string{"retail", "consumer"}},
	{"E-commerce", "e-commerce OR online retail", []string{"ecommerce", "retail", "consumer"}},
	{"Logistics", "logistics OR \"supply chain\"", []string{"logistics", "supply-chain"}},
	{"Shipping", "shipping OR freight", []string{"shipping", "logistics"}},
	{"Supply Chain", "\"supply chain\"", []string{"supply-chain", "logistics"}},
	{"Real Estate", "\"real estate\" market", []string{"real-estate", "financial"}},
	{"Construction", "construction industry", []string{"construction", "industrial"}},
	{"Healthcare", "healthcare OR hospitals", []string{"healthcare", "health"}},
	{"Biotechnology", "biotechnology OR biotech", []string{"biotech", "science", "healthcare"}},
	{"Pharmaceuticals", "pharmaceutical OR pharma", []string{"pharma", "healthcare"}},
	{"Agriculture", "agriculture OR farming", []string{"agriculture"}},
	{"Food", "food industry", []string{"food", "consumer"}},
	{"Telecom", "telecom OR 5G", []string{"telecom", "technology"}},
	{"Media", "media industry", []string{"media"}},
	{"Gaming", "\"video game\" OR gaming industry", []string{"gaming", "entertainment", "consumer"}},
	{"Entertainment", "entertainment industry", []string{"entertainment", "media"}},
	{"Energy", "energy industry", []string{"energy"}},
	{"Oil & Gas", "\"oil and gas\" OR crude oil", []string{"oil-gas", "energy", "commodities"}},
	{"Renewable Energy", "\"renewable energy\" OR solar OR wind power", []string{"renewables", "energy", "climate"}},
	{"Mining", "mining industry", []string{"mining", "commodities"}},
	{"Education", "education sector", []string{"education"}},
	{"Government", "government policy", []string{"government", "political"}},
	{"Legal", "legal industry OR law firm", []string{"legal"}},
	{"Climate", "climate change", []string{"climate", "environment"}},
	{"ESG", "ESG investing OR sustainability", []string{"esg", "financial", "climate"}},
	{"Venture Capital", "\"venture capital\"", []string{"vc", "startup", "financial"}},
	{"Startups", "startup funding", []string{"startup", "technology"}},
	{"Private Equity", "\"private equity\"", []string{"private-equity", "financial"}},
	{"Public Markets", "\"stock market\" OR equities", []string{"markets", "financial"}},
	{"Commodities", "commodities OR gold price", []string{"commodities", "financial"}},
	{"Cryptocurrency", "cryptocurrency OR bitcoin", []string{"crypto", "financial", "technology"}},
	{"Travel", "travel industry", []string{"travel"}},
	{"Hospitality", "hospitality OR hotels", []string{"hospitality", "travel"}},
	{"Sports", "sports", []string{"sports"}},
	{"Space", "space OR \"space agency\" OR rocket launch", []string{"space", "science"}},
	{"Disaster & Emergency", "earthquake OR hurricane OR \"natural disaster\"", []string{"disaster", "emergency", "alerts"}},
	{"Public Health", "\"public health\" OR outbreak", []string{"public-health", "health", "alerts"}},
	{"Elections", "election OR \"election results\"", []string{"elections", "political"}},
	{"Environment", "environment OR pollution", []string{"environment", "climate"}},
}

// industryLocales are the locales used to broaden industry coverage across major
// world languages (the spec's multilingual requirement, §5).
var industryLocales = []struct {
	hl, gl, lang string
}{
	{"en-US", "US", "en"},
	{"es-419", "MX", "es"},
	{"fr-FR", "FR", "fr"},
	{"de-DE", "DE", "de"},
	{"pt-BR", "BR", "pt"},
	{"ar", "EG", "ar"},
	{"hi-IN", "IN", "hi"},
	{"zh-CN", "CN", "zh"},
	{"ja-JP", "JP", "ja"},
	{"ru-RU", "RU", "ru"},
}

// keyMultilingualIndustries are duplicated across all industryLocales; the rest
// are English-only (to keep the candidate pool focused).
var keyMultilingualIndustries = map[string]bool{
	"Technology": true, "Artificial Intelligence": true, "Cybersecurity": true,
	"Business": true, "Energy": true, "Climate": true, "Healthcare": true,
	"Cryptocurrency": true, "Automotive": true, "Sports": true,
}

func gnewsSearchURL(query, hl, gl string) string {
	ceid := gl + ":" + langPrefix(hl)
	return "https://news.google.com/rss/search?q=" + url.QueryEscape(query) +
		"&hl=" + hl + "&gl=" + gl + "&ceid=" + ceid
}

// IndustryCandidates builds Google News search feeds per industry — English for
// all, plus multilingual variants for the key industries.
func IndustryCandidates() []Candidate {
	var out []Candidate
	for _, ind := range industries {
		locales := []struct{ hl, gl, lang string }{{"en-US", "US", "en"}}
		if keyMultilingualIndustries[ind.name] {
			locales = industryLocales
		}
		for _, loc := range locales {
			suffix := ""
			if loc.lang != "en" {
				suffix = " (" + strings.ToUpper(loc.lang) + ")"
			}
			out = append(out, Candidate{
				Name:            "Google News — " + ind.name + suffix,
				FeedURL:         gnewsSearchURL(ind.query, loc.hl, loc.gl),
				WebsiteURL:      "https://news.google.com",
				Country:         "",
				Region:          "Global",
				GeographicScope: "GLOBAL",
				Languages:       []string{loc.lang},
				Category:        "Industry",
				Industry:        ind.name,
				Publisher:       "Google News",
				OrgType:         "PRIVATE",
				SourceType:      "AGGREGATOR",
				OfficialFeed:    false,
				Priority:        3,
				Credibility:     0.66,
				Tags:            append([]string{"aggregator", "industry", "global"}, ind.tags...),
				Metadata:        map[string]any{"discoverySource": "google-news-search", "query": ind.query, "locale": loc.hl},
			})
		}
	}
	return out
}
