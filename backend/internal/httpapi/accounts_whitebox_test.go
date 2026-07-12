package httpapi

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
)

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Acme Corp":       "acme-corp",
		"  Big  Co  ":     "big-co",
		"under_score":     "under-score",
		"Trailing---":     "trailing",
		"!!!":             "",
		"Mix3d 99 Things": "mix3d-99-things",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Fatalf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestAccountForKeyBranches exercises the error/edge branches of accountForKey
// that the HTTP path can't reach (the middleware always sets a valid tenant).
func TestAccountForKeyBranches(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	s := &Server{DB: d, SigningSecret: "s"}

	// No tenant context → 500.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/account", nil)
	s.accountForKey(rec, req)
	if rec.Code != 500 {
		t.Fatalf("no tenant want 500 got %d", rec.Code)
	}

	// Tenant set but account missing → 404.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/v1/account", nil).WithContext(withTenant(context.Background(), "ghost"))
	s.accountForKey(rec, req)
	if rec.Code != 404 {
		t.Fatalf("missing account want 404 got %d", rec.Code)
	}

	// DB error (Account table renamed away) → 500.
	ctx := context.Background()
	if _, err := d.Pool.Exec(ctx, `ALTER TABLE "Account" RENAME TO "Account__h"`); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = d.Pool.Exec(ctx, `ALTER TABLE "Account__h" RENAME TO "Account"`) }()
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/v1/account", nil).WithContext(withTenant(ctx, db.DefaultAccountID))
	s.accountForKey(rec, req)
	if rec.Code != 500 {
		t.Fatalf("db error want 500 got %d", rec.Code)
	}
}
