package httpapi_test

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/worldsignal/backend/internal/auth"
	"github.com/worldsignal/backend/internal/cuid"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
)

var accountIDRe = regexp.MustCompile(`"createAccount":\{[^}]*"id":"([^"]+)"`)
var rawKeyRe = regexp.MustCompile(`"key":"(wsk_[^"]+)"`)

func TestAccountAdminGraphQL(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ht, _ := newServer(t, d)
	admin, _ := dbtest.AuthToken(t, d, auth.RoleAdmin)
	viewer, _ := dbtest.AuthToken(t, d, auth.RoleViewer)

	bearer = admin
	defer func() { bearer = "" }()

	// Create with a derived slug.
	_, body := postGQL(t, ht.URL, `{"query":"mutation($i:CreateAccountInput!){createAccount(input:$i){id name slug status plan}}","variables":{"i":{"name":"Acme Corp"}}}`)
	if !strings.Contains(body, `"slug":"acme-corp"`) || !strings.Contains(body, `"plan":"FREE"`) || !strings.Contains(body, `"status":"ACTIVE"`) {
		t.Fatalf("create derived slug: %s", body)
	}
	m := accountIDRe.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no account id: %s", body)
	}
	acmeID := m[1]

	// Explicit slug + plan.
	if _, body := postGQL(t, ht.URL, `{"query":"mutation($i:CreateAccountInput!){createAccount(input:$i){slug plan}}","variables":{"i":{"name":"Big Co","slug":"Big Co!!","plan":"PRO"}}}`); !strings.Contains(body, `"slug":"big-co"`) || !strings.Contains(body, `"plan":"PRO"`) {
		t.Fatalf("explicit slug/plan: %s", body)
	}

	// Validation: missing name, unsluggable name, duplicate slug, bad plan.
	for _, c := range []struct{ q, want string }{
		{`{"query":"mutation($i:CreateAccountInput!){createAccount(input:$i){id}}","variables":{"i":{"name":""}}}`, "name is required"},
		{`{"query":"mutation($i:CreateAccountInput!){createAccount(input:$i){id}}","variables":{"i":{"name":"!!!"}}}`, "slug"},
		{`{"query":"mutation($i:CreateAccountInput!){createAccount(input:$i){id}}","variables":{"i":{"name":"Dup","slug":"acme-corp"}}}`, "already taken"},
		{`{"query":"mutation($i:CreateAccountInput!){createAccount(input:$i){id}}","variables":{"i":{"name":"Bad","plan":"GOLD"}}}`, "unknown plan"},
	} {
		if _, body := postGQL(t, ht.URL, c.q); !strings.Contains(body, c.want) {
			t.Fatalf("want %q in %s", c.want, body)
		}
	}

	// List includes the default account + the two we created.
	if _, body := postGQL(t, ht.URL, `{"query":"{accounts{id name slug plan}}"}`); !strings.Contains(body, `"slug":"acme-corp"`) || !strings.Contains(body, `"slug":"default"`) {
		t.Fatalf("list accounts: %s", body)
	}
	// Single fetch: found + missing (null).
	if _, body := postGQL(t, ht.URL, `{"query":"query($id:ID!){account(id:$id){name}}","variables":{"id":"`+acmeID+`"}}`); !strings.Contains(body, `"name":"Acme Corp"`) {
		t.Fatalf("get account: %s", body)
	}
	if _, body := postGQL(t, ht.URL, `{"query":"query($id:ID!){account(id:$id){name}}","variables":{"id":"missing"}}`); !strings.Contains(body, `"account":null`) {
		t.Fatalf("missing account should be null: %s", body)
	}

	// Update: status + plan.
	if _, body := postGQL(t, ht.URL, `{"query":"mutation($id:ID!,$i:UpdateAccountInput!){updateAccount(id:$id,input:$i){status plan name}}","variables":{"id":"`+acmeID+`","i":{"status":"SUSPENDED","plan":"ENTERPRISE","name":"Acme Inc"}}}`); !strings.Contains(body, `"status":"SUSPENDED"`) || !strings.Contains(body, `"name":"Acme Inc"`) {
		t.Fatalf("update: %s", body)
	}
	// Update validation: empty name, bad status, bad plan, missing account.
	for _, c := range []struct{ q, want string }{
		{`{"query":"mutation($id:ID!,$i:UpdateAccountInput!){updateAccount(id:$id,input:$i){id}}","variables":{"id":"` + acmeID + `","i":{"name":"  "}}}`, "name cannot be empty"},
		{`{"query":"mutation($id:ID!,$i:UpdateAccountInput!){updateAccount(id:$id,input:$i){id}}","variables":{"id":"` + acmeID + `","i":{"status":"NOPE"}}}`, "unknown status"},
		{`{"query":"mutation($id:ID!,$i:UpdateAccountInput!){updateAccount(id:$id,input:$i){id}}","variables":{"id":"` + acmeID + `","i":{"plan":"NOPE"}}}`, "unknown plan"},
		{`{"query":"mutation($id:ID!,$i:UpdateAccountInput!){updateAccount(id:$id,input:$i){id}}","variables":{"id":"ghost","i":{"status":"ACTIVE"}}}`, "not found"},
	} {
		if _, body := postGQL(t, ht.URL, c.q); !strings.Contains(body, c.want) {
			t.Fatalf("update want %q in %s", c.want, body)
		}
	}

	// DB errors surface when the table is gone (covers the query/create/get/update
	// error branches).
	if _, err := d.Pool.Exec(context.Background(), `ALTER TABLE "Account" RENAME TO "Account__h"`); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`{"query":"{accounts{id}}"}`,
		`{"query":"query($id:ID!){account(id:$id){id}}","variables":{"id":"x"}}`,
		`{"query":"mutation($i:CreateAccountInput!){createAccount(input:$i){id}}","variables":{"i":{"name":"x"}}}`,
		`{"query":"mutation($id:ID!,$i:UpdateAccountInput!){updateAccount(id:$id,input:$i){id}}","variables":{"id":"x","i":{"status":"ACTIVE"}}}`,
	} {
		if _, body := postGQL(t, ht.URL, q); !strings.Contains(body, `"errors"`) {
			t.Fatalf("expected db error for %s: %s", q, body)
		}
	}
	if _, err := d.Pool.Exec(context.Background(), `ALTER TABLE "Account__h" RENAME TO "Account"`); err != nil {
		t.Fatal(err)
	}

	// Unauthenticated.
	bearer = ""
	if _, body := postGQL(t, ht.URL, `{"query":"{accounts{id}}"}`); !strings.Contains(body, "unauthenticated") {
		t.Fatalf("expected unauthenticated: %s", body)
	}
	// Viewer forbidden on every account operation.
	bearer = viewer
	for _, q := range []string{
		`{"query":"{accounts{id}}"}`,
		`{"query":"query($id:ID!){account(id:$id){id}}","variables":{"id":"x"}}`,
		`{"query":"mutation($i:CreateAccountInput!){createAccount(input:$i){id}}","variables":{"i":{"name":"x"}}}`,
		`{"query":"mutation($id:ID!,$i:UpdateAccountInput!){updateAccount(id:$id,input:$i){id}}","variables":{"id":"x","i":{"status":"ACTIVE"}}}`,
	} {
		if _, body := postGQL(t, ht.URL, q); !strings.Contains(body, "forbidden") {
			t.Fatalf("viewer should be forbidden: %s", body)
		}
	}
}

