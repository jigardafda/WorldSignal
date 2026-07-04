package db

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Signal holds the scalar columns of a Signal needed by the API serializers.
type Signal struct {
	ID           string
	Title        string
	Summary      string
	WhatHappened *string
	WhyItMatters *string
	Status       string
	Severity     string
	Confidence   float64
	EventType    *string
	Country      *string
	SourceCount  int
	FirstSeenAt  time.Time
	LastSeenAt   time.Time
	// Deep-enrichment attributes.
	Region          *string
	City            *string
	Locality        *string
	GeoScope        *string
	Sentiment       *string
	SentimentScore  *float64
	Influence       *string
	Relevance       *float64
	Language        *string
	OriginalTitle   *string
	OriginalSummary *string
}

// SignalTagRow is a tag attached to a signal.
type SignalTagRow struct {
	Code       string
	Label      string
	Confidence float64
}

// SignalSourceRow is an article/source backing a signal.
type SignalSourceRow struct {
	Publisher   string
	URL         *string
	PublishedAt *time.Time
	Relation    string
}

// SignalAttrRow is one stored dictionary attribute value on a signal.
type SignalAttrRow struct {
	Key        string
	ValueCode  string
	ValueText  string
	ValueNum   *float64
	Confidence float64
}

// SignalAggregate is a signal with its tags, sources and dictionary attributes.
type SignalAggregate struct {
	Signal
	Tags       []SignalTagRow
	Sources    []SignalSourceRow
	Attributes []SignalAttrRow
}

// SignalFilter captures the query filters shared by REST and GraphQL.
type SignalFilter struct {
	Country       *string
	Status        *string
	MinConfidence *float64
	Since         *time.Time
	Search        *string
	Tags          []string
	// Deep-enrichment attribute filters.
	Region       *string
	GeoScope     *string
	Sentiment    *string
	Influence    *string
	MinRelevance *float64
	Industry     *string // matches a SignalAttribute industry code
	Entity       *string // matches a SignalAttribute entity name (exact)
	Limit        int
	Offset       int
}

const signalScalarCols = `"id","title","summary","whatHappened","whyItMatters","status","severity","confidence","eventType","country","sourceCount","firstSeenAt","lastSeenAt","region","city","locality","geoScope","sentiment","sentimentScore","influence","relevance","language","originalTitle","originalSummary"`

