// Package dbtest provides Postgres test helpers mirroring backend/src/test-utils/db.ts:
// connect to the test database, truncate application tables, and seed the taxonomy.
package dbtest

import (
	"context"
	"os"
	"testing"

	"github.com/worldsignal/backend/internal/cuid"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/taxonomy"
)

// URL returns the test database connection string.
func URL() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgresql://jigardafda@localhost:5432/worldsignal_test?sslmode=disable"
}

// tables are truncated in dependency-safe order (CASCADE handles the rest).
var tables = []string{
	"DeliveryEvent", "Subscription", "Subscriber", "SignalTag", "SignalArticle",
	"Signal", "Article", "RawItem", "Source", "TaxonomyTag",
}

// Connect opens a pool to the test DB, skipping the test if it is unreachable.
func Connect(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Connect(context.Background(), URL())
	if err != nil {
		t.Skipf("test database unavailable (%v); set TEST_DATABASE_URL", err)
	}
	t.Cleanup(d.Close)
	return d
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
