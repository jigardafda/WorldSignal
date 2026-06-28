package db

// Source mirrors the Prisma Source model. JSON field order matches the schema
// definition order so that REST endpoints dumping raw rows are byte-compatible
// with Prisma's serialization.
type Source struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	Type           string      `json:"type"`
	URL            string      `json:"url"`
	Country        *string     `json:"country"`
	Region         *string     `json:"region"`
	Language       *string     `json:"language"`
	Category       *string     `json:"category"`
	Priority       int         `json:"priority"`
	Credibility    float64     `json:"credibility"`
	CrawlFrequency int         `json:"crawlFrequency"`
	ParserType     string      `json:"parserType"`
	Enabled        bool        `json:"enabled"`
	Config         RawJSON     `json:"config"`
	LastFetchedAt  *PrismaTime `json:"lastFetchedAt"`
	LastSuccessAt  *PrismaTime `json:"lastSuccessAt"`
	LastFailureAt  *PrismaTime `json:"lastFailureAt"`
	FailureCount   int         `json:"failureCount"`
	CreatedAt      PrismaTime  `json:"createdAt"`
	UpdatedAt      PrismaTime  `json:"updatedAt"`
}

// sourceColumns is the SELECT list in schema order.
const sourceColumns = `"id","name","type","url","country","region","language","category","priority","credibility","crawlFrequency","parserType","enabled","config","lastFetchedAt","lastSuccessAt","lastFailureAt","failureCount","createdAt","updatedAt"`
