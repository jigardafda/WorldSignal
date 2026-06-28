// The closed WorldSignal Taxonomy. Production tags MUST come from this list —
// the LLM is constrained to it, and the heuristic classifier matches keywords
// against the `keywords` field below.

export interface TaxonomyNode {
  code: string;
  label: string;
  description?: string;
  /** keywords used by the deterministic fallback classifier */
  keywords?: string[];
  aliases?: string[];
  children?: TaxonomyNode[];
}

export const TAXONOMY: TaxonomyNode[] = [
  {
    code: "POLITICS",
    label: "Politics",
    children: [
      { code: "POLITICS.ELECTIONS", label: "Elections", keywords: ["election", "ballot", "vote", "poll", "candidate", "campaign"] },
      { code: "POLITICS.POLICY", label: "Policy", keywords: ["policy", "legislation", "bill", "reform", "parliament", "congress"] },
      { code: "POLITICS.DIPLOMACY", label: "Diplomacy", keywords: ["diplomacy", "summit", "treaty", "sanction", "embassy", "foreign minister"] },
    ],
  },
  {
    code: "ECONOMY",
    label: "Economy",
    children: [
      { code: "ECONOMY.INFLATION", label: "Inflation", keywords: ["inflation", "cpi", "consumer price", "cost of living"] },
      { code: "ECONOMY.INTEREST_RATES", label: "Interest Rates", keywords: ["interest rate", "central bank", "rate hike", "basis points", "federal reserve", "rbi"] },
      { code: "ECONOMY.MARKETS", label: "Markets", keywords: ["stock market", "shares", "index", "nasdaq", "nifty", "sensex", "commodities", "crypto", "bitcoin"] },
      { code: "ECONOMY.JOBS", label: "Jobs & Employment", keywords: ["unemployment", "jobs report", "hiring", "labor market", "payroll"] },
    ],
  },
  {
    code: "BUSINESS",
    label: "Business",
    children: [
      { code: "BUSINESS.EARNINGS", label: "Earnings", keywords: ["earnings", "quarterly results", "revenue", "profit", "guidance"] },
      { code: "BUSINESS.MA", label: "Mergers & Acquisitions", keywords: ["acquisition", "merger", "takeover", "acquires", "buyout"] },
      { code: "BUSINESS.FUNDING", label: "Funding", keywords: ["funding", "series a", "series b", "raised", "venture", "valuation", "ipo"] },
      { code: "BUSINESS.LAYOFFS", label: "Layoffs", keywords: ["layoff", "job cuts", "restructuring", "redundancies"] },
    ],
  },
  {
    code: "TECHNOLOGY",
    label: "Technology",
    children: [
      { code: "TECHNOLOGY.AI", label: "Artificial Intelligence", keywords: ["artificial intelligence", "ai model", "machine learning", "llm", "openai", "chatbot", "neural"] },
      { code: "TECHNOLOGY.CYBERSECURITY", label: "Cybersecurity", keywords: ["data breach", "ransomware", "vulnerability", "hacked", "cyberattack", "malware", "cve"] },
      { code: "TECHNOLOGY.PRODUCT", label: "Product Launch", keywords: ["launches", "unveils", "release", "new device", "announced"] },
    ],
  },
  {
    code: "DISASTER",
    label: "Disaster",
    children: [
      { code: "DISASTER.EARTHQUAKE", label: "Earthquake", keywords: ["earthquake", "magnitude", "seismic", "tremor", "aftershock"] },
      { code: "DISASTER.FLOOD", label: "Flood", keywords: ["flood", "flooding", "inundation", "deluge"] },
      { code: "DISASTER.CYCLONE", label: "Cyclone / Storm", keywords: ["cyclone", "hurricane", "typhoon", "storm", "tornado"] },
      { code: "DISASTER.WILDFIRE", label: "Wildfire", keywords: ["wildfire", "bushfire", "forest fire", "blaze"] },
    ],
  },
  {
    code: "PUBLIC_HEALTH",
    label: "Public Health",
    children: [
      { code: "PUBLIC_HEALTH.OUTBREAK", label: "Disease Outbreak", keywords: ["outbreak", "epidemic", "pandemic", "virus", "infection", "cases surge"] },
      { code: "PUBLIC_HEALTH.DRUG", label: "Drug / Treatment", keywords: ["drug approval", "vaccine", "clinical trial", "fda approval", "treatment"] },
    ],
  },
  {
    code: "LEGAL",
    label: "Legal",
    children: [
      { code: "LEGAL.COURT_RULING", label: "Court Ruling", keywords: ["court", "ruling", "verdict", "supreme court", "judge", "lawsuit"] },
      { code: "LEGAL.REGULATION", label: "Regulation", keywords: ["regulation", "regulator", "compliance", "antitrust", "ban", "fine"] },
    ],
  },
  {
    code: "CONFLICT",
    label: "Conflict & Security",
    children: [
      { code: "CONFLICT.WAR", label: "War / Armed Conflict", keywords: ["war", "military", "airstrike", "troops", "ceasefire", "invasion", "missile"] },
      { code: "CONFLICT.TERRORISM", label: "Terrorism", keywords: ["terror", "bombing", "attack", "explosion", "shooting"] },
    ],
  },
  {
    code: "SPORTS",
    label: "Sports",
    children: [
      { code: "SPORTS.RESULT", label: "Match Result", keywords: ["wins", "defeats", "final", "tournament", "championship", "match"] },
      { code: "SPORTS.TRANSFER", label: "Transfer", keywords: ["transfer", "signs", "signing", "contract", "deal"] },
    ],
  },
  {
    code: "GENERAL",
    label: "General",
    children: [{ code: "GENERAL.OTHER", label: "Other / Uncategorized", keywords: [] }],
  },
];

/** Flattened list of every node (domains + leaves). */
export function flattenTaxonomy(nodes: TaxonomyNode[] = TAXONOMY): TaxonomyNode[] {
  const out: TaxonomyNode[] = [];
  for (const n of nodes) {
    out.push(n);
    if (n.children) out.push(...flattenTaxonomy(n.children));
  }
  return out;
}

/** Only leaf nodes (the assignable tags). */
export function leafTags(): TaxonomyNode[] {
  return flattenTaxonomy().filter((n) => !n.children);
}

/** The set of valid codes — used to validate LLM output. */
export const VALID_CODES: Set<string> = new Set(flattenTaxonomy().map((n) => n.code));

export const FALLBACK_CODE = "GENERAL.OTHER";
