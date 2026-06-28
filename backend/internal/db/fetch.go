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
	_, err := d.Pool.Exec(ctx,
		`UPDATE "Source" SET "lastFetchedAt"=$2,"lastSuccessAt"=$2,"failureCount"=0,"updatedAt"=now() WHERE "id"=$1`, id, at)
	return err
}

// MarkSourceFetchFailure records a failed fetch.
func (d *DB) MarkSourceFetchFailure(ctx context.Context, id string, at time.Time) error {
	_, err := d.Pool.Exec(ctx,
		`UPDATE "Source" SET "lastFetchedAt"=$2,"lastFailureAt"=$2,"failureCount"="failureCount"+1,"updatedAt"=now() WHERE "id"=$1`, id, at)
	return err
}
