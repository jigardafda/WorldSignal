package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/worldsignal/backend/internal/cuid"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
)

func TestAPIKeyStore(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	by := "admin"

	k, err := d.CreateAPIKey(ctx, cuid.New(), db.CreateAPIKeyInput{
		Name: "Prod", Hash: "hash-abc", Prefix: "wsk_abc", Scopes: []string{"signals:read", "stats:read"},
		RateLimitPerMin: 60, CreatedBy: &by,
	})
	if err != nil || !k.Enabled || k.RequestCount != 0 || len(k.Scopes) != 2 {
		t.Fatalf("create: %+v err=%v", k, err)
	}

	got, _ := d.GetAPIKeyByHash(ctx, "hash-abc")
	if got == nil || got.ID != k.ID {
		t.Fatalf("lookup by hash: %+v", got)
	}
	if miss, err := d.GetAPIKeyByHash(ctx, "nope"); miss != nil || err != nil {
		t.Fatalf("missing hash should be nil,nil: %+v %v", miss, err)
	}

	list, _ := d.ListAPIKeys(ctx)
	if len(list) != 1 {
		t.Fatalf("list: %d", len(list))
	}

	// Touch records usage.
	now := time.Now()
	if err := d.TouchAPIKey(ctx, k.ID, now); err != nil {
		t.Fatal(err)
	}
	got, _ = d.GetAPIKeyByHash(ctx, "hash-abc")
	if got.RequestCount != 1 || got.LastUsedAt == nil {
		t.Fatalf("touch: count=%d last=%v", got.RequestCount, got.LastUsedAt)
	}

	// Disable.
	dis, _ := d.SetAPIKeyEnabled(ctx, k.ID, false)
	if dis == nil || dis.Enabled {
		t.Fatalf("disable: %+v", dis)
	}
	if x, err := d.SetAPIKeyEnabled(ctx, "missing", true); x != nil || err != nil {
		t.Fatalf("disable missing: %+v %v", x, err)
	}

	// Delete.
	ok, err := d.DeleteAPIKey(ctx, k.ID)
	if err != nil || !ok {
		t.Fatalf("delete: %v %v", ok, err)
	}
	if ok, _ := d.DeleteAPIKey(ctx, k.ID); ok {
		t.Fatal("double delete should be false")
	}
}

func TestAPIKeyRateLimit(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	k, err := d.CreateAPIKey(ctx, cuid.New(), db.CreateAPIKeyInput{Name: "rl", Hash: "h", Prefix: "wsk_h", Scopes: []string{"signals:read"}, RateLimitPerMin: 3})
	if err != nil {
		t.Fatal(err)
	}
	win := time.Now().Truncate(time.Minute)

	// First 3 allowed with decreasing remaining; 4th blocked.
	for i := 1; i <= 3; i++ {
		allowed, rem, err := d.AllowAPIRequest(ctx, k.ID, 3, win)
		if err != nil || !allowed {
			t.Fatalf("req %d should be allowed: allowed=%v err=%v", i, allowed, err)
		}
		if rem != 3-i {
			t.Fatalf("req %d remaining want %d got %d", i, 3-i, rem)
		}
	}
	allowed, rem, _ := d.AllowAPIRequest(ctx, k.ID, 3, win)
	if allowed || rem != 0 {
		t.Fatalf("4th request should be blocked: allowed=%v rem=%d", allowed, rem)
	}

	// A new window resets the counter and prunes the old one.
	next := win.Add(time.Minute)
	allowed, rem, _ = d.AllowAPIRequest(ctx, k.ID, 3, next)
	if !allowed || rem != 2 {
		t.Fatalf("new window should reset: allowed=%v rem=%d", allowed, rem)
	}
	var windows int
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM "ApiKeyUsage" WHERE "keyId"=$1`, k.ID).Scan(&windows); err != nil {
		t.Fatal(err)
	}
	if windows != 1 {
		t.Fatalf("stale window not pruned: %d rows", windows)
	}
}

func TestAPIKeyErrorPaths(t *testing.T) {
	d := closed(t)
	ctx := context.Background()
	now := time.Now()
	mustErr := func(name string, err error) {
		if err == nil {
			t.Fatalf("%s: expected error on closed pool", name)
		}
	}
	_, err := d.ListAPIKeys(ctx)
	mustErr("list", err)
	_, err = d.CreateAPIKey(ctx, "id", db.CreateAPIKeyInput{Name: "n"})
	mustErr("create", err)
	_, err = d.GetAPIKeyByHash(ctx, "h")
	mustErr("getByHash", err)
	_, err = d.SetAPIKeyEnabled(ctx, "id", true)
	mustErr("setEnabled", err)
	_, err = d.DeleteAPIKey(ctx, "id")
	mustErr("delete", err)
	mustErr("touch", d.TouchAPIKey(ctx, "id", now))
	_, _, err = d.AllowAPIRequest(ctx, "id", 10, now)
	mustErr("allow", err)
}
