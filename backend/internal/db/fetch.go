package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/worldsignal/backend/internal/cuid"
)

// SourceForFetch holds the fields needed to fetch a source.
type SourceForFetch struct {
	ID      string
	Name    string
	URL     string
	Enabled bool
}

// GetSourceForFetch loads a source for fetching (nil if absent).
func (d *DB) GetSourceForFetch(ctx context.Context, id string) (*SourceForFetch, error) {
	var s SourceForFetch
	err := d.Pool.QueryRow(ctx, `SELECT "id","name","url","enabled" FROM "Source" WHERE "id"=$1`, id).
		Scan(&s.ID, &s.Name, &s.URL, &s.Enabled)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// RawItemExists reports whether a (sourceId, sourceGuid) raw item already exists.
func (d *DB) RawItemExists(ctx context.Context, sourceID, sourceGuid string) (bool, error) {
	var exists bool
	err := d.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM "RawItem" WHERE "sourceId"=$1 AND "sourceGuid"=$2)`, sourceID, sourceGuid).Scan(&exists)
	return exists, err
}

// NewRawItem is the data for inserting a RawItem.
type NewRawItem struct {
	SourceID    string
	SourceGuid  *string
	RawURL      *string
	RawTitle    string
	RawContent  string
	RawPayload  []byte
	PublishedAt *time.Time
}

// CreateRawItem inserts a PENDING RawItem and returns its id ("" on unique race).
func (d *DB) CreateRawItem(ctx context.Context, r NewRawItem) (string, error) {
	id := cuid.New()
	var got string
	err := d.Pool.QueryRow(ctx,
		`INSERT INTO "RawItem" ("id","sourceId","sourceGuid","rawUrl","rawTitle","rawContent","rawPayload","publishedAt","status")
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'PENDING')
		 ON CONFLICT ("sourceId","sourceGuid") DO NOTHING RETURNING "id"`,
		id, r.SourceID, r.SourceGuid, r.RawURL, r.RawTitle, r.RawContent, r.RawPayload, r.PublishedAt).Scan(&got)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return got, nil
}

// MarkSourceFetchSuccess records a successful fetch.
func (d *DB) MarkSourceFetchSuccess(ctx context.Context, id string, at time.Time) error {
	// A successful fetch resets the failure counter and lifts any cooldown.
	_, err := d.Pool.Exec(ctx,
		`UPDATE "Source" SET "lastFetchedAt"=$2,"lastSuccessAt"=$2,"failureCount"=0,
		 "cooldownUntil"=NULL,"updatedAt"=now() WHERE "id"=$1`, id, at)
	return err
}

// MarkSourceFetchFailure records a failed fetch. When the consecutive failure
// count reaches threshold, the source is placed in cooldown for cooldown
// (skipped by the scheduler until it elapses, then retried automatically).
func (d *DB) MarkSourceFetchFailure(ctx context.Context, id string, at time.Time, threshold int, cooldown time.Duration, reason string) error {
	cooldownUntil := at.Add(cooldown) // applied only once the threshold is reached
	var errPtr any
	if reason != "" {
		errPtr = reason
	}
	_, err := d.Pool.Exec(ctx,
		`UPDATE "Source" SET
		   "lastFetchedAt"=$2,"lastFailureAt"=$2,
		   "failureCount"="failureCount"+1,
		   "cooldownUntil"=CASE WHEN "failureCount"+1 >= $3 THEN $4::timestamptz ELSE "cooldownUntil" END,
		   "lastValidationError"=$5,
		   "updatedAt"=now()
		 WHERE "id"=$1`,
		id, at, threshold, cooldownUntil, errPtr)
	return err
}

// ListDueSources returns ids of enabled, non-cooled-down sources whose crawl
// interval has elapsed, highest priority first, capped at limit. The due filter
// runs in SQL so it scales to thousands of sources.
func (d *DB) ListDueSources(ctx context.Context, now time.Time, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 5000
	}
	rows, err := d.Pool.Query(ctx, `
SELECT "id" FROM "Source"
WHERE "enabled"=true
  AND ("cooldownUntil" IS NULL OR "cooldownUntil" <= $1)
  AND ("lastFetchedAt" IS NULL OR $1 - "lastFetchedAt" >= ("crawlFrequency" * interval '1 second'))
ORDER BY "priority" ASC, "lastFetchedAt" ASC NULLS FIRST
LIMIT $2`, now, limit)
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
