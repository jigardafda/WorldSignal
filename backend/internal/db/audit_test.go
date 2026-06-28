package db_test

import (
	"context"
	"testing"

	"github.com/worldsignal/backend/internal/cuid"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
)

func TestAuditLogStore(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	actor := "admin@x.io"

	rec := func(action, tt, tid string, meta map[string]any) {
		if err := d.RecordAudit(ctx, db.AuditEntry{ID: cuid.New(), ActorEmail: &actor, Action: action, TargetType: tt, TargetID: tid, Metadata: meta}); err != nil {
			t.Fatal(err)
		}
	}
	rec("USER_CREATED", "user", "u1", map[string]any{"role": "EDITOR"})
	rec("SOURCE_DELETED", "source", "s1", nil)
	rec("LLM_KEY_CREATED", "llmKey", "k1", map[string]any{"label": "Prod"})

	// All, newest first.
	all, total, err := d.ListAuditLogs(ctx, db.AuditFilter{Limit: 10})
	if err != nil || total != 3 || len(all) != 3 {
		t.Fatalf("list all: total=%d len=%d err=%v", total, len(all), err)
	}
	if all[0].Action != "LLM_KEY_CREATED" {
		t.Fatalf("expected newest-first, got %s", all[0].Action)
	}

	// Filter by action.
	if _, n, _ := d.ListAuditLogs(ctx, db.AuditFilter{Action: ptr("USER_CREATED")}); n != 1 {
		t.Fatalf("action filter expected 1, got %d", n)
	}
	// Filter by target type.
	if _, n, _ := d.ListAuditLogs(ctx, db.AuditFilter{TargetType: ptr("source")}); n != 1 {
		t.Fatalf("targetType filter expected 1, got %d", n)
	}
	// Filter by actor (email ILIKE).
	if _, n, _ := d.ListAuditLogs(ctx, db.AuditFilter{Actor: ptr("admin@x.io")}); n != 3 {
		t.Fatalf("actor filter expected 3, got %d", n)
	}
	// Search across action/actor/target.
	if _, n, _ := d.ListAuditLogs(ctx, db.AuditFilter{Search: ptr("SOURCE")}); n != 1 {
		t.Fatalf("search expected 1, got %d", n)
	}
	// Pagination.
	page, _, _ := d.ListAuditLogs(ctx, db.AuditFilter{Limit: 2, Offset: 0})
	if len(page) != 2 {
		t.Fatalf("page size: %d", len(page))
	}
}

func TestAuditLogDBErrors(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	if _, err := d.Pool.Exec(ctx, `ALTER TABLE "AuditLog" RENAME TO "AuditLog__h"`); err != nil {
		t.Fatal(err)
	}
	defer d.Pool.Exec(ctx, `ALTER TABLE "AuditLog__h" RENAME TO "AuditLog"`)
	if err := d.RecordAudit(ctx, db.AuditEntry{ID: "x", Action: "X", Metadata: map[string]any{"a": 1}}); err == nil {
		t.Fatal("RecordAudit should error with table hidden")
	}
	if _, _, err := d.ListAuditLogs(ctx, db.AuditFilter{}); err == nil {
		t.Fatal("ListAuditLogs should error with table hidden")
	}
}
