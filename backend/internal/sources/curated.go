package sources

import "strings"

// cur is a compact curated-feed row; langs/tags are space-separated for brevity.
type cur struct {
	name, feed, web, cc, region, scope, cat, ind, pub, org string
	langs, tags                                            string
	prio                                                   int
	cred                                                   float64
}

// toCandidate expands a cur row, inferring SourceType from the URL and applying
// defaults (PRIVATE org, official direct feed).
func (c cur) toCandidate() Candidate {
	st := "RSS"
	if strings.Contains(c.feed, ".atom") || strings.Contains(c.feed, "/atom") {
		st = "ATOM"
	}
	org := c.org
	if org == "" {
		org = "PRIVATE"
	}
	scope := c.scope
	if scope == "" {
		if c.cc == "" {
			scope = "GLOBAL"
		} else {
			scope = "NATIONAL"
		}
	}
	return Candidate{
		Name: c.name, FeedURL: c.feed, WebsiteURL: c.web,
		Country: c.cc, Region: c.region, GeographicScope: scope,
		Languages: strings.Fields(c.langs), Category: c.cat, Industry: c.ind,
		Publisher: c.pub, OrgType: org, SourceType: st, OfficialFeed: true,
		Priority: c.prio, Credibility: c.cred, Tags: strings.Fields(c.tags),
		Metadata: map[string]any{"discoverySource": "curated"},
	}
}

// CuratedCandidates returns the hand-built list of leading direct feeds across
// global majors, science, security, finance, tech, government and regional press.
func CuratedCandidates() []Candidate {
	out := make([]Candidate, 0, len(curated))
	for _, c := range curated {
		out = append(out, c.toCandidate())
	}
	return out
}

