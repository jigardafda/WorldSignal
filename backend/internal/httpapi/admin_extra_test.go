package httpapi_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/worldsignal/backend/internal/auth"
	"github.com/worldsignal/backend/internal/dbtest"
	"github.com/worldsignal/backend/internal/httpapi"
)

// TestAdminForbiddenForViewer covers the authorization branch of every admin
// resolver (a VIEWER must be rejected).
func TestAdminForbiddenForViewer(t *testing.T) {
	ht, d := authServer(t)
	dbtest.SeedUser(t, d, "v@x.io", auth.RoleViewer)
	tok := login(t, ht.URL, "v@x.io", "password123")

	ops := []string{
		`{"query":"{users{id}}"}`,
		`{"query":"query($id:ID!){user(id:$id){id}}","variables":{"id":"x"}}`,
		`{"query":"mutation($i:CreateUserInput!){createUser(input:$i){id}}","variables":{"i":{"email":"a@b.c","password":"password123"}}}`,
		`{"query":"mutation($id:ID!,$i:UpdateUserInput!){updateUser(id:$id,input:$i){id}}","variables":{"id":"x","i":{"name":"y"}}}`,
		`{"query":"mutation($id:ID!){deleteUser(id:$id)}","variables":{"id":"x"}}`,
		`{"query":"{teams{id}}"}`,
		`{"query":"query($id:ID!){team(id:$id){id}}","variables":{"id":"x"}}`,
		`{"query":"mutation{createTeam(name:\"X\"){id}}"}`,
		`{"query":"mutation($id:ID!){deleteTeam(id:$id)}","variables":{"id":"x"}}`,
		`{"query":"mutation($t:ID!,$u:ID!){addTeamMember(teamId:$t,userId:$u)}","variables":{"t":"x","u":"y"}}`,
		`{"query":"mutation($t:ID!,$u:ID!){removeTeamMember(teamId:$t,userId:$u)}","variables":{"t":"x","u":"y"}}`,
	}
	for _, op := range ops {
		if b := gql(t, ht.URL, tok, op); !strings.Contains(b, "forbidden") {
			t.Fatalf("viewer should be forbidden:\n  op: %s\n  got: %s", op, b)
		}
	}

	// Taxonomy resolver: viewer (has signals:read) is allowed; unauth is rejected.
	if b := gql(t, ht.URL, tok, `{"query":"{taxonomy}"}`); !strings.Contains(b, "POLITICS") {
		t.Fatalf("viewer taxonomy: %s", b)
	}
	if b := gql(t, ht.URL, "", `{"query":"{taxonomy}"}`); !strings.Contains(b, "unauthenticated") {
		t.Fatalf("unauth taxonomy: %s", b)
	}
}

// TestAdminDBErrorsViaHiddenTables exercises admin resolver DB-error branches by
// keeping auth intact but hiding the target table.
func TestAdminDBErrorsViaHiddenTables(t *testing.T) {
	ht, d := authServer(t)
	httpapi.SeedDefaultAdmin(context.Background(), d, "admin@x.io", "admin12345")
	tok := login(t, ht.URL, "admin@x.io", "admin12345")

	restore := func(tbl string) func() {
		ctx := context.Background()
		if _, err := d.Pool.Exec(ctx, `ALTER TABLE "`+tbl+`" RENAME TO "`+tbl+`__h"`); err != nil {
			t.Fatalf("hide %s: %v", tbl, err)
		}
		return func() { d.Pool.Exec(ctx, `ALTER TABLE "`+tbl+`__h" RENAME TO "`+tbl+`"`) }
	}

	// Team table hidden → team ops error (auth via User/Session still works).
	done := restore("Team")
	for _, op := range []string{
		`{"query":"{teams{id}}"}`,
		`{"query":"mutation{createTeam(name:\"X\"){id}}"}`,
		`{"query":"mutation($id:ID!){deleteTeam(id:$id)}","variables":{"id":"x"}}`,
	} {
		if b := gql(t, ht.URL, tok, op); !strings.Contains(b, `"errors"`) {
			t.Fatalf("hidden Team should error: op=%s got=%s", op, b)
		}
	}
	done()

	// TeamMember hidden → member ops error.
	done = restore("TeamMember")
	for _, op := range []string{
		`{"query":"mutation($t:ID!,$u:ID!){addTeamMember(teamId:$t,userId:$u)}","variables":{"t":"x","u":"y"}}`,
		`{"query":"mutation($t:ID!,$u:ID!){removeTeamMember(teamId:$t,userId:$u)}","variables":{"t":"x","u":"y"}}`,
	} {
		if b := gql(t, ht.URL, tok, op); !strings.Contains(b, `"errors"`) {
			t.Fatalf("hidden TeamMember should error: op=%s got=%s", op, b)
		}
	}
	done()
}

func TestLoginAndSeedClosedDB(t *testing.T) {
	d := dbtest.Connect(t)
	d.Close()
	srv := &httpapi.Server{DB: d, Enqueue: &recordEnqueuer{}, SigningSecret: "s"}
	ht := httptest.NewServer(srv.Handler())
	defer ht.Close()
	if b := gql(t, ht.URL, "", `{"query":"mutation{login(email:\"a@b.c\",password:\"x\"){token}}"}`); !strings.Contains(b, `"errors"`) {
		t.Fatalf("login on closed DB should error: %s", b)
	}
	if _, err := httpapi.SeedDefaultAdmin(context.Background(), d, "a@b.c", "pw"); err == nil {
		t.Fatal("SeedDefaultAdmin should error on closed DB")
	}
}
