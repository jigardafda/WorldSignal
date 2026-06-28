package auth

import (
	"context"
	"errors"
	"testing"
)

func TestHashAndCheckPassword(t *testing.T) {
	h, err := HashPassword("s3cret!")
	if err != nil {
		t.Fatal(err)
	}
	if h == "s3cret!" || h == "" {
		t.Fatal("hash should differ from plaintext")
	}
	if !CheckPassword(h, "s3cret!") {
		t.Fatal("correct password should verify")
	}
	if CheckPassword(h, "wrong") {
		t.Fatal("wrong password should not verify")
	}
}

func TestGenerateTokenUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		tok, err := GenerateToken()
		if err != nil || tok == "" {
			t.Fatalf("token err: %v", err)
		}
		if seen[tok] {
			t.Fatal("duplicate token")
		}
		seen[tok] = true
	}
}

func TestValidRole(t *testing.T) {
	for _, r := range []string{RoleAdmin, RoleEditor, RoleViewer} {
		if !ValidRole(r) {
			t.Fatalf("%s should be valid", r)
		}
	}
	if ValidRole("SUPERUSER") {
		t.Fatal("unknown role should be invalid")
	}
}

func TestPermissionsMatrix(t *testing.T) {
	if !Can(RoleAdmin, PermUsersManage) {
		t.Fatal("admin should manage users")
	}
	if Can(RoleEditor, PermUsersManage) {
		t.Fatal("editor should not manage users")
	}
	if !Can(RoleEditor, PermSourcesWrite) {
		t.Fatal("editor should write sources")
	}
	if Can(RoleViewer, PermSourcesWrite) {
		t.Fatal("viewer should not write sources")
	}
	if !Can(RoleViewer, PermSignalsRead) {
		t.Fatal("viewer should read signals")
	}
	// Admin has the full set; viewer only read perms.
	if len(Permissions(RoleAdmin)) != len(allPerms) {
		t.Fatalf("admin perms = %d, want %d", len(Permissions(RoleAdmin)), len(allPerms))
	}
	if len(Permissions(RoleViewer)) != len(readPerms) {
		t.Fatalf("viewer perms = %d, want %d", len(Permissions(RoleViewer)), len(readPerms))
	}
	// Sorted + stable.
	p := Permissions(RoleEditor)
	for i := 1; i < len(p); i++ {
		if p[i-1] > p[i] {
			t.Fatal("permissions should be sorted")
		}
	}
	if len(Permissions("NOPE")) != 0 {
		t.Fatal("unknown role → no perms")
	}
}

func TestContextHelpers(t *testing.T) {
	ctx := context.Background()
	if IdentityFrom(ctx) != nil {
		t.Fatal("empty context has no identity")
	}
	if _, err := Require(ctx); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("Require should fail without identity")
	}
	if err := RequirePermission(ctx, PermSignalsRead); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("RequirePermission should fail without identity")
	}

	ctx = WithIdentity(ctx, &Identity{UserID: "u", Email: "a@b.c", Role: RoleEditor})
	id, err := Require(ctx)
	if err != nil || id.Email != "a@b.c" {
		t.Fatalf("Require: %+v %v", id, err)
	}
	if err := RequirePermission(ctx, PermSourcesWrite); err != nil {
		t.Fatalf("editor should pass sources:write: %v", err)
	}
	if err := RequirePermission(ctx, PermUsersManage); !errors.Is(err, ErrForbidden) {
		t.Fatal("editor should be forbidden from users:manage")
	}
}
