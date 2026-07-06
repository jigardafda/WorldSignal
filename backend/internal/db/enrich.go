package db

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/worldsignal/backend/internal/jsonx"
)

// validText strips invalid UTF-8 byte sequences from text bound for a Postgres
// text column. Upstream feed titles and crawled bodies can be cut mid-character
// (e.g. a partial Devanagari/Cyrillic multi-byte rune); Postgres then rejects
// the whole write with `invalid byte sequence for encoding "UTF8"`. This is most
// visible on the translated path, which stores source-language originals.
func validText(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	return strings.ToValidUTF8(s, "")
}

func validTextPtr(s *string) *string {
	if s == nil {
		return nil
	}
	v := validText(*s)
	return &v
}

// EnrichLink is one article/source backing a signal, for enrichment.
type EnrichLink struct {
	RelationType string
	Title        string
	Body         *string
	Summary      *string
	CanonicalURL *string
	SourceID     string
	Credibility  float64
	SourceName   string
}

// SignalForEnrich is the data needed to enrich a signal.
type SignalForEnrich struct {
	PublishedAt *time.Time
	Metadata    []byte
	Links       []EnrichLink
}

// LoadSignalForEnrich loads a signal's publishedAt/metadata and its links
// (articles + sources) ordered by addedAt asc. Returns nil if the signal is
// absent or has no links.
func (d *DB) LoadSignalForEnrich(ctx context.Context, signalID string) (*SignalForEnrich, error) {
	var s SignalForEnrich
	err := d.Pool.QueryRow(ctx, `SELECT "publishedAt","metadata" FROM "Signal" WHERE "id"=$1`, signalID).
		Scan(&s.PublishedAt, &s.Metadata)
	if err != nil {
		return nil, err
	}
	rows, err := d.Pool.Query(ctx,
		`SELECT sa."relationType", a."title", a."body", a."summary", a."canonicalUrl", a."sourceId", src."credibility", src."name"
		 FROM "SignalArticle" sa
		 JOIN "Article" a ON a."id"=sa."articleId"
		 JOIN "Source" src ON src."id"=a."sourceId"
		 WHERE sa."signalId"=$1 ORDER BY sa."addedAt" ASC`, signalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var l EnrichLink
		if err := rows.Scan(&l.RelationType, &l.Title, &l.Body, &l.Summary, &l.CanonicalURL, &l.SourceID, &l.Credibility, &l.SourceName); err != nil {
			return nil, err
		}
		s.Links = append(s.Links, l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(s.Links) == 0 {
		return nil, nil
	}
	return &s, nil
}

// UncategorizedSignalIDs returns the ids of signals whose primary category is
// missing or GENERAL (eventType is NULL or in the GENERAL domain) — the set that
// benefits from re-enrichment after a taxonomy/classifier change. Newest first;
// limit <= 0 means no limit.
func (d *DB) UncategorizedSignalIDs(ctx context.Context, limit int) ([]string, error) {
	q := `SELECT "id" FROM "Signal"
	       WHERE "eventType" IS NULL OR "eventType" LIKE 'GENERAL%'
	       ORDER BY "lastSeenAt" DESC`
	var args []any
	if limit > 0 {
		q += ` LIMIT $1`
		args = append(args, limit)
	}
	rows, err := d.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// SignalText is a signal's id and the narrative text used for reclassification.
// Body is the longest linked-article body (richer than the summary), or empty.
type SignalText struct {
	ID      string
	Title   string
	Summary string
	Body    string
}

// UncategorizedSignalTexts loads the id/title/summary and the richest linked
// article body of signals whose primary category is missing or GENERAL — the
// input for an in-place recategorization backfill. Newest first; limit <= 0 = all.
func (d *DB) UncategorizedSignalTexts(ctx context.Context, limit int) ([]SignalText, error) {
	q := `SELECT s."id", s."title", COALESCE(s."summary",''),
	              COALESCE((
	                SELECT a."body" FROM "SignalArticle" sa
	                JOIN "Article" a ON a."id"=sa."articleId"
	                WHERE sa."signalId"=s."id"
	                ORDER BY length(COALESCE(a."body",'')) DESC LIMIT 1
	              ),'')
	       FROM "Signal" s
	       WHERE s."eventType" IS NULL OR s."eventType" LIKE 'GENERAL%'
	       ORDER BY s."lastSeenAt" DESC`
	var args []any
	if limit > 0 {
		q += ` LIMIT $1`
		args = append(args, limit)
	}
	rows, err := d.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SignalText
	for rows.Next() {
		var s SignalText
		if err := rows.Scan(&s.ID, &s.Title, &s.Summary, &s.Body); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// CategoryTag is a taxonomy category code with a confidence, for recategorization.
type CategoryTag struct {
	Code       string
	Confidence float64
}

// SetSignalCategory updates a signal's primary category (eventType) and replaces
// its `category` SignalAttribute rows in one transaction, leaving other attribute
// keys (industry, entity, …) untouched. tags[0] is the primary category.
func (d *DB) SetSignalCategory(ctx context.Context, signalID string, tags []CategoryTag) error {
	if len(tags) == 0 {
		return nil
	}
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`UPDATE "Signal" SET "eventType"=$2,"updatedAt"=now() WHERE "id"=$1`, signalID, tags[0].Code); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM "SignalAttribute" WHERE "signalId"=$1 AND "key"='category'`, signalID); err != nil {
		return err
	}
	for _, t := range tags {
		// valueText is NOT NULL; category values live in valueCode, so store '' (as
		// ApplyEnrichment does), not NULL.
		if _, err := tx.Exec(ctx,
			`INSERT INTO "SignalAttribute" ("signalId","key","valueCode","valueText","valueNum","confidence")
			 VALUES ($1,'category',$2,'',NULL,$3)`, signalID, t.Code, t.Confidence); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// TagIDsByCodes resolves taxonomy tag ids by code.
func (d *DB) TagIDsByCodes(ctx context.Context, codes []string) (map[string]string, error) {
	out := map[string]string{}
	if len(codes) == 0 {
		return out, nil
	}
	rows, err := d.Pool.Query(ctx, `SELECT "code","id" FROM "TaxonomyTag" WHERE "code" = ANY($1)`, codes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var code, id string
		if err := rows.Scan(&code, &id); err != nil {
			return nil, err
		}
		out[code] = id
	}
	return out, rows.Err()
}

// EnrichmentUpdate carries the enrichment results to persist. The geo/sentiment/
// influence/relevance pointers are applied with COALESCE semantics: a nil value
// leaves the existing column untouched (so a later enrichment that fails to
// detect, say, a country never erases one a prior pass found).
type EnrichmentUpdate struct {
	Title        string
	Summary      string
	WhatHappened *string
	WhyItMatters *string
	Severity     string
	Confidence   float64
	Status       string
	EventType    *string
	PublishedAt  time.Time
	Metadata     map[string]any
	Tags         []TagAssignment

	// Deep-enrichment attributes (all optional; nil = keep existing).
	Country        *string
	Region         *string
	City           *string
	Locality       *string
	GeoScope       *string
	Sentiment      *string
	SentimentScore *float64
	Influence      *string
	Relevance      *float64
	Language       *string // detected source language (ISO 639-1); narrative is English
	// Original-language title/summary, kept for display alongside the English
	// translation. Set (overwritten, including to NULL) on every enrichment.
	OriginalTitle   *string
	OriginalSummary *string
	// Attributes fully replaces the signal's SignalAttribute rows.
	Attributes []SignalAttr
}

// TagAssignment is a resolved tag id with confidence.
type TagAssignment struct {
	TagID      string
	Confidence float64
}

// SignalAttr is one normalized dictionary attribute value for a signal.
type SignalAttr struct {
	Key        string
	ValueCode  string
	ValueText  string
	ValueNum   *float64
	Confidence float64
}

// ApplyEnrichment updates the signal and replaces its tags in one transaction.
func (d *DB) ApplyEnrichment(ctx context.Context, signalID string, u EnrichmentUpdate) error {
	// Guarantee valid UTF-8 for every text column so a truncated non-Latin rune
	// upstream can't fail the whole enrichment write.
	u.Title = validText(u.Title)
	u.Summary = validText(u.Summary)
	u.WhatHappened = validTextPtr(u.WhatHappened)
	u.WhyItMatters = validTextPtr(u.WhyItMatters)
	u.Region = validTextPtr(u.Region)
	u.City = validTextPtr(u.City)
	u.Locality = validTextPtr(u.Locality)
	u.OriginalTitle = validTextPtr(u.OriginalTitle)
	u.OriginalSummary = validTextPtr(u.OriginalSummary)
	for i := range u.Attributes {
		u.Attributes[i].ValueText = validText(u.Attributes[i].ValueText)
		u.Attributes[i].ValueCode = validText(u.Attributes[i].ValueCode)
	}

	meta, err := jsonx.Marshal(u.Metadata)
	if err != nil {
		return err
	}
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`UPDATE "Signal" SET "title"=$2,"summary"=$3,"whatHappened"=$4,"whyItMatters"=$5,
		 "severity"=$6::"Severity","confidence"=$7,"status"=$8::"SignalStatus","eventType"=$9,
		 "publishedAt"=$10,"metadata"=$11,
		 "country"=COALESCE($12,"country"),"region"=COALESCE($13,"region"),
		 "city"=COALESCE($14,"city"),"locality"=COALESCE($15,"locality"),
		 "geoScope"=COALESCE($16,"geoScope"),"sentiment"=COALESCE($17,"sentiment"),
		 "sentimentScore"=COALESCE($18,"sentimentScore"),"influence"=COALESCE($19,"influence"),
		 "relevance"=COALESCE($20,"relevance"),"language"=COALESCE($21,"language"),
		 "originalTitle"=$22,"originalSummary"=$23,"updatedAt"=now() WHERE "id"=$1`,
		signalID, u.Title, u.Summary, u.WhatHappened, u.WhyItMatters, u.Severity, u.Confidence,
		u.Status, u.EventType, u.PublishedAt, meta,
		u.Country, u.Region, u.City, u.Locality, u.GeoScope, u.Sentiment, u.SentimentScore,
		u.Influence, u.Relevance, u.Language, u.OriginalTitle, u.OriginalSummary); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM "SignalTag" WHERE "signalId"=$1`, signalID); err != nil {
		return err
	}
	for _, tg := range u.Tags {
		if _, err := tx.Exec(ctx,
			`INSERT INTO "SignalTag" ("signalId","tagId","confidence") VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`,
			signalID, tg.TagID, tg.Confidence); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM "SignalAttribute" WHERE "signalId"=$1`, signalID); err != nil {
		return err
	}
	for _, a := range u.Attributes {
		if _, err := tx.Exec(ctx,
			`INSERT INTO "SignalAttribute" ("signalId","key","valueCode","valueText","valueNum","confidence")
			 VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`,
			signalID, a.Key, a.ValueCode, a.ValueText, a.ValueNum, a.Confidence); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
