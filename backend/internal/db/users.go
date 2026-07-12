package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/worldsignal/backend/internal/cuid"
)

// ErrDuplicateEmail is returned when a user email already exists.
var ErrDuplicateEmail = errors.New("email already exists")

// User mirrors the User table (password hash never leaves the db layer). A nil
// AccountID marks a platform-staff user (operator console); a non-nil AccountID
// binds the user to a tenant.
type User struct {
	ID           string
	Email        string
	Name         string
	PasswordHash string
	Role         string
	Status       string
	AccountID    *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

const userCols = `"id","email","name","passwordHash","role","status","accountId","createdAt","updatedAt"`

func scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.Role, &u.Status, &u.AccountID, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func isDup(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// CountUsers returns the number of users.
func (d *DB) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM "User"`).Scan(&n)
	return n, err
}

// CreateUser inserts a user.
func (d *DB) CreateUser(ctx context.Context, email, name, passwordHash, role string) (*User, error) {
	id := cuid.New()
	row := d.Pool.QueryRow(ctx,
		`INSERT INTO "User" ("id","email","name","passwordHash","role") VALUES ($1,$2,$3,$4,$5) RETURNING `+userCols,
		id, email, name, passwordHash, role)
	u, err := scanUser(row)
	if err != nil {
		if isDup(err) {
			return nil, ErrDuplicateEmail
		}
		return nil, err
	}
	return u, nil
}

// GetUserByEmail returns a user by email, or (nil,nil).
func (d *DB) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	u, err := scanUser(d.Pool.QueryRow(ctx, `SELECT `+userCols+` FROM "User" WHERE "email"=$1`, email))
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// GetUserByID returns a user by id, or (nil,nil).
func (d *DB) GetUserByID(ctx context.Context, id string) (*User, error) {
	u, err := scanUser(d.Pool.QueryRow(ctx, `SELECT `+userCols+` FROM "User" WHERE "id"=$1`, id))
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// ListUsers returns all users ordered by creation time.
func (d *DB) ListUsers(ctx context.Context) ([]*User, error) {
	// Bounded admin list: a hard cap guards against unbounded result sets.
	// High-cardinality lists (signals/articles/sources/…) use limit/offset/total.
	rows, err := d.Pool.Query(ctx, `SELECT `+userCols+` FROM "User" ORDER BY "createdAt" ASC, "email" ASC LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// UserPatch holds optional user updates. AccountID binds (or, when set to a
// pointer to "", unbinds) the user to a tenant.
type UserPatch struct {
	Name      *string
	Role      *string
	Status    *string
	AccountID *string
}

// UpdateUser applies a partial update and returns the row.
func (d *DB) UpdateUser(ctx context.Context, id string, p UserPatch) (*User, error) {
	sets := `"updatedAt"=now()`
	args := []any{id}
	if p.Name != nil {
		args = append(args, *p.Name)
		sets += `, "name"=$` + itoa(len(args))
	}
	if p.Role != nil {
		args = append(args, *p.Role)
		sets += `, "role"=$` + itoa(len(args))
	}
	if p.Status != nil {
		args = append(args, *p.Status)
		sets += `, "status"=$` + itoa(len(args))
	}
	if p.AccountID != nil {
		// A pointer to the empty string unbinds the user (platform staff).
		if *p.AccountID == "" {
			sets += `, "accountId"=NULL`
		} else {
			args = append(args, *p.AccountID)
			sets += `, "accountId"=$` + itoa(len(args))
		}
	}
	row := d.Pool.QueryRow(ctx, `UPDATE "User" SET `+sets+` WHERE "id"=$1 RETURNING `+userCols, args...)
	u, err := scanUser(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// UpdatePassword sets a user's password hash.
func (d *DB) UpdatePassword(ctx context.Context, id, passwordHash string) error {
	_, err := d.Pool.Exec(ctx, `UPDATE "User" SET "passwordHash"=$2, "updatedAt"=now() WHERE "id"=$1`, id, passwordHash)
	return err
}

// DeleteUser removes a user; returns false if it did not exist.
func (d *DB) DeleteUser(ctx context.Context, id string) (bool, error) {
	tag, err := d.Pool.Exec(ctx, `DELETE FROM "User" WHERE "id"=$1`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// --- sessions ---

// CreateSession stores a session token.
func (d *DB) CreateSession(ctx context.Context, userID, token string, expiresAt time.Time) error {
	_, err := d.Pool.Exec(ctx,
		`INSERT INTO "Session" ("id","token","userId","expiresAt") VALUES ($1,$2,$3,$4)`,
		cuid.New(), token, userID, expiresAt)
	return err
}

// UserForToken returns the active (non-expired) user for a session token, or nil.
func (d *DB) UserForToken(ctx context.Context, token string) (*User, error) {
	row := d.Pool.QueryRow(ctx,
		`SELECT `+prefixed("u", userCols)+` FROM "Session" s JOIN "User" u ON u."id"=s."userId"
		 WHERE s."token"=$1 AND s."expiresAt" > now()`, token)
	u, err := scanUser(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// DeleteSession removes a session token.
func (d *DB) DeleteSession(ctx context.Context, token string) error {
	_, err := d.Pool.Exec(ctx, `DELETE FROM "Session" WHERE "token"=$1`, token)
	return err
}

// prefixed rewrites a quoted column list to use a table alias prefix.
func prefixed(alias, cols string) string {
	out := ""
	for i := 0; i < len(cols); i++ {
		if cols[i] == '"' {
			// find closing quote
			j := i + 1
			for j < len(cols) && cols[j] != '"' {
				j++
			}
			out += alias + "." + cols[i:j+1]
			i = j
		} else {
			out += string(cols[i])
		}
	}
	return out
}