func scanSignal(row pgx.Row) (*Signal, error) {
	var s Signal
	err := row.Scan(&s.ID, &s.Title, &s.Summary, &s.WhatHappened, &s.WhyItMatters,
		&s.Status, &s.Severity, &s.Confidence, &s.EventType, &s.Country, &s.SourceCount,
		&s.FirstSeenAt, &s.LastSeenAt,
		&s.Region, &s.City, &s.Locality, &s.GeoScope, &s.Sentiment, &s.SentimentScore,
		&s.Influence, &s.Relevance, &s.Language, &s.OriginalTitle, &s.OriginalSummary)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// ListSignals returns signals matching the filter, ranked by relevance when a
// search term is present (else by lastSeenAt desc), each with tags and sources
// loaded.
//
// signalWhere builds the shared WHERE clause + args for the signal filter. The
// third return value is the SQL placeholder holding the raw search text (for
// ts_rank in ORDER BY), or "" when the filter has no search term.
func signalWhere(f SignalFilter) (string, []any, string) {
	var conds []string
	var args []any
	add := func(cond string, val any) {
		args = append(args, val)
		conds = append(conds, strings.Replace(cond, "?", "$"+itoa(len(args)), 1))
	}
	if f.Country != nil {
		add(`"country" = ?`, *f.Country)
	}
	if f.Status != nil {
		add(`"status" = ?::"SignalStatus"`, *f.Status)
	}
	if f.MinConfidence != nil {
		add(`"confidence" >= ?`, *f.MinConfidence)
	}
	if f.Region != nil {
		add(`"region" = ?`, *f.Region)
	}
	if f.GeoScope != nil {
		add(`"geoScope" = ?`, *f.GeoScope)
	}
	if f.Sentiment != nil {
		add(`"sentiment" = ?`, *f.Sentiment)
	}
	if f.Influence != nil {
		add(`"influence" = ?`, *f.Influence)
	}
	if f.MinRelevance != nil {
		add(`"relevance" >= ?`, *f.MinRelevance)
	}
	if f.Industry != nil {
		args = append(args, *f.Industry)
		p := "$" + itoa(len(args))
		conds = append(conds, `EXISTS (SELECT 1 FROM "SignalAttribute" sa WHERE sa."signalId"="Signal"."id" AND sa."key"='industry' AND sa."valueCode"=`+p+`)`)
	}
	if f.Entity != nil {
		args = append(args, *f.Entity)
		p := "$" + itoa(len(args))
		conds = append(conds, `EXISTS (SELECT 1 FROM "SignalAttribute" sa WHERE sa."signalId"="Signal"."id" AND sa."key"='entity' AND sa."valueText"=`+p+`)`)
	}
	if f.Since != nil {
		add(`"lastSeenAt" >= ?`, *f.Since)
	}
	qParam := ""
	if f.Search != nil && strings.TrimSpace(*f.Search) != "" {
		// Full-text (ranked, GIN-indexed) OR a trigram/substring fallback so
		// partial words and typos still surface results.
		args = append(args, strings.TrimSpace(*f.Search))
		qParam = "$" + itoa(len(args))
		args = append(args, "%"+strings.TrimSpace(*f.Search)+"%")
		like := "$" + itoa(len(args))
		conds = append(conds, `("searchVector" @@ websearch_to_tsquery('english', `+qParam+`) OR "title" ILIKE `+like+` OR "summary" ILIKE `+like+`)`)
	}
	if len(f.Tags) > 0 {
		args = append(args, f.Tags)
		p := "$" + itoa(len(args))
		conds = append(conds, `EXISTS (SELECT 1 FROM "SignalTag" st JOIN "TaxonomyTag" tt ON tt."id"=st."tagId" WHERE st."signalId"="Signal"."id" AND tt."code" = ANY(`+p+`))`)
	}
	if len(conds) == 0 {
		return "", args, qParam
	}
	return " WHERE " + strings.Join(conds, " AND "), args, qParam
}

// CountSignals returns the number of signals matching the filter.
func (d *DB) CountSignals(ctx context.Context, f SignalFilter) (int, error) {
	where, args, _ := signalWhere(f)
	var n int
	err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM "Signal"`+where, args...).Scan(&n)
	return n, err
}

func (d *DB) ListSignals(ctx context.Context, f SignalFilter) ([]*SignalAggregate, error) {
	where, args, qParam := signalWhere(f)
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	// Rank by text relevance when searching; otherwise most-recent first.
	order := `ORDER BY "lastSeenAt" DESC`
	if qParam != "" {
		order = `ORDER BY ts_rank("searchVector", websearch_to_tsquery('english', ` + qParam + `)) DESC, "lastSeenAt" DESC`
	}
	args = append(args, limit)
	limitP := "$" + itoa(len(args))
	args = append(args, f.Offset)
	offsetP := "$" + itoa(len(args))

	q := `SELECT ` + signalScalarCols + ` FROM "Signal"` + where + ` ` + order + ` LIMIT ` + limitP + ` OFFSET ` + offsetP
	rows, err := d.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	var signals []*Signal
	for rows.Next() {
		s, err := scanSignal(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		signals = append(signals, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]*SignalAggregate, 0, len(signals))
	for _, s := range signals {
		agg, err := d.loadAggregate(ctx, s)
		if err != nil {
			return nil, err
		}
		out = append(out, agg)
	}
	return out, nil
}

// GetSignal returns a single signal aggregate, or (nil, nil) if not found.
func (d *DB) GetSignal(ctx context.Context, id string) (*SignalAggregate, error) {
	row := d.Pool.QueryRow(ctx, `SELECT `+signalScalarCols+` FROM "Signal" WHERE "id"=$1`, id)
	s, err := scanSignal(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return d.loadAggregate(ctx, s)
}

func (d *DB) loadAggregate(ctx context.Context, s *Signal) (*SignalAggregate, error) {
	tags, err := d.signalTags(ctx, s.ID)
	if err != nil {
		return nil, err
	}
	sources, err := d.signalSources(ctx, s.ID)
	if err != nil {
		return nil, err
	}
	attrs, err := d.SignalAttributes(ctx, s.ID)
	if err != nil {
		return nil, err
	}
	return &SignalAggregate{Signal: *s, Tags: tags, Sources: sources, Attributes: attrs}, nil
}

// SignalAttributes loads the stored dictionary attributes for a signal, ordered
// deterministically by key then value.
func (d *DB) SignalAttributes(ctx context.Context, signalID string) ([]SignalAttrRow, error) {
	rows, err := d.Pool.Query(ctx,
		`SELECT "key","valueCode","valueText","valueNum","confidence"
		 FROM "SignalAttribute" WHERE "signalId"=$1 ORDER BY "key","valueCode","valueText"`, signalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SignalAttrRow{}
	for rows.Next() {
		var r SignalAttrRow
		if err := rows.Scan(&r.Key, &r.ValueCode, &r.ValueText, &r.ValueNum, &r.Confidence); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// signalTags loads tags for a signal. Order matches Prisma's relation load
// (SignalTag rows in physical/primary-key order, joined to TaxonomyTag).
func (d *DB) signalTags(ctx context.Context, signalID string) ([]SignalTagRow, error) {
	rows, err := d.Pool.Query(ctx,
		`SELECT tt."code", tt."label", st."confidence"
		 FROM "SignalTag" st JOIN "TaxonomyTag" tt ON tt."id"=st."tagId"
		 WHERE st."signalId"=$1 ORDER BY st."tagId" ASC`, signalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SignalTagRow{}
	for rows.Next() {
		var r SignalTagRow
		if err := rows.Scan(&r.Code, &r.Label, &r.Confidence); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// signalSources loads article/source links for a signal, ordered by addedAt.
func (d *DB) signalSources(ctx context.Context, signalID string) ([]SignalSourceRow, error) {
	rows, err := d.Pool.Query(ctx,
		`SELECT src."name", a."canonicalUrl", a."publishedAt", sa."relationType"
		 FROM "SignalArticle" sa
		 JOIN "Article" a ON a."id"=sa."articleId"
		 JOIN "Source" src ON src."id"=a."sourceId"
		 WHERE sa."signalId"=$1 ORDER BY sa."addedAt" ASC, a."id" ASC`, signalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SignalSourceRow{}
	for rows.Next() {
		var r SignalSourceRow
		if err := rows.Scan(&r.Publisher, &r.URL, &r.PublishedAt, &r.Relation); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
