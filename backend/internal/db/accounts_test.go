package db_test

import (
	"context"
	"testing"

	"github.com/worldsignal/backend/internal/cuid"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
)

func TestAccountValidators(t *testing.T) {
	for _, s := range []string{"ACTIVE", "SUSPENDED", "DELETED"} {
		if !db.ValidAccountStatus(s) {
			t.Fatalf("status %q should be valid", s)
		}
	}
	if db.ValidAccountStatus("NOPE") {
		t.Fatal("unknown status should be invalid")
	}
	for _, p := range []string{"FREE", "PRO", "ENTERPRISE"} {
		if !db.ValidAccountPlan(p) {
			t.Fatalf("plan %q should be valid", p)
		}
	}
	if db.ValidAccountPlan("NOPE") {
		t.Fatal("unknown plan should be invalid")
	}
}

func TestAccountCRUD(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()

	// Reset re-seeds the default account; EnsureDefaultAccount is idempotent.
	if err := d.EnsureDefaultAccount(ctx); err != nil {
		t.Fatalf("ensure default: %v", err)
	}
	def, err := d.GetAccount(ctx, db.DefaultAccountID)
	if err != nil || def == nil {
		t.Fatalf("default account missing: %v %v", def, err)
	}
	if def.Slug != "default" || def.Status != "ACTIVE" || def.Plan != "FREE" {
		t.Fatalf("default account fields: %+v", def)
	}

	// Create with an explicit plan, then one defaulting to FREE.
	acme, err := d.CreateAccount(ctx, cuid.New(), "Acme Corp", "acme", "PRO")
	if err != nil {
		t.Fatalf("create acme: %v", err)
	}
	if acme.Plan != "PRO" {
		t.Fatalf("plan want PRO got %q", acme.Plan)
	}
	globex, err := d.CreateAccount(ctx, cuid.New(), "Globex", "globex", "")
	if err != nil {
		t.Fatalf("create globex: %v", err)
	}
	if globex.Plan != "FREE" {
		t.Fatalf("empty plan should default to FREE, got %q", globex.Plan)
	}

	// Duplicate slug is rejected.
	if _, err := d.CreateAccount(ctx, cuid.New(), "Acme Two", "acme", "FREE"); err != db.ErrDuplicateSlug {
		t.Fatalf("dup slug want ErrDuplicateSlug got %v", err)
	}

	// Lookups.
	if got, _ := d.GetAccount(ctx, acme.ID); got == nil || got.Name != "Acme Corp" {
		t.Fatalf("get by id: %+v", got)
	}
	if got, _ := d.GetAccountBySlug(ctx, "globex"); got == nil || got.ID != globex.ID {
		t.Fatalf("get by slug: %+v", got)
	}
	if got, _ := d.GetAccount(ctx, "missing"); got != nil {
		t.Fatalf("missing account should be nil, got %+v", got)
	}
	if got, _ := d.GetAccountBySlug(ctx, "missing"); got != nil {
		t.Fatalf("missing slug should be nil, got %+v", got)
	}

	// List returns all three (default + acme + globex).
	all, err := d.ListAccounts(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("want 3 accounts, got %d", len(all))
	}

	// Partial updates.
	name, status, plan := "Acme Inc", "SUSPENDED", "ENTERPRISE"
	up, err := d.UpdateAccount(ctx, acme.ID, db.AccountPatch{Name: &name, Status: &status, Plan: &plan})
	if err != nil || up == nil {
		t.Fatalf("update: %v %v", up, err)
	}
	if up.Name != "Acme Inc" || up.Status != "SUSPENDED" || up.Plan != "ENTERPRISE" {
		t.Fatalf("update fields: %+v", up)
	}
	// Empty patch just bumps updatedAt and returns the row.
	if noop, err := d.UpdateAccount(ctx, acme.ID, db.AccountPatch{}); err != nil || noop == nil {
		t.Fatalf("noop update: %v %v", noop, err)
	}
	// Updating a missing account returns (nil,nil).
	if miss, err := d.UpdateAccount(ctx, "missing", db.AccountPatch{Name: &name}); err != nil || miss != nil {
		t.Fatalf("missing update should be nil,nil got %v %v", miss, err)
	}
}

func TestAccountDBErrors(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()

	if _, err := d.Pool.Exec(ctx, `ALTER TABLE "Account" RENAME TO "Account__h"`); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = d.Pool.Exec(ctx, `ALTER TABLE "Account__h" RENAME TO "Account"`) }()

	if _, err := d.ListAccounts(ctx); err == nil {
		t.Fatal("ListAccounts should error when the table is gone")
	}
	if _, err := d.CreateAccount(ctx, cuid.New(), "x", "x", "FREE"); err == nil {
		t.Fatal("CreateAccount should error when the table is gone")
	}
	if _, err := d.GetAccount(ctx, "x"); err == nil {
		t.Fatal("GetAccount should error when the table is gone")
	}
}

func TestListAPIKeysByAccountError(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	if _, err := d.Pool.Exec(ctx, `ALTER TABLE "ApiKey" RENAME TO "ApiKey__h"`); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = d.Pool.Exec(ctx, `ALTER TABLE "ApiKey__h" RENAME TO "ApiKey"`) }()
	if _, err := d.ListAPIKeysByAccount(ctx, db.DefaultAccountID); err == nil {
		t.Fatal("ListAPIKeysByAccount should error when the table is gone")
	}
}

func TestListAPIKeysByAccount(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()

	acme, err := d.CreateAccount(ctx, cuid.New(), "Acme", "acme", "FREE")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	// A key with no explicit account lands on the default account.
	kDefault, err := d.CreateAPIKey(ctx, cuid.New(), db.CreateAPIKeyInput{Name: "def", Hash: "h1", Prefix: "wsk_a", Scopes: []string{"signals:read"}, RateLimitPerMin: 10})
	if err != nil {
		t.Fatalf("create default key: %v", err)
	}
	if kDefault.AccountID != db.DefaultAccountID {
		t.Fatalf("empty account should default, got %q", kDefault.AccountID)
	}
	// A key scoped to acme.
	if _, err := d.CreateAPIKey(ctx, cuid.New(), db.CreateAPIKeyInput{AccountID: acme.ID, Name: "acme", Hash: "h2", Prefix: "wsk_b", Scopes: []string{"signals:read"}, RateLimitPerMin: 10}); err != nil {
		t.Fatalf("create acme key: %v", err)
	}

	acmeKeys, err := d.ListAPIKeysByAccount(ctx, acme.ID)
	if err != nil {
		t.Fatalf("list acme keys: %v", err)
	}
	if len(acmeKeys) != 1 || acmeKeys[0].Name != "acme" {
		t.Fatalf("acme keys: %+v", acmeKeys)
	}
	defKeys, _ := d.ListAPIKeysByAccount(ctx, db.DefaultAccountID)
	if len(defKeys) != 1 || defKeys[0].Name != "def" {
		t.Fatalf("default keys: %+v", defKeys)
	}
}
