package db

import (
	"context"
	"time"

	"github.com/worldsignal/backend/internal/jsonx"
)

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
