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
//
// Every topical domain ends with a `<DOMAIN>.OTHER` leaf: a story recognized at
// the domain level but not matching a specific leaf lands there, so it stays in
// the correct domain instead of falling through to GENERAL.OTHER. GENERAL.OTHER
// is the true last resort — used only when no domain applies at all.
var Taxonomy = []Node{
	domain("POLITICS", "Politics",
		leaf("POLITICS.ELECTIONS", "Elections", "election", "elections", "ballot", "vote", "voting", "poll", "polls", "candidate", "campaign", "primary", "referendum", "electoral", "turnout", "constituency"),
		leaf("POLITICS.POLICY", "Policy & Legislation", "policy", "legislation", "bill", "reform", "parliament", "congress", "senate", "lawmakers", "budget", "spending bill", "executive order", "mandate", "governance"),
		leaf("POLITICS.DIPLOMACY", "Diplomacy & Foreign Affairs", "diplomacy", "diplomatic", "summit", "treaty", "sanction", "sanctions", "embassy", "foreign minister", "foreign policy", "bilateral", "alliance", "ambassador", "geopolitics", "un security council"),
		leaf("POLITICS.GOVERNMENT", "Government & Leadership", "president", "prime minister", "cabinet", "minister", "government", "administration", "coalition", "appointment", "resignation", "impeachment", "no-confidence", "sworn in", "inauguration"),
		leaf("POLITICS.PROTEST", "Protests & Civil Unrest", "protest", "protests", "demonstration", "rally", "march", "riot", "unrest", "uprising", "strike action", "boycott", "sit-in", "clashes with police"),
		leaf("POLITICS.CORRUPTION", "Corruption & Scandal", "corruption", "scandal", "bribery", "graft", "embezzlement", "misconduct", "nepotism", "cronyism", "kickback", "abuse of power"),
		leaf("POLITICS.OTHER", "Other Politics", "political", "politician", "party leader", "opposition", "government official"),
	),
	domain("ECONOMY", "Economy",
		leaf("ECONOMY.INFLATION", "Inflation", "inflation", "cpi", "consumer price", "cost of living", "price rise", "deflation", "price index"),
		leaf("ECONOMY.INTEREST_RATES", "Interest Rates & Monetary Policy", "interest rate", "central bank", "rate hike", "rate cut", "basis points", "federal reserve", "the fed", "rbi", "ecb", "bank of england", "monetary policy", "quantitative easing"),
		leaf("ECONOMY.MARKETS", "Markets", "stock market", "share price", "shares rose", "shares fell", "stocks", "nasdaq", "dow jones", "s&p 500", "nifty", "sensex", "ftse", "commodities", "crypto", "bitcoin", "ethereum", "bond yields", "wall street", "sell-off"),
		leaf("ECONOMY.JOBS", "Jobs & Employment", "unemployment", "jobs report", "hiring", "labor market", "labour market", "payroll", "jobless", "wage growth", "job growth", "employment rate"),
		leaf("ECONOMY.TRADE", "Trade & Tariffs", "tariff", "tariffs", "trade deal", "trade war", "exports", "imports", "trade deficit", "wto", "supply chain", "trade agreement", "customs"),
		leaf("ECONOMY.GROWTH", "Growth & Recession", "gdp", "recession", "economic growth", "downturn", "slowdown", "stimulus", "fiscal", "debt ceiling", "sovereign debt", "economic outlook"),
		leaf("ECONOMY.OTHER", "Other Economy", "economy", "economic", "economist", "macroeconomic"),
	),
	domain("BUSINESS", "Business",
		leaf("BUSINESS.EARNINGS", "Earnings", "earnings", "quarterly results", "revenue", "profit", "guidance", "net income", "beats estimates", "misses estimates", "quarterly profit"),
		leaf("BUSINESS.MA", "Mergers & Acquisitions", "acquisition", "merger", "takeover", "acquires", "acquired", "buyout", "merges with", "deal to buy"),
		leaf("BUSINESS.FUNDING", "Funding & IPOs", "funding", "series a", "series b", "series c", "raised", "venture", "valuation", "ipo", "seed round", "fundraise", "public offering", "spac"),
		leaf("BUSINESS.LAYOFFS", "Layoffs & Restructuring", "layoff", "layoffs", "job cuts", "restructuring", "redundancies", "downsizing", "cutting jobs", "workforce reduction"),
		leaf("BUSINESS.LEADERSHIP", "Corporate Leadership", "ceo", "chief executive", "steps down", "appoints", "new ceo", "executive", "board of directors", "chairman", "founder resigns"),
		leaf("BUSINESS.OTHER", "Other Business", "company", "corporate", "startup", "firm", "enterprise", "business"),
	),
	domain("TECHNOLOGY", "Technology",
		leaf("TECHNOLOGY.AI", "Artificial Intelligence", "artificial intelligence", "ai model", "machine learning", "llm", "openai", "chatgpt", "chatbot", "neural", "generative ai", "deep learning", "anthropic", "gemini", "copilot"),
		leaf("TECHNOLOGY.CYBERSECURITY", "Cybersecurity", "data breach", "ransomware", "vulnerability", "hacked", "hack", "cyberattack", "cyberattacks", "malware", "cve", "phishing", "zero-day", "ddos", "exploit", "security flaw"),
		leaf("TECHNOLOGY.PRODUCT", "Product Launch", "new device", "smartphone", "gadget", "software update", "new app", "rolls out", "new feature", "operating system"),
		leaf("TECHNOLOGY.PLATFORMS", "Platforms & Social Media", "social media", "platform", "app store", "twitter", "meta", "facebook", "instagram", "tiktok", "youtube", "content moderation", "big tech"),
		leaf("TECHNOLOGY.OTHER", "Other Technology", "technology", "tech company", "gadget", "digital", "internet", "software"),
	),
	domain("SCIENCE", "Science",
		leaf("SCIENCE.SPACE", "Space & Astronomy", "space", "nasa", "spacex", "rocket", "satellite", "satellites", "starlink", "astronaut", "mars", "moon landing", "telescope", "asteroid", "exoplanet", "orbit", "spacecraft", "galaxy", "isro", "cosmic"),
		leaf("SCIENCE.RESEARCH", "Research & Discovery", "study finds", "study reveals", "study shows", "study of", "new study", "researchers", "scientists", "discovery", "breakthrough", "experiment", "peer-reviewed", "new species", "fossil", "physics", "genetics", "genome", "archaeologist", "archaeologists", "archaeology", "excavation", "ancient tomb"),
		leaf("SCIENCE.OTHER", "Other Science", "science", "scientific", "laboratory", "innovation"),
	),
	domain("ENVIRONMENT", "Environment",
		leaf("ENVIRONMENT.CLIMATE", "Climate Change", "climate change", "global warming", "carbon emissions", "greenhouse gas", "net zero", "cop28", "cop29", "paris agreement", "climate crisis", "decarbonization"),
		leaf("ENVIRONMENT.POLLUTION", "Pollution", "pollution", "air quality", "smog", "toxic", "contamination", "oil spill", "plastic waste", "emissions", "hazardous waste"),
		leaf("ENVIRONMENT.CONSERVATION", "Conservation & Wildlife", "wildlife", "endangered", "biodiversity", "deforestation", "conservation", "extinction", "ecosystem", "coral reef", "poaching", "rainforest"),
		leaf("ENVIRONMENT.OTHER", "Other Environment", "environment", "environmental", "sustainability", "ecological"),
	),
	domain("DISASTER", "Disaster",
		leaf("DISASTER.EARTHQUAKE", "Earthquake", "earthquake", "magnitude", "seismic", "tremor", "aftershock", "richter"),
		leaf("DISASTER.FLOOD", "Flood", "flood", "flooding", "flooded", "inundation", "deluge", "flash flood", "monsoon"),
		leaf("DISASTER.CYCLONE", "Cyclone / Storm", "cyclone", "hurricane", "typhoon", "tropical storm", "tornado", "landfall"),
		leaf("DISASTER.WILDFIRE", "Wildfire", "wildfire", "wildfires", "bushfire", "forest fire", "blaze", "wildland fire"),
		leaf("DISASTER.OTHER", "Other Disaster", "disaster", "landslide", "drought", "volcano", "volcanic", "eruption", "tsunami", "avalanche", "heatwave", "famine", "evacuation", "natural disaster", "state of emergency"),
	),
	domain("PUBLIC_HEALTH", "Public Health",
		leaf("PUBLIC_HEALTH.OUTBREAK", "Disease Outbreak", "outbreak", "epidemic", "pandemic", "virus", "infection", "cases surge", "covid", "measles", "ebola", "flu season", "contagion"),
		leaf("PUBLIC_HEALTH.DRUG", "Drug / Treatment", "drug approval", "vaccine", "clinical trial", "fda approval", "treatment", "therapy", "medication", "cure", "immunization"),
		leaf("PUBLIC_HEALTH.OTHER", "Other Public Health", "public health", "healthcare", "hospital", "mental health", "disease", "who warns", "health officials"),
	),
	domain("LEGAL", "Legal",
		leaf("LEGAL.COURT_RULING", "Court Ruling", "court", "ruling", "verdict", "supreme court", "judge", "lawsuit", "sued", "appeal", "trial", "convicted", "acquitted", "sentenced", "jury"),
		leaf("LEGAL.REGULATION", "Regulation", "regulation", "regulator", "compliance", "antitrust", "fined", "hefty fine", "watchdog", "regulatory", "penalty", "sec charges", "investigation into"),
		leaf("LEGAL.OTHER", "Other Legal", "legal", "law", "attorney", "prosecutor", "litigation", "settlement"),
	),
	domain("CRIME", "Crime & Justice",
		leaf("CRIME.VIOLENT", "Violent Crime", "murder", "homicide", "assault", "stabbing", "kidnapping", "manslaughter", "gang violence", "killed", "shot dead"),
		leaf("CRIME.FRAUD", "Fraud & Financial Crime", "fraud", "scam", "money laundering", "embezzlement", "ponzi", "insider trading", "financial crime", "counterfeit"),
		leaf("CRIME.TRAFFICKING", "Trafficking & Smuggling", "drug trafficking", "human trafficking", "smuggling", "cartel", "narcotics", "contraband", "drug bust"),
		leaf("CRIME.OTHER", "Other Crime", "crime", "criminal", "arrested", "police", "suspect", "robbery", "theft", "burglary", "raid"),
	),
	domain("CONFLICT", "Conflict & Security",
		leaf("CONFLICT.WAR", "War / Armed Conflict", "war", "airstrike", "troops", "ceasefire", "invasion", "missile", "offensive", "frontline", "combat", "shelling", "occupation", "insurgency"),
		leaf("CONFLICT.TERRORISM", "Terrorism", "terror", "terrorist", "bombing", "suicide bomber", "explosion", "extremist", "militant", "isis", "al-qaeda", "hostage"),
		leaf("CONFLICT.MILITARY", "Military & Defense", "military", "defense", "defence", "army", "navy", "air force", "weapons", "arms deal", "warship", "nato", "pentagon", "drone strike", "nuclear weapon"),
		leaf("CONFLICT.OTHER", "Other Conflict", "conflict", "security forces", "armed group", "border clash", "militia"),
	),
	domain("SOCIETY", "Society & Social Issues",
		leaf("SOCIETY.IMMIGRATION", "Immigration & Migration", "immigration", "migrant", "migrants", "refugee", "refugees", "asylum", "border crossing", "deportation", "visa", "citizenship", "undocumented"),
		leaf("SOCIETY.HUMAN_RIGHTS", "Human Rights", "human rights", "civil rights", "discrimination", "persecution", "genocide", "freedom of speech", "activist", "abuses", "amnesty international"),
		leaf("SOCIETY.LABOR", "Labor & Unions", "trade union", "labour union", "labor union", "workers union", "unions", "strike", "strikes", "striking", "workers", "collective bargaining", "picket", "walkout", "walk out", "wages", "pay dispute", "labor dispute", "industrial action", "minimum wage"),
		leaf("SOCIETY.RELIGION", "Religion", "religion", "religious", "church", "mosque", "temple", "pope", "vatican", "islam", "christian", "hindu", "faith", "pilgrimage", "clergy"),
		leaf("SOCIETY.OTHER", "Other Society", "social issue", "inequality", "poverty", "community", "gender", "lgbtq", "welfare", "demographic"),
	),
	domain("CULTURE", "Culture & Entertainment",
		leaf("CULTURE.ENTERTAINMENT", "Film & Television", "film", "movie", "hollywood", "box office", "tv show", "tv series", "streaming series", "netflix", "actor", "actress", "movie premiere", "oscars", "cinema", "season finale", "episode"),
		leaf("CULTURE.MUSIC", "Music", "music", "album", "new single", "concert", "concert tour", "world tour", "on tour", "singer", "grammy", "artist releases", "chart-topping", "musician"),
		leaf("CULTURE.CELEBRITY", "Celebrity & Royals", "celebrity", "wedding", "divorce", "royal", "prince", "princess", "duchess", "red carpet", "engagement", "dating", "split", "gossip", "reality star"),
		leaf("CULTURE.ARTS", "Arts & Books", "art", "artist", "museum", "exhibition", "book", "author", "novel", "literature", "gallery", "sculpture", "theatre", "festival", "musical", "broadway", "awards show", "opera", "ballet"),
		leaf("CULTURE.MEDIA", "Media & Press", "media", "journalist", "newspaper", "broadcaster", "press freedom", "publishing", "editorial", "journalism"),
		leaf("CULTURE.OTHER", "Other Culture", "culture", "cultural", "heritage", "entertainment", "viral", "trending"),
	),
	domain("LIFESTYLE", "Lifestyle",
		leaf("LIFESTYLE.FOOD", "Food & Drink", "recipe", "cuisine", "restaurant", "chef", "cooking", "dish", "menu", "food review", "michelin", "wine", "cocktail", "bakery", "dessert"),
		leaf("LIFESTYLE.WELLNESS", "Wellness & Fitness", "wellness", "fitness", "workout", "horoscope", "zodiac", "self-care", "meditation", "yoga", "mindfulness", "diet tips", "lifestyle tips"),
		leaf("LIFESTYLE.FASHION", "Fashion & Beauty", "fashion", "style", "beauty", "makeup", "skincare", "sunscreen", "outfit", "wardrobe", "cosmetics", "designer wear", "runway show"),
		leaf("LIFESTYLE.OTHER", "Other Lifestyle", "lifestyle", "home decor", "gardening", "parenting", "relationship advice", "hobby"),
	),
	domain("TRAVEL", "Travel & Hospitality",
		leaf("TRAVEL.HOSPITALITY", "Hotels & Hospitality", "hotel", "hotels", "resort", "resorts", "hospitality", "inn", "motel", "lodging", "guesthouse", "marriott", "hilton", "hyatt", "accommodation"),
		leaf("TRAVEL.TOURISM", "Tourism & Destinations", "tourism", "tourist", "travel", "vacation", "holiday destination", "cruise", "getaway", "sightseeing", "backpacking", "travel guide", "itinerary"),
		leaf("TRAVEL.OTHER", "Other Travel", "traveller", "traveler", "tour operator", "booking", "staycation"),
	),
	domain("SPORTS", "Sports",
		leaf("SPORTS.RESULT", "Match Result", "wins", "defeats", "final", "tournament", "championship", "match", "beat", "victory", "playoffs", "world cup", "league", "grand slam", "gold medal"),
		leaf("SPORTS.TRANSFER", "Transfer & Signings", "transfer", "signs", "signing", "contract", "loan deal", "free agent", "traded", "transfer window"),
		leaf("SPORTS.OTHER", "Other Sports", "sport", "sports", "athlete", "coach", "olympics", "football", "soccer", "cricket", "basketball", "tennis", "golf", "rugby", "formula 1", "goalkeeper", "striker", "midfielder", "doping", "world cup"),
	),
	domain("EDUCATION", "Education",
		leaf("EDUCATION.SCHOOLS", "Schools", "school", "schools", "students", "teachers", "curriculum", "classroom", "k-12", "exam", "pupils", "education board"),
		leaf("EDUCATION.HIGHER_ED", "Higher Education", "university", "universities", "college", "campus", "tuition", "faculty", "graduate", "academic", "phd", "student loan", "admissions"),
		leaf("EDUCATION.OTHER", "Other Education", "education", "educational", "learning", "edtech", "scholarship"),
	),
	domain("ENERGY", "Energy",
		leaf("ENERGY.OIL_GAS", "Oil & Gas", "crude oil", "natural gas", "opec", "barrel of oil", "petroleum", "oil pipeline", "refinery", "gas prices", "brent crude", "oil drilling"),
		leaf("ENERGY.RENEWABLES", "Renewable Energy", "solar", "wind power", "renewable", "clean energy", "hydrogen", "battery storage", "green energy", "offshore wind", "hydropower"),
		leaf("ENERGY.NUCLEAR", "Nuclear Power", "nuclear power", "nuclear plant", "reactor", "uranium", "nuclear energy", "power plant"),
		leaf("ENERGY.OTHER", "Other Energy", "energy", "electricity", "power grid", "utility", "blackout", "power outage", "fuel"),
	),
	domain("TRANSPORT", "Transport & Infrastructure",
		leaf("TRANSPORT.AVIATION", "Aviation", "flight", "airline", "airport", "plane crash", "aircraft", "boeing", "airbus", "aviation", "grounded", "runway", "turbulence"),
		leaf("TRANSPORT.RAIL", "Rail", "train", "railway", "rail", "metro", "derailment", "high-speed rail", "locomotive", "subway"),
		leaf("TRANSPORT.ROAD", "Road & Auto", "highway", "road accident", "car crash", "traffic", "collision", "motorway", "bus crash", "pile-up"),
		leaf("TRANSPORT.OTHER", "Other Transport", "transport", "transportation", "infrastructure", "shipping", "port", "cargo", "logistics", "bridge collapse", "ferry"),
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
