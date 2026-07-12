// Package dbtest provides Postgres test helpers mirroring backend/src/test-utils/db.ts:
// connect to the test database, truncate application tables, and seed the taxonomy.
package dbtest

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/worldsignal/backend/internal/auth"
	"github.com/worldsignal/backend/internal/cuid"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/taxonomy"
)

// URL returns the test database connection string.
func URL() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgresql://worldsignal:worldsignal@localhost:5432/worldsignal_test?sslmode=disable"
}

// tables are truncated in dependency-safe order (CASCADE handles the rest).
var tables = []string{
	"DigestQueue", "DeliveryEvent", "Subscription", "Subscriber", "SignalTag", "SignalArticle",
	"Signal", "Article", "RawItem", "SourceValidationLog", "Source", "TaxonomyTag",
	"ApiKeyUsage", "ApiKey", "LLMKey", "EmailConnector", "AuditLog", "Session", "TeamMember", "Team", "User", "Account",
}

// Connect opens a pool to the test DB (ensuring auth tables exist), skipping the
// test if the database is unreachable.
func Connect(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Connect(context.Background(), URL())
	if err != nil {
		t.Skipf("test database unavailable (%v); set TEST_DATABASE_URL", err)
	}
	if err := d.MigrateAuth(context.Background()); err != nil {
		d.Close()
		t.Fatalf("migrate auth: %v", err)
	}
	if err := d.MigrateContent(context.Background()); err != nil {
		d.Close()
		t.Fatalf("migrate content: %v", err)
	}
	t.Cleanup(d.Close)
	return d
}

// SeedUser creates a user with the given role and returns it.
func SeedUser(t *testing.T, d *db.DB, email, role string) *db.User {
	t.Helper()
	hash, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatal(err)
	}
	u, err := d.CreateUser(context.Background(), email, email, hash, role)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u
}

// AuthToken creates a user with the given role plus an active session, returning
// the bearer token (and the user).
func AuthToken(t *testing.T, d *db.DB, role string) (string, *db.User) {
	t.Helper()
	u := SeedUser(t, d, role+"-"+cuid.New()+"@test.local", role)
	token, err := auth.GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := d.CreateSession(context.Background(), u.ID, token, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("create session: %v", err)
	}
	return token, u
}

// AuthTokenTenant creates an account-scoped (tenant) user with an active session,
// returning the bearer token and the user. The account must already exist.
func AuthTokenTenant(t *testing.T, d *db.DB, role, accountID string) (string, *db.User) {
	t.Helper()
	u := SeedUser(t, d, "tenant-"+cuid.New()+"@test.local", role)
	if _, err := d.UpdateUser(context.Background(), u.ID, db.UserPatch{AccountID: &accountID}); err != nil {
		t.Fatalf("bind user to account: %v", err)
	}
	token, err := auth.GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := d.CreateSession(context.Background(), u.ID, token, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("create session: %v", err)
	}
	return token, u
}

// Reset truncates all application tables and restarts identities.
func Reset(t *testing.T, d *db.DB) {
	t.Helper()
	list := ""
	for i, tbl := range tables {
		if i > 0 {
			list += ", "
		}
		list += `"` + tbl + `"`
	}
	if _, err := d.Pool.Exec(context.Background(), "TRUNCATE TABLE "+list+" RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	// Re-seed the default tenant so account-scoped foreign keys (ApiKey.accountId)
	// resolve after a truncate.
	if err := d.EnsureDefaultAccount(context.Background()); err != nil {
		t.Fatalf("ensure default account: %v", err)
	}
}

// SeedTaxonomy inserts the full taxonomy tree (domains then their leaves).
func SeedTaxonomy(t *testing.T, d *db.DB) {
	t.Helper()
	ctx := context.Background()
	var insert func(n taxonomy.Node, parentID *string)
	insert = func(n taxonomy.Node, parentID *string) {
		id := cuid.New()
		aliases := []string{}
		_, err := d.Pool.Exec(ctx,
			`INSERT INTO "TaxonomyTag" ("id","code","label","parentId","aliases","active") VALUES ($1,$2,$3,$4,$5,true)`,
			id, n.Code, n.Label, parentID, aliases)
		if err != nil {
			t.Fatalf("seed tag %s: %v", n.Code, err)
		}
		for _, c := range n.Children {
			cid := id
			insert(c, &cid)
		}
	}
	for _, dmn := range taxonomy.Taxonomy {
		insert(dmn, nil)
	}
}
