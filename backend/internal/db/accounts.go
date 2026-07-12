package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// DefaultAccountID is the tenant every pre-multitenancy row is backfilled to. It
// is created by MigrateContent and re-ensured by the test harness so account-
// scoped foreign keys (ApiKey.accountId) always resolve.
const DefaultAccountID = "acct_default"

// Account is a SaaS tenant: it owns API keys, subscriptions and (later) billing.
// The global signal corpus stays shared; an account only owns its lens onto it.
type Account struct {
	ID        string
	Name      string
	Slug      string
	Status    string
	Plan      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Account status + plan vocabularies. Kept as closed sets so the API can reject
// unknown values instead of persisting typos.
const (
	AccountActive    = "ACTIVE"
	AccountSuspended = "SUSPENDED"
	AccountDeleted   = "DELETED"
)

// ValidAccountStatus reports whether s is a known account status.
func ValidAccountStatus(s string) bool {
	return s == AccountActive || s == AccountSuspended || s == AccountDeleted
}

// ValidAccountPlan reports whether p is a known billing plan.
func ValidAccountPlan(p string) bool {
	return p == "FREE" || p == "PRO" || p == "ENTERPRISE"
}

// ErrDuplicateSlug is returned when an account slug already exists.
var ErrDuplicateSlug = errors.New("account slug already exists")

const accountCols = `"id","name","slug","status","plan","createdAt","updatedAt"`

func scanAccount(row pgx.Row) (*Account, error) {
	var a Account
	if err := row.Scan(&a.ID, &a.Name, &a.Slug, &a.Status, &a.Plan, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return nil, err
	}
	return &a, nil
}

// EnsureDefaultAccount inserts the default tenant if it is missing (idempotent).
func (d *DB) EnsureDefaultAccount(ctx context.Context) error {
	_, err := d.Pool.Exec(ctx,
		`INSERT INTO "Account" ("id","name","slug") VALUES ($1,'Default Account','default') ON CONFLICT ("id") DO NOTHING`,
		DefaultAccountID)
	return err
}

// CreateAccount inserts an account, mapping a unique-slug violation to
// ErrDuplicateSlug.
func (d *DB) CreateAccount(ctx context.Context, id, name, slug, plan string) (*Account, error) {
	if plan == "" {
		plan = "FREE"
	}
	a, err := scanAccount(d.Pool.QueryRow(ctx,
		`INSERT INTO "Account" ("id","name","slug","plan") VALUES ($1,$2,$3,$4) RETURNING `+accountCols,
		id, name, slug, plan))
	if err != nil {
		if isDup(err) {
			return nil, ErrDuplicateSlug
		}
		return nil, err
	}
	return a, nil
}

// GetAccount returns an account by id, or (nil,nil) if absent.
func (d *DB) GetAccount(ctx context.Context, id string) (*Account, error) {
	a, err := scanAccount(d.Pool.QueryRow(ctx, `SELECT `+accountCols+` FROM "Account" WHERE "id"=$1`, id))
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return a, err
}

// GetAccountBySlug returns an account by slug, or (nil,nil) if absent.
func (d *DB) GetAccountBySlug(ctx context.Context, slug string) (*Account, error) {
	a, err := scanAccount(d.Pool.QueryRow(ctx, `SELECT `+accountCols+` FROM "Account" WHERE "slug"=$1`, slug))
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return a, err
}

// ListAccounts returns all accounts, newest first (bounded admin list).
func (d *DB) ListAccounts(ctx context.Context) ([]*Account, error) {
	rows, err := d.Pool.Query(ctx, `SELECT `+accountCols+` FROM "Account" ORDER BY "createdAt" DESC, "name" ASC LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Account{}
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// AccountPatch holds optional account updates.
type AccountPatch struct {
	Name   *string
	Status *string
	Plan   *string
}

// UpdateAccount applies a partial update and returns the row (or nil,nil if the
// account does not exist).
func (d *DB) UpdateAccount(ctx context.Context, id string, p AccountPatch) (*Account, error) {
	sets := `"updatedAt"=now()`
	args := []any{id}
	if p.Name != nil {
		args = append(args, *p.Name)
		sets += `, "name"=$` + itoa(len(args))
	}
	if p.Status != nil {
		args = append(args, *p.Status)
		sets += `, "status"=$` + itoa(len(args))
	}
	if p.Plan != nil {
		args = append(args, *p.Plan)
		sets += `, "plan"=$` + itoa(len(args))
	}
	a, err := scanAccount(d.Pool.QueryRow(ctx, `UPDATE "Account" SET `+sets+` WHERE "id"=$1 RETURNING `+accountCols, args...))
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return a, err
}