var curated = []cur{
	// ── Global majors / wire-grade (English) ──
	{"BBC News — Top Stories", "http://feeds.bbci.co.uk/news/rss.xml", "https://www.bbc.com/news", "GB", "Europe", "", "General", "", "BBC", "PUBLIC", "en", "international breaking-news public-broadcaster", 1, 0.92},
	{"BBC News — World", "http://feeds.bbci.co.uk/news/world/rss.xml", "https://www.bbc.com/news/world", "GB", "Europe", "GLOBAL", "World", "", "BBC", "PUBLIC", "en", "international world public-broadcaster", 1, 0.92},
	{"BBC News — Business", "http://feeds.bbci.co.uk/news/business/rss.xml", "https://www.bbc.com/news/business", "GB", "Europe", "GLOBAL", "Business", "Finance", "BBC", "PUBLIC", "en", "business financial", 2, 0.9},
	{"BBC News — Technology", "http://feeds.bbci.co.uk/news/technology/rss.xml", "https://www.bbc.com/news/technology", "GB", "Europe", "GLOBAL", "Technology", "Technology", "BBC", "PUBLIC", "en", "technology", 2, 0.9},
	{"BBC News — Science & Environment", "http://feeds.bbci.co.uk/news/science_and_environment/rss.xml", "https://www.bbc.com/news/science_and_environment", "GB", "Europe", "GLOBAL", "Science", "Science", "BBC", "PUBLIC", "en", "science environment climate", 2, 0.9},
	{"BBC News — Health", "http://feeds.bbci.co.uk/news/health/rss.xml", "https://www.bbc.com/news/health", "GB", "Europe", "GLOBAL", "Health", "Healthcare", "BBC", "PUBLIC", "en", "health public-health", 2, 0.9},
	{"BBC Sport", "http://feeds.bbci.co.uk/sport/rss.xml", "https://www.bbc.com/sport", "GB", "Europe", "GLOBAL", "Sports", "Sports", "BBC", "PUBLIC", "en", "sports", 3, 0.88},
	{"The Guardian — World", "https://www.theguardian.com/world/rss", "https://www.theguardian.com/world", "GB", "Europe", "GLOBAL", "World", "", "Guardian Media Group", "PRIVATE", "en", "international world", 1, 0.88},
	{"The Guardian — International", "https://www.theguardian.com/international/rss", "https://www.theguardian.com/international", "GB", "Europe", "GLOBAL", "General", "", "Guardian Media Group", "PRIVATE", "en", "international", 1, 0.88},
	{"The Guardian — Technology", "https://www.theguardian.com/technology/rss", "https://www.theguardian.com/technology", "GB", "Europe", "GLOBAL", "Technology", "Technology", "Guardian Media Group", "PRIVATE", "en", "technology", 2, 0.86},
	{"The Guardian — Business", "https://www.theguardian.com/business/rss", "https://www.theguardian.com/business", "GB", "Europe", "GLOBAL", "Business", "Finance", "Guardian Media Group", "PRIVATE", "en", "business financial", 2, 0.86},
	{"The Guardian — Science", "https://www.theguardian.com/science/rss", "https://www.theguardian.com/science", "GB", "Europe", "GLOBAL", "Science", "Science", "Guardian Media Group", "PRIVATE", "en", "science research", 2, 0.86},
	{"The Guardian — Environment", "https://www.theguardian.com/environment/rss", "https://www.theguardian.com/environment", "GB", "Europe", "GLOBAL", "Environment", "Climate", "Guardian Media Group", "PRIVATE", "en", "environment climate", 2, 0.86},
	{"Al Jazeera English", "https://www.aljazeera.com/xml/rss/all.xml", "https://www.aljazeera.com", "QA", "Middle East", "GLOBAL", "General", "", "Al Jazeera Media Network", "PUBLIC", "en", "international middle-east", 1, 0.85},
	{"NPR — News", "https://feeds.npr.org/1001/rss.xml", "https://www.npr.org", "US", "North America", "NATIONAL", "General", "", "NPR", "PUBLIC", "en", "public-broadcaster national", 1, 0.9},
	{"NPR — World", "https://feeds.npr.org/1004/rss.xml", "https://www.npr.org/sections/world", "US", "North America", "GLOBAL", "World", "", "NPR", "PUBLIC", "en", "world public-broadcaster", 2, 0.9},
	{"NPR — Technology", "https://feeds.npr.org/1019/rss.xml", "https://www.npr.org/sections/technology", "US", "North America", "GLOBAL", "Technology", "Technology", "NPR", "PUBLIC", "en", "technology", 2, 0.89},
	{"NPR — Science", "https://feeds.npr.org/1007/rss.xml", "https://www.npr.org/sections/science", "US", "North America", "GLOBAL", "Science", "Science", "NPR", "PUBLIC", "en", "science research", 2, 0.89},
	{"Deutsche Welle — English", "https://rss.dw.com/rdf/rss-en-all", "https://www.dw.com/en", "DE", "Europe", "GLOBAL", "General", "", "Deutsche Welle", "PUBLIC", "en", "international public-broadcaster", 1, 0.88},
	{"Deutsche Welle — Deutsch", "https://rss.dw.com/rdf/rss-de-all", "https://www.dw.com/de", "DE", "Europe", "GLOBAL", "General", "", "Deutsche Welle", "PUBLIC", "de", "international public-broadcaster", 2, 0.88},
	{"France 24 — English", "https://www.france24.com/en/rss", "https://www.france24.com/en", "FR", "Europe", "GLOBAL", "General", "", "France Médias Monde", "PUBLIC", "en", "international public-broadcaster", 1, 0.87},
	{"France 24 — Français", "https://www.france24.com/fr/rss", "https://www.france24.com/fr", "FR", "Europe", "GLOBAL", "General", "", "France Médias Monde", "PUBLIC", "fr", "international public-broadcaster", 2, 0.87},
	{"CNN — Top Stories", "http://rss.cnn.com/rss/edition.rss", "https://edition.cnn.com", "US", "North America", "GLOBAL", "General", "", "CNN", "PRIVATE", "en", "international breaking-news", 2, 0.82},
	{"CNN — World", "http://rss.cnn.com/rss/edition_world.rss", "https://edition.cnn.com/world", "US", "North America", "GLOBAL", "World", "", "CNN", "PRIVATE", "en", "world", 2, 0.82},
	{"Sky News — World", "https://feeds.skynews.com/feeds/rss/world.xml", "https://news.sky.com/world", "GB", "Europe", "GLOBAL", "World", "", "Sky News", "PRIVATE", "en", "world breaking-news", 2, 0.84},
	{"Sky News — Technology", "https://feeds.skynews.com/feeds/rss/technology.xml", "https://news.sky.com/technology", "GB", "Europe", "GLOBAL", "Technology", "Technology", "Sky News", "PRIVATE", "en", "technology", 3, 0.83},
	{"Euronews", "https://www.euronews.com/rss", "https://www.euronews.com", "FR", "Europe", "CONTINENTAL", "General", "", "Euronews", "PRIVATE", "en", "europe international", 2, 0.83},
	{"The Independent — World", "https://www.independent.co.uk/news/world/rss", "https://www.independent.co.uk", "GB", "Europe", "GLOBAL", "World", "", "The Independent", "PRIVATE", "en", "world", 2, 0.82},
	{"TIME", "https://time.com/feed/", "https://time.com", "US", "North America", "GLOBAL", "General", "", "TIME", "PRIVATE", "en", "international long-form-analysis", 2, 0.84},
	{"The Atlantic", "https://www.theatlantic.com/feed/all/", "https://www.theatlantic.com", "US", "North America", "GLOBAL", "General", "", "The Atlantic", "PRIVATE", "en", "long-form-analysis", 2, 0.84},
	{"Vox", "https://www.vox.com/rss/index.xml", "https://www.vox.com", "US", "North America", "GLOBAL", "General", "", "Vox Media", "PRIVATE", "en", "explainer analysis", 3, 0.8},

	// ── United States ──
	{"The New York Times — Home", "https://rss.nytimes.com/services/xml/rss/nyt/HomePage.xml", "https://www.nytimes.com", "US", "North America", "NATIONAL", "General", "", "The New York Times", "PRIVATE", "en", "national breaking-news", 1, 0.9},
	{"The New York Times — World", "https://rss.nytimes.com/services/xml/rss/nyt/World.xml", "https://www.nytimes.com/section/world", "US", "North America", "GLOBAL", "World", "", "The New York Times", "PRIVATE", "en", "world", 1, 0.9},
	{"The New York Times — Technology", "https://rss.nytimes.com/services/xml/rss/nyt/Technology.xml", "https://www.nytimes.com/section/technology", "US", "North America", "GLOBAL", "Technology", "Technology", "The New York Times", "PRIVATE", "en", "technology", 2, 0.89},
	{"The New York Times — Business", "https://rss.nytimes.com/services/xml/rss/nyt/Business.xml", "https://www.nytimes.com/section/business", "US", "North America", "GLOBAL", "Business", "Finance", "The New York Times", "PRIVATE", "en", "business financial", 2, 0.89},
	{"The New York Times — Science", "https://rss.nytimes.com/services/xml/rss/nyt/Science.xml", "https://www.nytimes.com/section/science", "US", "North America", "GLOBAL", "Science", "Science", "The New York Times", "PRIVATE", "en", "science research", 2, 0.89},
	{"The New York Times — Health", "https://rss.nytimes.com/services/xml/rss/nyt/Health.xml", "https://www.nytimes.com/section/health", "US", "North America", "GLOBAL", "Health", "Healthcare", "The New York Times", "PRIVATE", "en", "health", 2, 0.89},
	{"The Washington Post — World", "http://feeds.washingtonpost.com/rss/world", "https://www.washingtonpost.com/world", "US", "North America", "GLOBAL", "World", "", "The Washington Post", "PRIVATE", "en", "world", 2, 0.88},
	{"The Washington Post — Business", "http://feeds.washingtonpost.com/rss/business", "https://www.washingtonpost.com/business", "US", "North America", "GLOBAL", "Business", "Finance", "The Washington Post", "PRIVATE", "en", "business financial", 2, 0.87},
	{"The Washington Post — Technology", "http://feeds.washingtonpost.com/rss/business/technology", "https://www.washingtonpost.com/technology", "US", "North America", "GLOBAL", "Technology", "Technology", "The Washington Post", "PRIVATE", "en", "technology", 2, 0.87},
	{"Politico", "https://www.politico.com/rss/politicopicks.xml", "https://www.politico.com", "US", "North America", "NATIONAL", "Politics", "Government", "Politico", "PRIVATE", "en", "politics government national", 2, 0.84},
	{"The Hill", "https://thehill.com/news/feed/", "https://thehill.com", "US", "North America", "NATIONAL", "Politics", "Government", "The Hill", "PRIVATE", "en", "politics government", 3, 0.8},
	{"NBC News", "http://feeds.nbcnews.com/nbcnews/public/news", "https://www.nbcnews.com", "US", "North America", "NATIONAL", "General", "", "NBC News", "PRIVATE", "en", "national breaking-news", 2, 0.84},
	{"CBS News", "https://www.cbsnews.com/latest/rss/main", "https://www.cbsnews.com", "US", "North America", "NATIONAL", "General", "", "CBS News", "PRIVATE", "en", "national", 2, 0.83},
	{"ABC News (US)", "https://abcnews.go.com/abcnews/topstories", "https://abcnews.go.com", "US", "North America", "NATIONAL", "General", "", "ABC News", "PRIVATE", "en", "national", 2, 0.83},
	{"ProPublica", "https://www.propublica.org/feeds/propublica/main", "https://www.propublica.org", "US", "North America", "NATIONAL", "Investigative", "", "ProPublica", "INDEPENDENT", "en", "investigative long-form-analysis", 2, 0.88},
	{"The Wall Street Journal — World", "https://feeds.a.dj.com/rss/RSSWorldNews.xml", "https://www.wsj.com/world", "US", "North America", "GLOBAL", "World", "", "Dow Jones", "PRIVATE", "en", "world business", 2, 0.88},
	{"The Wall Street Journal — Markets", "https://feeds.a.dj.com/rss/RSSMarketsMain.xml", "https://www.wsj.com/finance/markets", "US", "North America", "GLOBAL", "Markets", "Public Markets", "Dow Jones", "PRIVATE", "en", "markets financial", 2, 0.88},
	{"The Wall Street Journal — Technology", "https://feeds.a.dj.com/rss/RSSWSJD.xml", "https://www.wsj.com/tech", "US", "North America", "GLOBAL", "Technology", "Technology", "Dow Jones", "PRIVATE", "en", "technology", 2, 0.88},

	// ── Technology ──
	{"TechCrunch", "https://techcrunch.com/feed/", "https://techcrunch.com", "US", "North America", "GLOBAL", "Technology", "Startups", "TechCrunch", "PRIVATE", "en", "technology startup vc", 2, 0.83},
	{"Ars Technica", "https://feeds.arstechnica.com/arstechnica/index", "https://arstechnica.com", "US", "North America", "GLOBAL", "Technology", "Technology", "Condé Nast", "PRIVATE", "en", "technology software", 2, 0.86},
	{"The Verge", "https://www.theverge.com/rss/index.xml", "https://www.theverge.com", "US", "North America", "GLOBAL", "Technology", "Technology", "Vox Media", "PRIVATE", "en", "technology consumer", 2, 0.84},
	{"WIRED", "https://www.wired.com/feed/rss", "https://www.wired.com", "US", "North America", "GLOBAL", "Technology", "Technology", "Condé Nast", "PRIVATE", "en", "technology long-form-analysis", 2, 0.85},
	{"Engadget", "https://www.engadget.com/rss.xml", "https://www.engadget.com", "US", "North America", "GLOBAL", "Technology", "Technology", "Yahoo", "PRIVATE", "en", "technology consumer", 3, 0.8},
	{"Hacker News (Front Page)", "https://news.ycombinator.com/rss", "https://news.ycombinator.com", "US", "North America", "GLOBAL", "Technology", "Software Engineering", "Y Combinator", "PRIVATE", "en", "technology software open-source community", 3, 0.78},
	{"MIT Technology Review", "https://www.technologyreview.com/feed/", "https://www.technologyreview.com", "US", "North America", "GLOBAL", "Technology", "Artificial Intelligence", "MIT", "PUBLIC", "en", "technology ai research", 2, 0.88},
	{"VentureBeat", "https://venturebeat.com/feed/", "https://venturebeat.com", "US", "North America", "GLOBAL", "Technology", "Artificial Intelligence", "VentureBeat", "PRIVATE", "en", "technology ai startup", 3, 0.79},
	{"ZDNet", "https://www.zdnet.com/news/rss.xml", "https://www.zdnet.com", "US", "North America", "GLOBAL", "Technology", "Enterprise", "ZDNet", "PRIVATE", "en", "technology enterprise", 3, 0.79},
	{"The Register", "https://www.theregister.com/headlines.atom", "https://www.theregister.com", "GB", "Europe", "GLOBAL", "Technology", "Enterprise", "The Register", "PRIVATE", "en", "technology enterprise cloud", 3, 0.8},
	{"TechRadar", "https://www.techradar.com/rss", "https://www.techradar.com", "GB", "Europe", "GLOBAL", "Technology", "Technology", "Future plc", "PRIVATE", "en", "technology consumer", 3, 0.76},
	{"IEEE Spectrum", "https://spectrum.ieee.org/feeds/feed.rss", "https://spectrum.ieee.org", "US", "North America", "GLOBAL", "Technology", "Robotics", "IEEE", "PUBLIC", "en", "technology robotics science research", 2, 0.88},
	{"Slashdot", "http://rss.slashdot.org/Slashdot/slashdotMain", "https://slashdot.org", "US", "North America", "GLOBAL", "Technology", "Software Engineering", "Slashdot Media", "PRIVATE", "en", "technology software community", 3, 0.74},
	{"9to5Mac", "https://9to5mac.com/feed/", "https://9to5mac.com", "US", "North America", "GLOBAL", "Technology", "Technology", "9to5", "PRIVATE", "en", "technology consumer product-announcements", 3, 0.74},
	{"Android Police", "https://www.androidpolice.com/feed/", "https://www.androidpolice.com", "US", "North America", "GLOBAL", "Technology", "Technology", "Valnet", "PRIVATE", "en", "technology consumer", 4, 0.72},

	// ── AI / open source / developer / product ──
	{"GitHub Blog", "https://github.blog/feed/", "https://github.blog", "US", "North America", "GLOBAL", "Technology", "Software Engineering", "GitHub", "PRIVATE", "en", "software open-source product-announcements developer", 3, 0.82},
	{"Stack Overflow Blog", "https://stackoverflow.blog/feed/", "https://stackoverflow.blog", "US", "North America", "GLOBAL", "Technology", "Software Engineering", "Stack Overflow", "PRIVATE", "en", "software developer community", 3, 0.8},
	{"AWS — What's New", "https://aws.amazon.com/about-aws/whats-new/recent/feed/", "https://aws.amazon.com/new/", "US", "North America", "GLOBAL", "Technology", "Cloud Computing", "Amazon Web Services", "PRIVATE", "en", "cloud product-announcements official-announcements", 3, 0.82},
	{"Kubernetes Blog", "https://kubernetes.io/feed.xml", "https://kubernetes.io/blog/", "US", "North America", "GLOBAL", "Technology", "DevOps", "CNCF", "INDEPENDENT", "en", "cloud devops open-source", 3, 0.82},
	{"The Cloud Native Computing Foundation", "https://www.cncf.io/feed/", "https://www.cncf.io", "US", "North America", "GLOBAL", "Technology", "Cloud Computing", "CNCF", "INDEPENDENT", "en", "cloud devops open-source", 3, 0.8},
	{"Mozilla Blog", "https://blog.mozilla.org/en/feed/", "https://blog.mozilla.org", "US", "North America", "GLOBAL", "Technology", "Software Engineering", "Mozilla", "INDEPENDENT", "en", "open-source product-announcements", 3, 0.8},
	{"MarkTechPost (AI)", "https://www.marktechpost.com/feed/", "https://www.marktechpost.com", "US", "North America", "GLOBAL", "Technology", "Artificial Intelligence", "MarkTechPost", "PRIVATE", "en", "ai research", 3, 0.72},

	// ── Cybersecurity & security advisories ──
	{"Krebs on Security", "https://krebsonsecurity.com/feed/", "https://krebsonsecurity.com", "US", "North America", "GLOBAL", "Technology", "Cybersecurity", "Brian Krebs", "INDEPENDENT", "en", "security cybersecurity investigative", 2, 0.88},
	{"The Hacker News (Security)", "https://feeds.feedburner.com/TheHackersNews", "https://thehackernews.com", "IN", "South Asia", "GLOBAL", "Technology", "Cybersecurity", "The Hacker News", "PRIVATE", "en", "security cybersecurity alerts", 2, 0.8},
	{"BleepingComputer", "https://www.bleepingcomputer.com/feed/", "https://www.bleepingcomputer.com", "US", "North America", "GLOBAL", "Technology", "Cybersecurity", "BleepingComputer", "PRIVATE", "en", "security cybersecurity alerts", 2, 0.82},
	{"Dark Reading", "https://www.darkreading.com/rss.xml", "https://www.darkreading.com", "US", "North America", "GLOBAL", "Technology", "Cybersecurity", "Informa", "PRIVATE", "en", "security cybersecurity enterprise", 3, 0.82},
	{"SANS Internet Storm Center", "https://isc.sans.edu/rssfeed.xml", "https://isc.sans.edu", "US", "North America", "GLOBAL", "Technology", "Cybersecurity", "SANS Institute", "PUBLIC", "en", "security cybersecurity alerts advisory", 2, 0.86},
	{"Schneier on Security", "https://www.schneier.com/feed/atom/", "https://www.schneier.com", "US", "North America", "GLOBAL", "Technology", "Cybersecurity", "Bruce Schneier", "INDEPENDENT", "en", "security cybersecurity analysis", 3, 0.85},
	{"SecurityWeek", "https://www.securityweek.com/feed/", "https://www.securityweek.com", "US", "North America", "GLOBAL", "Technology", "Cybersecurity", "Wired Business Media", "PRIVATE", "en", "security cybersecurity", 3, 0.8},
	{"CISA — Cybersecurity Advisories", "https://www.cisa.gov/cybersecurity-advisories/all.xml", "https://www.cisa.gov", "US", "North America", "GLOBAL", "Security Advisory", "Cybersecurity", "CISA", "GOVERNMENT", "en", "security cybersecurity advisory alerts regulatory official-announcements", 1, 0.95},

	// ── Science / space / research ──
	{"ScienceDaily", "https://www.sciencedaily.com/rss/all.xml", "https://www.sciencedaily.com", "US", "North America", "GLOBAL", "Science", "Science", "ScienceDaily", "PRIVATE", "en", "science research", 2, 0.82},
	{"Phys.org", "https://phys.org/rss-feed/", "https://phys.org", "GB", "Europe", "GLOBAL", "Science", "Science", "Science X", "PRIVATE", "en", "science research", 2, 0.82},
	{"Nature", "https://www.nature.com/nature.rss", "https://www.nature.com", "GB", "Europe", "GLOBAL", "Research Feed", "Science", "Springer Nature", "PUBLIC", "en", "science research scientific", 1, 0.93},
	{"Science (AAAS) — News", "https://www.science.org/rss/news_current.xml", "https://www.science.org", "US", "North America", "GLOBAL", "Research Feed", "Science", "AAAS", "PUBLIC", "en", "science research scientific", 1, 0.93},
	{"New Scientist", "https://www.newscientist.com/feed/home/", "https://www.newscientist.com", "GB", "Europe", "GLOBAL", "Science", "Science", "New Scientist", "PRIVATE", "en", "science research", 2, 0.84},
	{"Quanta Magazine", "https://api.quantamagazine.org/feed/", "https://www.quantamagazine.org", "US", "North America", "GLOBAL", "Science", "Science", "Simons Foundation", "INDEPENDENT", "en", "science research long-form-analysis", 2, 0.88},
	{"Live Science", "https://www.livescience.com/feeds/all", "https://www.livescience.com", "US", "North America", "GLOBAL", "Science", "Science", "Future plc", "PRIVATE", "en", "science", 3, 0.78},
	{"Space.com", "https://www.space.com/feeds/all", "https://www.space.com", "US", "North America", "GLOBAL", "Science", "Space", "Future plc", "PRIVATE", "en", "space science", 2, 0.8},
	{"SpaceNews", "https://spacenews.com/feed/", "https://spacenews.com", "US", "North America", "GLOBAL", "Science", "Space", "SpaceNews", "PRIVATE", "en", "space aerospace industry", 2, 0.83},
	{"NASA — Breaking News", "https://www.nasa.gov/news-release/feed/", "https://www.nasa.gov", "US", "North America", "GLOBAL", "Science", "Space", "NASA", "GOVERNMENT", "en", "space science official-announcements", 1, 0.95},
	{"arXiv — Computer Science", "https://export.arxiv.org/rss/cs", "https://arxiv.org", "US", "North America", "GLOBAL", "Research Feed", "Software Engineering", "arXiv (Cornell)", "PUBLIC", "en", "research scientific software ai", 2, 0.9},
	{"arXiv — Artificial Intelligence", "https://export.arxiv.org/rss/cs.AI", "https://arxiv.org/list/cs.AI/recent", "US", "North America", "GLOBAL", "Research Feed", "Artificial Intelligence", "arXiv (Cornell)", "PUBLIC", "en", "research scientific ai", 2, 0.9},

	// ── Climate / environment ──
	{"Carbon Brief", "https://www.carbonbrief.org/feed/", "https://www.carbonbrief.org", "GB", "Europe", "GLOBAL", "Environment", "Climate", "Carbon Brief", "INDEPENDENT", "en", "climate environment science", 2, 0.86},
	{"Inside Climate News", "https://insideclimatenews.org/feed/", "https://insideclimatenews.org", "US", "North America", "GLOBAL", "Environment", "Climate", "Inside Climate News", "INDEPENDENT", "en", "climate environment investigative", 2, 0.85},
	{"Yale Environment 360", "https://e360.yale.edu/feed.xml", "https://e360.yale.edu", "US", "North America", "GLOBAL", "Environment", "Climate", "Yale", "PUBLIC", "en", "climate environment analysis", 3, 0.85},
	{"Grist", "https://grist.org/feed/", "https://grist.org", "US", "North America", "GLOBAL", "Environment", "Climate", "Grist", "INDEPENDENT", "en", "climate environment", 3, 0.78},

	// ── Finance / business / crypto ──
	{"CNBC — Top News", "https://search.cnbc.com/rs/search/combinedcms/view.xml?partnerId=wrss01&id=100003114", "https://www.cnbc.com", "US", "North America", "GLOBAL", "Business", "Finance", "CNBC", "PRIVATE", "en", "business financial markets", 2, 0.83},
	{"MarketWatch — Top Stories", "http://feeds.marketwatch.com/marketwatch/topstories/", "https://www.marketwatch.com", "US", "North America", "GLOBAL", "Markets", "Public Markets", "Dow Jones", "PRIVATE", "en", "markets financial", 2, 0.82},
	{"Financial Times — Home", "https://www.ft.com/rss/home", "https://www.ft.com", "GB", "Europe", "GLOBAL", "Business", "Finance", "Financial Times", "PRIVATE", "en", "business financial markets", 1, 0.9},
	{"Fortune", "https://fortune.com/feed/", "https://fortune.com", "US", "North America", "GLOBAL", "Business", "Finance", "Fortune Media", "PRIVATE", "en", "business financial", 3, 0.8},
	{"Business Insider", "https://www.businessinsider.com/rss", "https://www.businessinsider.com", "US", "North America", "GLOBAL", "Business", "Finance", "Insider Inc.", "PRIVATE", "en", "business financial markets", 3, 0.76},
	{"Investing.com — News", "https://www.investing.com/rss/news.rss", "https://www.investing.com", "US", "North America", "GLOBAL", "Markets", "Public Markets", "Investing.com", "PRIVATE", "en", "markets financial commodities", 3, 0.74},
	{"CoinDesk", "https://www.coindesk.com/arc/outboundfeeds/rss/", "https://www.coindesk.com", "US", "North America", "GLOBAL", "Business", "Cryptocurrency", "CoinDesk", "PRIVATE", "en", "crypto financial technology", 2, 0.79},
	{"Cointelegraph", "https://cointelegraph.com/rss", "https://cointelegraph.com", "US", "North America", "GLOBAL", "Business", "Cryptocurrency", "Cointelegraph", "PRIVATE", "en", "crypto financial", 3, 0.74},
	{"Decrypt", "https://decrypt.co/feed", "https://decrypt.co", "US", "North America", "GLOBAL", "Business", "Cryptocurrency", "Decrypt", "PRIVATE", "en", "crypto technology", 3, 0.73},

	// ── Government / IGO / NGO / disaster / health ──
	{"UN News", "https://news.un.org/feed/subscribe/en/news/all/rss.xml", "https://news.un.org", "US", "North America", "GLOBAL", "Government Feed", "Government", "United Nations", "PUBLIC", "en", "international government official-announcements", 1, 0.92},
	{"U.S. Federal Reserve — Press Releases", "https://www.federalreserve.gov/feeds/press_all.xml", "https://www.federalreserve.gov", "US", "North America", "GLOBAL", "Government Feed", "Banking", "Federal Reserve", "GOVERNMENT", "en", "financial government regulatory press-releases official-announcements", 1, 0.95},
	{"ReliefWeb — Updates", "https://reliefweb.int/updates/rss.xml", "https://reliefweb.int", "US", "North America", "GLOBAL", "Alerts", "Government", "UN OCHA", "PUBLIC", "en", "disaster emergency humanitarian alerts", 1, 0.92},
	{"GDACS — Global Disaster Alerts", "https://www.gdacs.org/xml/rss.xml", "https://www.gdacs.org", "IT", "Europe", "GLOBAL", "Alerts", "Disaster & Emergency", "GDACS", "PUBLIC", "en", "disaster emergency alerts weather", 1, 0.93},
	{"USGS — Earthquakes (M2.5+, past day)", "https://earthquake.usgs.gov/earthquakes/feed/v1.0/summary/2.5_day.atom", "https://earthquake.usgs.gov", "US", "North America", "GLOBAL", "Alerts", "Disaster & Emergency", "USGS", "GOVERNMENT", "en", "disaster earthquake alerts official-announcements", 0, 0.99},
	{"STAT News (Health)", "https://www.statnews.com/feed/", "https://www.statnews.com", "US", "North America", "GLOBAL", "Health", "Healthcare", "STAT", "PRIVATE", "en", "health healthcare pharma biotech", 2, 0.85},
	{"KFF Health News", "https://kffhealthnews.org/feed/", "https://kffhealthnews.org", "US", "North America", "NATIONAL", "Health", "Healthcare", "KFF", "INDEPENDENT", "en", "health healthcare public-health", 3, 0.85},

	// ── Sports ──
	{"ESPN — Top Headlines", "https://www.espn.com/espn/rss/news", "https://www.espn.com", "US", "North America", "GLOBAL", "Sports", "Sports", "ESPN", "PRIVATE", "en", "sports", 3, 0.82},
	{"Sky Sports", "https://www.skysports.com/rss/12040", "https://www.skysports.com", "GB", "Europe", "GLOBAL", "Sports", "Sports", "Sky Sports", "PRIVATE", "en", "sports", 3, 0.8},

	// ── Aviation / maritime / energy / mining ──
	{"gCaptain (Maritime)", "https://gcaptain.com/feed/", "https://gcaptain.com", "US", "North America", "GLOBAL", "Industry", "Maritime", "gCaptain", "PRIVATE", "en", "maritime shipping logistics", 3, 0.82},
	{"Splash 247 (Maritime)", "https://splash247.com/feed/", "https://splash247.com", "SG", "Southeast Asia", "GLOBAL", "Industry", "Maritime", "Splash", "PRIVATE", "en", "maritime shipping", 3, 0.8},
	{"OilPrice.com", "https://oilprice.com/rss/main", "https://oilprice.com", "US", "North America", "GLOBAL", "Industry", "Oil & Gas", "Oilprice", "PRIVATE", "en", "energy oil-gas commodities", 3, 0.78},
	{"Mining.com", "https://www.mining.com/feed/", "https://www.mining.com", "CA", "North America", "GLOBAL", "Industry", "Mining", "Mining.com", "PRIVATE", "en", "mining commodities", 3, 0.79},

	// ── Europe (national press, native languages) ──
	{"Le Monde (Une)", "https://www.lemonde.fr/rss/une.xml", "https://www.lemonde.fr", "FR", "Europe", "NATIONAL", "General", "", "Le Monde", "PRIVATE", "fr", "national europe", 1, 0.88},
	{"Der Spiegel — Schlagzeilen", "https://www.spiegel.de/schlagzeilen/tops/index.rss", "https://www.spiegel.de", "DE", "Europe", "NATIONAL", "General", "", "Der Spiegel", "PRIVATE", "de", "national europe", 1, 0.88},
	{"Die Zeit", "https://newsfeed.zeit.de/index", "https://www.zeit.de", "DE", "Europe", "NATIONAL", "General", "", "Die Zeit", "PRIVATE", "de", "national europe analysis", 2, 0.86},
	{"El Mundo — Portada", "https://e00-elmundo.uecdn.es/elmundo/rss/portada.xml", "https://www.elmundo.es", "ES", "Europe", "NATIONAL", "General", "", "El Mundo", "PRIVATE", "es", "national europe", 2, 0.84},
	{"La Repubblica — Homepage", "https://www.repubblica.it/rss/homepage/rss2.0.xml", "https://www.repubblica.it", "IT", "Europe", "NATIONAL", "General", "", "La Repubblica", "PRIVATE", "it", "national europe", 2, 0.84},
	{"ANSA — Top News", "https://www.ansa.it/sito/ansait_rss.xml", "https://www.ansa.it", "IT", "Europe", "NATIONAL", "General", "", "ANSA", "PRIVATE", "it", "national news-agency europe", 1, 0.86},
	{"NOS Nieuws (NL)", "https://feeds.nos.nl/nosnieuwsalgemeen", "https://nos.nl", "NL", "Europe", "NATIONAL", "General", "", "NOS", "PUBLIC", "nl", "national public-broadcaster europe", 1, 0.87},
	{"RTÉ News (IE)", "https://www.rte.ie/feeds/rss/?index=/news/", "https://www.rte.ie/news/", "IE", "Europe", "NATIONAL", "General", "", "RTÉ", "PUBLIC", "en", "national public-broadcaster europe", 2, 0.85},
	{"Euractiv (EU policy)", "https://www.euractiv.com/feed/", "https://www.euractiv.com", "BE", "Europe", "CONTINENTAL", "Politics", "Government", "Euractiv", "PRIVATE", "en", "europe politics government regulatory", 3, 0.82},

	// ── Asia / Middle East / Africa / LatAm (national press) ──
	{"The Times of India — Top Stories", "https://timesofindia.indiatimes.com/rssfeedstopstories.cms", "https://timesofindia.indiatimes.com", "IN", "South Asia", "NATIONAL", "General", "", "The Times of India", "PRIVATE", "en", "national south-asia", 2, 0.8},
	{"The Hindu — National", "https://www.thehindu.com/news/national/feeder/default.rss", "https://www.thehindu.com", "IN", "South Asia", "NATIONAL", "General", "", "The Hindu", "PRIVATE", "en", "national south-asia", 2, 0.84},
	{"NDTV — Top Stories", "https://feeds.feedburner.com/ndtvnews-top-stories", "https://www.ndtv.com", "IN", "South Asia", "NATIONAL", "General", "", "NDTV", "PRIVATE", "en", "national south-asia", 2, 0.8},
	{"Dawn (Pakistan)", "https://www.dawn.com/feeds/home", "https://www.dawn.com", "PK", "South Asia", "NATIONAL", "General", "", "Dawn", "PRIVATE", "en", "national south-asia", 2, 0.82},
	{"The Japan Times", "https://www.japantimes.co.jp/feed/", "https://www.japantimes.co.jp", "JP", "East Asia", "NATIONAL", "General", "", "The Japan Times", "PRIVATE", "en", "national east-asia", 2, 0.84},
	{"Channel News Asia", "https://www.channelnewsasia.com/api/v1/rss-outbound-feed?_format=xml", "https://www.channelnewsasia.com", "SG", "Southeast Asia", "REGIONAL", "General", "", "Mediacorp", "PUBLIC", "en", "southeast-asia regional public-broadcaster", 2, 0.85},
	{"The Times of Israel", "https://www.timesofisrael.com/feed/", "https://www.timesofisrael.com", "IL", "Middle East", "NATIONAL", "General", "", "The Times of Israel", "PRIVATE", "en", "national middle-east", 2, 0.83},
	{"The Jerusalem Post", "https://www.jpost.com/rss/rssfeedsheadlines.aspx", "https://www.jpost.com", "IL", "Middle East", "NATIONAL", "General", "", "The Jerusalem Post", "PRIVATE", "en", "national middle-east", 3, 0.8},
	{"Middle East Eye", "https://www.middleeasteye.net/rss", "https://www.middleeasteye.net", "GB", "Middle East", "REGIONAL", "General", "", "Middle East Eye", "INDEPENDENT", "en", "middle-east regional", 3, 0.78},
	{"Premium Times (Nigeria)", "https://www.premiumtimesng.com/feed", "https://www.premiumtimesng.com", "NG", "Africa", "NATIONAL", "General", "", "Premium Times", "INDEPENDENT", "en", "national africa investigative", 3, 0.8},
	{"Folha de S.Paulo", "https://feeds.folha.uol.com.br/emcimadahora/rss091.xml", "https://www.folha.uol.com.br", "BR", "South America", "NATIONAL", "General", "", "Folha de S.Paulo", "PRIVATE", "pt", "national south-america", 2, 0.83},
	{"CBC News — Top Stories (Canada)", "https://www.cbc.ca/webfeed/rss/rss-topstories", "https://www.cbc.ca/news", "CA", "North America", "NATIONAL", "General", "", "CBC", "PUBLIC", "en", "national public-broadcaster", 1, 0.88},
	{"CBC News — World", "https://www.cbc.ca/webfeed/rss/rss-world", "https://www.cbc.ca/news/world", "CA", "North America", "GLOBAL", "World", "", "CBC", "PUBLIC", "en", "world public-broadcaster", 2, 0.87},
}
