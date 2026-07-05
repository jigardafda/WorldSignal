package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// ApiKey is a credential for the public REST API. Only the SHA-256 hash of the
// raw key is stored; KeyPrefix is safe to display.
type ApiKey struct {
	ID              string
	Name            string
	KeyHash         string
	KeyPrefix       string
	Scopes          []string
	RateLimitPerMin int
	Enabled         bool
	ExpiresAt       *time.Time
	LastUsedAt      *time.Time
	RequestCount    int64
	CreatedBy       *string
	CreatedAt       time.Time
}

const apiKeyCols = `"id","name","keyHash","keyPrefix","scopes","rateLimitPerMin","enabled","expiresAt","lastUsedAt","requestCount","createdBy","createdAt"`

func scanAPIKey(row pgx.Row) (*ApiKey, error) {
	var k ApiKey
	if err := row.Scan(&k.ID, &k.Name, &k.KeyHash, &k.KeyPrefix, &k.Scopes, &k.RateLimitPerMin,
		&k.Enabled, &k.ExpiresAt, &k.LastUsedAt, &k.RequestCount, &k.CreatedBy, &k.CreatedAt); err != nil {
		return nil, err
	}
	return &k, nil
}

// ListAPIKeys returns all keys, newest first (hashes included; callers must not
// expose them).
func (d *DB) ListAPIKeys(ctx context.Context) ([]*ApiKey, error) {
	rows, err := d.Pool.Query(ctx, `SELECT `+apiKeyCols+` FROM "ApiKey" ORDER BY "createdAt" DESC LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*ApiKey{}
	for rows.Next() {
		k, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// GetAPIKeyByHash looks up a key by its SHA-256 hash, or (nil, nil) if absent.
func (d *DB) GetAPIKeyByHash(ctx context.Context, hash string) (*ApiKey, error) {
	k, err := scanAPIKey(d.Pool.QueryRow(ctx, `SELECT `+apiKeyCols+` FROM "ApiKey" WHERE "keyHash"=$1`, hash))
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return k, err
}

// CreateAPIKeyInput carries the fields to persist for a new key.
type CreateAPIKeyInput struct {
	Name, Hash, Prefix string
	Scopes             []string
	RateLimitPerMin    int
	ExpiresAt          *time.Time
	CreatedBy          *string
}

// CreateAPIKey inserts a key and returns it.
func (d *DB) CreateAPIKey(ctx context.Context, id string, in CreateAPIKeyInput) (*ApiKey, error) {
	if in.Scopes == nil {
		in.Scopes = []string{}
	}
	return scanAPIKey(d.Pool.QueryRow(ctx, `
INSERT INTO "ApiKey" ("id","name","keyHash","keyPrefix","scopes","rateLimitPerMin","expiresAt","createdBy")
VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
RETURNING `+apiKeyCols,
		id, in.Name, in.Hash, in.Prefix, in.Scopes, in.RateLimitPerMin, in.ExpiresAt, in.CreatedBy))
}

// SetAPIKeyEnabled toggles a key, returning it (or nil,nil if unknown).
func (d *DB) SetAPIKeyEnabled(ctx context.Context, id string, enabled bool) (*ApiKey, error) {
	k, err := scanAPIKey(d.Pool.QueryRow(ctx,
		`UPDATE "ApiKey" SET "enabled"=$2 WHERE "id"=$1 RETURNING `+apiKeyCols, id, enabled))
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return k, err
}

// DeleteAPIKey removes a key, returning whether a row was deleted.
func (d *DB) DeleteAPIKey(ctx context.Context, id string) (bool, error) {
	tag, err := d.Pool.Exec(ctx, `DELETE FROM "ApiKey" WHERE "id"=$1`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// TouchAPIKey records that a key was used (best-effort; callers ignore the error).
func (d *DB) TouchAPIKey(ctx context.Context, id string, at time.Time) error {
	_, err := d.Pool.Exec(ctx,
		`UPDATE "ApiKey" SET "lastUsedAt"=$2,"requestCount"="requestCount"+1 WHERE "id"=$1`, id, at)
	return err
}

// AllowAPIRequest atomically increments the fixed-window (per-minute) counter for
// a key and reports whether the request is within the limit, along with how many
// requests remain in the window. windowStart must be the minute-truncated time.
// Stale windows for the key are pruned in the same statement.
func (d *DB) AllowAPIRequest(ctx context.Context, keyID string, limit int, windowStart time.Time) (allowed bool, remaining int, err error) {
	var count int
	qerr := d.Pool.QueryRow(ctx, `
WITH pruned AS (
  DELETE FROM "ApiKeyUsage" WHERE "keyId"=$1 AND "windowStart" < $2
), upserted AS (
  INSERT INTO "ApiKeyUsage" ("keyId","windowStart","count") VALUES ($1,$2,1)
  ON CONFLICT ("keyId","windowStart") DO UPDATE SET "count"="ApiKeyUsage"."count"+1
  RETURNING "count"
)
SELECT "count" FROM upserted`, keyID, windowStart).Scan(&count)
	if qerr != nil {
		return false, 0, qerr
	}
	rem := limit - count
	if rem < 0 {
		rem = 0
	}
	return count <= limit, rem, nil
}
