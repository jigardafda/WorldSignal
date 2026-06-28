package db

// Source mirrors the Prisma Source model plus the rich metadata added by
// MigrateContent. JSON field order matches the schema definition order so that
// REST endpoints dumping raw rows are byte-compatible with Prisma's serialization.
type Source struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	URL            string  `json:"url"`
	Country        *string `json:"country"`
	Region         *string `json:"region"`
	Language       *string `json:"language"`
	Category       *string `json:"category"`
	Priority       int     `json:"priority"`
	Credibility    float64 `json:"credibility"`
	CrawlFrequency int     `json:"crawlFrequency"`
	ParserType     string  `json:"parserType"`
	Enabled        bool    `json:"enabled"`
	Config         RawJSON `json:"config"`
	// Rich metadata (global-source expansion).
	WebsiteURL          *string     `json:"websiteUrl"`
	Languages           []string    `json:"languages"`
	GeographicScope     *string     `json:"geographicScope"`
	Industry            *string     `json:"industry"`
	Subcategory         *string     `json:"subcategory"`
	Publisher           *string     `json:"publisher"`
	OrgType             *string     `json:"orgType"`
	SourceType          *string     `json:"sourceType"`
	OfficialFeed        bool        `json:"officialFeed"`
	ContentType         *string     `json:"contentType"`
	UpdateFrequency     *string     `json:"updateFrequency"`
	Tags                []string    `json:"tags"`
	BiasRating          *string     `json:"biasRating"`
	HealthScore         *int        `json:"healthScore"`
	ValidationStatus    string      `json:"validationStatus"`
	LastValidatedAt     *PrismaTime `json:"lastValidatedAt"`
	LastValidationError *string     `json:"lastValidationError"`
	AvgResponseMs       *int        `json:"avgResponseMs"`
	Metadata            RawJSON     `json:"metadata"`

	LastFetchedAt *PrismaTime `json:"lastFetchedAt"`
	LastSuccessAt *PrismaTime `json:"lastSuccessAt"`
	LastFailureAt *PrismaTime `json:"lastFailureAt"`
	FailureCount  int         `json:"failureCount"`
	CreatedAt     PrismaTime  `json:"createdAt"`
	UpdatedAt     PrismaTime  `json:"updatedAt"`
}

// sourceColumns is the SELECT list in schema order (legacy columns first, then
// the rich-metadata columns, then fetch tracking + timestamps).
const sourceColumns = `"id","name","type","url","country","region","language","category","priority","credibility","crawlFrequency","parserType","enabled","config","websiteUrl","languages","geographicScope","industry","subcategory","publisher","orgType","sourceType","officialFeed","contentType","updateFrequency","tags","biasRating","healthScore","validationStatus","lastValidatedAt","lastValidationError","avgResponseMs","metadata","lastFetchedAt","lastSuccessAt","lastFailureAt","failureCount","createdAt","updatedAt"`

// ValidationLog is one row of a source's validation history.
type ValidationLog struct {
	ID           string      `json:"id"`
	SourceID     string      `json:"sourceId"`
	CheckedAt    PrismaTime  `json:"checkedAt"`
	OK           bool        `json:"ok"`
	HTTPStatus   *int        `json:"httpStatus"`
	ResponseMs   *int        `json:"responseMs"`
	ItemCount    *int        `json:"itemCount"`
	NewestItemAt *PrismaTime `json:"newestItemAt"`
	RedirectedTo *string     `json:"redirectedTo"`
	Error        *string     `json:"error"`
}
