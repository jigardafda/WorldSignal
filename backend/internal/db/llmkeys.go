package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// LLMKey is an admin-managed provider API key. The secret itself is stored only
// as ciphertext; callers decrypt on demand. KeyLast4 is safe to display.
type LLMKey struct {
	ID            string      `json:"id"`
	Provider      string      `json:"provider"`
	Label         string      `json:"label"`
	KeyCiphertext string      `json:"-"` // never serialized
	KeyLast4      string      `json:"keyLast4"`
	Model         *string     `json:"model"`
	IsActive      bool        `json:"isActive"`
	Status        string      `json:"status"`
	LastTestedAt  *PrismaTime `json:"lastTestedAt"`
	LastError     *string     `json:"lastError"`
	CreatedBy     *string     `json:"createdBy"`
	CreatedAt     PrismaTime  `json:"createdAt"`
	UpdatedAt     PrismaTime  `json:"updatedAt"`
}

const llmKeyColumns = `"id","provider","label","keyCiphertext","keyLast4","model","isActive","status","lastTestedAt","lastError","createdBy","createdAt","updatedAt"`

func scanLLMKey(row pgx.Row) (*LLMKey, error) {
	var k LLMKey
	var lastTested *time.Time
	var createdAt, updatedAt time.Time
	if err := row.Scan(&k.ID, &k.Provider, &k.Label, &k.KeyCiphertext, &k.KeyLast4, &k.Model,
		&k.IsActive, &k.Status, &lastTested, &k.LastError, &k.CreatedBy, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	k.LastTestedAt = NewTimePtr(lastTested)
	k.CreatedAt = NewTime(createdAt)
	k.UpdatedAt = NewTime(updatedAt)
	return &k, nil
}

// ListLLMKeys returns all keys, active first then newest.
func (d *DB) ListLLMKeys(ctx context.Context) ([]*LLMKey, error) {
	rows, err := d.Pool.Query(ctx, `SELECT `+llmKeyColumns+` FROM "LLMKey" ORDER BY "isActive" DESC, "createdAt" DESC LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*LLMKey{}
	for rows.Next() {
		k, err := scanLLMKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// GetLLMKey returns one key by id, or (nil, nil) if absent.
func (d *DB) GetLLMKey(ctx context.Context, id string) (*LLMKey, error) {
	k, err := scanLLMKey(d.Pool.QueryRow(ctx, `SELECT `+llmKeyColumns+` FROM "LLMKey" WHERE "id"=$1`, id))
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return k, err
}

// GetActiveLLMKey returns the active key for a provider, or (nil, nil) if none.
func (d *DB) GetActiveLLMKey(ctx context.Context, provider string) (*LLMKey, error) {
	k, err := scanLLMKey(d.Pool.QueryRow(ctx,
		`SELECT `+llmKeyColumns+` FROM "LLMKey" WHERE "provider"=$1 AND "isActive"=true ORDER BY "updatedAt" DESC LIMIT 1`, provider))
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return k, err
}

// CreateLLMKeyInput carries the fields to persist for a new key.
type CreateLLMKeyInput struct {
	Provider, Label, Ciphertext, Last4 string
	Model                              *string
	CreatedBy                          *string
}

// CreateLLMKey inserts a key (inactive, untested) and returns it.
func (d *DB) CreateLLMKey(ctx context.Context, id string, in CreateLLMKeyInput) (*LLMKey, error) {
	return scanLLMKey(d.Pool.QueryRow(ctx, `
INSERT INTO "LLMKey" ("id","provider","label","keyCiphertext","keyLast4","model","createdBy","updatedAt")
VALUES ($1,$2,$3,$4,$5,$6,$7,now())
RETURNING `+llmKeyColumns,
		id, in.Provider, in.Label, in.Ciphertext, in.Last4, in.Model, in.CreatedBy))
}

// SetActiveLLMKey makes one key the sole active key for its provider.
func (d *DB) SetActiveLLMKey(ctx context.Context, id string) (*LLMKey, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var provider string
	if err := tx.QueryRow(ctx, `SELECT "provider" FROM "LLMKey" WHERE "id"=$1`, id).Scan(&provider); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE "LLMKey" SET "isActive"=false,"updatedAt"=now() WHERE "provider"=$1 AND "id"<>$2`, provider, id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE "LLMKey" SET "isActive"=true,"updatedAt"=now() WHERE "id"=$1`, id); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return d.GetLLMKey(ctx, id)
}

// UpdateLLMKeyStatus records the outcome of a validation test.
func (d *DB) UpdateLLMKeyStatus(ctx context.Context, id, status string, errMsg *string) error {
	_, err := d.Pool.Exec(ctx,
		`UPDATE "LLMKey" SET "status"=$2,"lastError"=$3,"lastTestedAt"=now(),"updatedAt"=now() WHERE "id"=$1`,
		id, status, errMsg)
	return err
}

// DeleteLLMKey removes a key, returning whether a row was deleted.
func (d *DB) DeleteLLMKey(ctx context.Context, id string) (bool, error) {
	tag, err := d.Pool.Exec(ctx, `DELETE FROM "LLMKey" WHERE "id"=$1`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
