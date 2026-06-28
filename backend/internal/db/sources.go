package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func scanSource(row pgx.Row) (*Source, error) {
	var s Source
	var (
		config, metadata                      []byte
		lastValidated                         *time.Time
		lastFetched, lastSuccess, lastFailure *time.Time
		createdAt, updatedAt                  time.Time
	)
	err := row.Scan(
		&s.ID, &s.Name, &s.Type, &s.URL, &s.Country, &s.Region, &s.Language, &s.Category,
		&s.Priority, &s.Credibility, &s.CrawlFrequency, &s.ParserType, &s.Enabled, &config,
		&s.WebsiteURL, &s.Languages, &s.GeographicScope, &s.Industry, &s.Subcategory, &s.Publisher,
		&s.OrgType, &s.SourceType, &s.OfficialFeed, &s.ContentType, &s.UpdateFrequency, &s.Tags,
		&s.BiasRating, &s.HealthScore, &s.ValidationStatus, &lastValidated, &s.LastValidationError,
		&s.AvgResponseMs, &metadata,
		&lastFetched, &lastSuccess, &lastFailure, &s.FailureCount, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	s.Config = RawJSON(config)
	s.Metadata = RawJSON(metadata)
	s.LastValidatedAt = NewTimePtr(lastValidated)
	s.LastFetchedAt = NewTimePtr(lastFetched)
	s.LastSuccessAt = NewTimePtr(lastSuccess)
	s.LastFailureAt = NewTimePtr(lastFailure)
	s.CreatedAt = NewTime(createdAt)
	s.UpdatedAt = NewTime(updatedAt)
	return &s, nil
}

// SourceFilter filters and paginates the source registry.
type SourceFilter struct {
	Search           *string
	Country          *string
	Region           *string
	Language         *string
	Scope            *string
	Industry         *string
	OrgType          *string
	SourceType       *string
	ValidationStatus *string
	Tag              *string
	Enabled          *bool
	Limit            int
	Offset           int
}

// sourceWhere builds the WHERE clause + args for a SourceFilter.
func sourceWhere(f SourceFilter) (string, []any) {
	var conds []string
	var args []any
	add := func(cond string, val any) {
		args = append(args, val)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if f.Search != nil {
		add(`("name" ILIKE '%%'||$%d||'%%' OR "publisher" ILIKE '%%'||$%[1]d||'%%' OR "url" ILIKE '%%'||$%[1]d||'%%')`, *f.Search)
	}
	if f.Country != nil {
		add(`"country" = $%d`, *f.Country)
	}
	if f.Region != nil {
		add(`"region" = $%d`, *f.Region)
	}
	if f.Language != nil {
		add(`($%d = ANY("languages") OR "language" = $%[1]d)`, *f.Language)
	}
	if f.Scope != nil {
		add(`"geographicScope" = $%d`, *f.Scope)
	}
	if f.Industry != nil {
		add(`"industry" = $%d`, *f.Industry)
	}
	if f.OrgType != nil {
		add(`"orgType" = $%d`, *f.OrgType)
	}
	if f.SourceType != nil {
		add(`"sourceType" = $%d`, *f.SourceType)
	}
	if f.ValidationStatus != nil {
		add(`"validationStatus" = $%d`, *f.ValidationStatus)
	}
	if f.Tag != nil {
		add(`$%d = ANY("tags")`, *f.Tag)
	}
	if f.Enabled != nil {
		add(`"enabled" = $%d`, *f.Enabled)
	}
	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// ListSources returns all sources ordered by priority asc then name asc
// (unfiltered, unpaginated) — retained for callers that need the full set.
func (d *DB) ListSources(ctx context.Context) ([]*Source, error) {
	rows, _, err := d.ListSourcesFiltered(ctx, SourceFilter{Limit: 100000})
	return rows, err
}

// ListSourcesFiltered returns matching sources plus the total count for the
// filter (ignoring pagination), ordered by priority asc, healthScore desc, name.
func (d *DB) ListSourcesFiltered(ctx context.Context, f SourceFilter) ([]*Source, int, error) {
	where, args := sourceWhere(f)

	var total int
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM "Source"`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	q := `SELECT ` + sourceColumns + ` FROM "Source"` + where +
		` ORDER BY "priority" ASC, "healthScore" DESC NULLS LAST, "name" ASC` +
		fmt.Sprintf(` LIMIT %d OFFSET %d`, limit, f.Offset)
	rows, err := d.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []*Source{}
	for rows.Next() {
		s, err := scanSource(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, s)
	}
	return out, total, rows.Err()
}

// CountSources returns the number of sources matching a filter.
func (d *DB) CountSources(ctx context.Context, f SourceFilter) (int, error) {
	where, args := sourceWhere(f)
	var n int
	err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM "Source"`+where, args...).Scan(&n)
	return n, err
}

// GetSource returns a single source by id, or (nil, nil) if not found.
func (d *DB) GetSource(ctx context.Context, id string) (*Source, error) {
	row := d.Pool.QueryRow(ctx, `SELECT `+sourceColumns+` FROM "Source" WHERE "id"=$1`, id)
	s, err := scanSource(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return s, err
}

// SourceCoverage aggregates the registry across coverage dimensions.
func (d *DB) SourceCoverage(ctx context.Context) (map[string][]Bucket, error) {
	dims := map[string]string{
		"byRegion":     `"region"`,
		"byScope":      `"geographicScope"`,
		"byOrgType":    `"orgType"`,
		"byValidation": `"validationStatus"`,
		"byIndustry":   `"industry"`,
		"byCountry":    `"country"`,
		"bySourceType": `"sourceType"`,
	}
	out := make(map[string][]Bucket, len(dims)+1)
	for key, col := range dims {
		bs, err := d.groupCount(ctx, col)
		if err != nil {
			return nil, err
		}
		out[key] = bs
	}
	// Languages are an array column — unnest.
	bs, err := d.groupCountRaw(ctx, `SELECT l AS key, count(*) FROM "Source", unnest("languages") l GROUP BY l ORDER BY count(*) DESC`)
	if err != nil {
		return nil, err
	}
	out["byLanguage"] = bs
	return out, nil
}

func (d *DB) groupCount(ctx context.Context, col string) ([]Bucket, error) {
	q := fmt.Sprintf(`SELECT COALESCE(%s,'(none)') AS key, count(*) FROM "Source" GROUP BY %s ORDER BY count(*) DESC`, col, col)
	return d.groupCountRaw(ctx, q)
}

func (d *DB) groupCountRaw(ctx context.Context, q string) ([]Bucket, error) {
	rows, err := d.Pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Bucket
	for rows.Next() {
		var b Bucket
		if err := rows.Scan(&b.Key, &b.Count); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ListValidationLogs returns a source's validation history, newest first.
func (d *DB) ListValidationLogs(ctx context.Context, sourceID string, limit int) ([]ValidationLog, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := d.Pool.Query(ctx, `
SELECT "id","sourceId","checkedAt","ok","httpStatus","responseMs","itemCount","newestItemAt","redirectedTo","error"
FROM "SourceValidationLog" WHERE "sourceId"=$1 ORDER BY "checkedAt" DESC LIMIT $2`, sourceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ValidationLog{}
	for rows.Next() {
		var l ValidationLog
		var checked time.Time
		var newest *time.Time
		if err := rows.Scan(&l.ID, &l.SourceID, &checked, &l.OK, &l.HTTPStatus, &l.ResponseMs, &l.ItemCount, &newest, &l.RedirectedTo, &l.Error); err != nil {
			return nil, err
		}
		l.CheckedAt = NewTime(checked)
		l.NewestItemAt = NewTimePtr(newest)
		out = append(out, l)
	}
	return out, rows.Err()
}

// ValidationOutcome carries the result of a single (re)validation to persist.
type ValidationOutcome struct {
	OK           bool
	HTTPStatus   int
	ResponseMs   int
	ItemCount    int
	NewestItemAt *time.Time
	RedirectedTo string
	HealthScore  int
	Error        string
}

// RecordValidation updates a source's validation state and appends a log row.
func (d *DB) RecordValidation(ctx context.Context, sourceID, logID string, o ValidationOutcome) error {
	status := "INVALID"
	var errPtr any
	if o.OK {
		status = "VALID"
	} else if o.Error != "" {
		errPtr = o.Error
	}
	if _, err := d.Pool.Exec(ctx, `
UPDATE "Source" SET
  "validationStatus"=$2, "healthScore"=$3, "avgResponseMs"=$4,
  "lastValidatedAt"=now(), "lastValidationError"=$5,
  "lastSuccessAt"=CASE WHEN $6 THEN now() ELSE "lastSuccessAt" END,
  "lastFailureAt"=CASE WHEN $6 THEN "lastFailureAt" ELSE now() END,
  "updatedAt"=now()
WHERE "id"=$1`, sourceID, status, o.HealthScore, o.ResponseMs, errPtr, o.OK); err != nil {
		return err
	}
	var newest any
	if o.NewestItemAt != nil {
		newest = *o.NewestItemAt
	}
	var redir any
	if o.RedirectedTo != "" {
		redir = o.RedirectedTo
	}
	_, err := d.Pool.Exec(ctx, `
INSERT INTO "SourceValidationLog" ("id","sourceId","checkedAt","ok","httpStatus","responseMs","itemCount","newestItemAt","redirectedTo","error")
VALUES ($1,$2,now(),$3,$4,$5,$6,$7,$8,$9)`,
		logID, sourceID, o.OK, o.HTTPStatus, o.ResponseMs, o.ItemCount, newest, redir, errPtr)
	return err
}