func TestAccountForKeyREST(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ht, _ := newServer(t, d) // seeds a full-scope key on the default account
	admin, _ := dbtest.AuthToken(t, d, auth.RoleAdmin)

	// The default-account key resolves /v1/account to the default tenant.
	if st, body := get(t, ht.URL, "/v1/account"); st != 200 || !strings.Contains(body, `"id":"`+db.DefaultAccountID+`"`) || !strings.Contains(body, `"slug":"default"`) {
		t.Fatalf("default key /v1/account: %d %s", st, body)
	}

	// Create an account + an API key scoped to it, then confirm the key resolves
	// to that tenant.
	acme, err := d.CreateAccount(context.Background(), cuid.New(), "Acme", "acme", "PRO")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	bearer = admin
	_, body := postGQL(t, ht.URL, `{"query":"mutation($i:CreateApiKeyInput!){createApiKey(input:$i){key accountId}}","variables":{"i":{"name":"acme-key","scopes":["signals:read"],"accountId":"`+acme.ID+`"}}}`)
	if !strings.Contains(body, `"accountId":"`+acme.ID+`"`) {
		t.Fatalf("create acme key: %s", body)
	}
	km := rawKeyRe.FindStringSubmatch(body)
	if km == nil {
		t.Fatalf("no raw key: %s", body)
	}
	bearer = ""
	old := apiKey
	apiKey = km[1]
	if st, body := get(t, ht.URL, "/v1/account"); st != 200 || !strings.Contains(body, `"id":"`+acme.ID+`"`) || !strings.Contains(body, `"plan":"PRO"`) {
		t.Fatalf("acme key /v1/account: %d %s", st, body)
	}
	apiKey = old

	// createApiKey against an unknown account is rejected.
	bearer = admin
	if _, body := postGQL(t, ht.URL, `{"query":"mutation($i:CreateApiKeyInput!){createApiKey(input:$i){id}}","variables":{"i":{"name":"x","scopes":["signals:read"],"accountId":"ghost"}}}`); !strings.Contains(body, "unknown account") {
		t.Fatalf("unknown account should be rejected: %s", body)
	}
	bearer = ""
}
