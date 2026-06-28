package httpapi

import (
	"context"
	"testing"

	"github.com/worldsignal/backend/internal/auth"
	"github.com/worldsignal/backend/internal/dbtest"
)

func TestAuditResolverAndRecording(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	s := &Server{DB: d, SigningSecret: "secret"}
	ctx := adminCtx(t, d, auth.RoleAdmin)

	// Record via both helpers.
	s.audit(ctx, "USER_CREATED", "user", "u1", map[string]any{"role": "EDITOR"})
	s.auditAnon(ctx, "LOGIN_FAILED", "bad@x.io", nil)

	res, err := s.resolveAuditLogs(ctx, map[string]any{"limit": 10})
	if err != nil {
		t.Fatal(err)
	}
	m := res.(map[string]any)
	if m["total"].(int) != 2 {
		t.Fatalf("expected 2 audit rows, got %v", m["total"])
	}
	items := m["items"].([]any)
	if items[0].(map[string]any)["action"] != "LOGIN_FAILED" {
		t.Fatalf("expected newest-first, got %+v", items[0])
	}

	// Filter by action.
	r2, _ := s.resolveAuditLogs(ctx, map[string]any{"action": "USER_CREATED"})
	if r2.(map[string]any)["total"].(int) != 1 {
		t.Fatalf("action filter: %+v", r2)
	}
	// Search.
	r3, _ := s.resolveAuditLogs(ctx, map[string]any{"search": "LOGIN"})
	if r3.(map[string]any)["total"].(int) != 1 {
		t.Fatalf("search filter: %+v", r3)
	}

	// Forbidden for non-admin.
	if _, err := s.resolveAuditLogs(adminCtx(t, d, auth.RoleViewer), nil); err == nil {
		t.Fatal("viewer should be forbidden from audit logs")
	}

	// DB error branch.
	if _, err := d.Pool.Exec(context.Background(), `ALTER TABLE "AuditLog" RENAME TO "AuditLog__h3"`); err != nil {
		t.Fatal(err)
	}
	defer d.Pool.Exec(context.Background(), `ALTER TABLE "AuditLog__h3" RENAME TO "AuditLog"`)
	if _, err := s.resolveAuditLogs(ctx, nil); err == nil {
		t.Fatal("resolveAuditLogs should surface DB error")
	}
}
