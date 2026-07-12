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

var firstIDRe = regexp.MustCompile(`"id":"([^"]+)"`)

// TestTenantConsoleSeparation verifies that an account-scoped (tenant) user is
// confined to the customer surface: it can read shared signals + manage its own
// account/API keys, but cannot reach operator resolvers, and its self-service
// key management is scoped to its own account.
func TestTenantConsoleSeparation(t *testing.T) {
	d := dbtest.Connect(t)
	seed(t, d) // resets + seeds a signal 'sg'
	ht, _ := newServer(t, d)
	ctx := context.Background()

	acme, err := d.CreateAccount(ctx, cuid.New(), "Acme", "acme", "PRO")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	other, _ := d.CreateAccount(ctx, cuid.New(), "Other", "other", "FREE")

	tenant, _ := dbtest.AuthTokenTenant(t, d, auth.RoleAdmin, acme.ID)
	bearer = tenant
	defer func() { bearer = "" }()

	// me/permissions reflect the tenant capability set (signals:read, analytics:read)
	// — NOT the operator role matrix, even though the stored role is ADMIN. me also
	// carries the tenant's account for workspace context.
	if _, body := postGQL(t, ht.URL, `{"query":"{me{role accountId permissions account{name plan}}}"}`); !strings.Contains(body, `"permissions":["analytics:read","signals:read"]`) || !strings.Contains(body, `"accountId":"`+acme.ID+`"`) || !strings.Contains(body, `"account":{"name":"Acme"`) || !strings.Contains(body, `"plan":"PRO"`) {
		t.Fatalf("tenant me: %s", body)
	}

	// Tenant CAN read the shared corpus.
	if _, body := postGQL(t, ht.URL, `{"query":"{signals(limit:1){id}}"}`); !strings.Contains(body, `"id":"sg"`) {
		t.Fatalf("tenant should read signals: %s", body)
	}

	// Tenant CANNOT reach operator resolvers.
	for _, q := range []string{
		`{"query":"{sources{id}}"}`,
		`{"query":"{users{id}}"}`,
		`{"query":"{accounts{id}}"}`,
		`{"query":"mutation($i:CreateAccountInput!){createAccount(input:$i){id}}","variables":{"i":{"name":"x"}}}`,
	} {
		if _, body := postGQL(t, ht.URL, q); !strings.Contains(body, "forbidden") {
			t.Fatalf("tenant should be forbidden for %s: %s", q, body)
		}
	}

	// myAccount returns the tenant's own account.
	if _, body := postGQL(t, ht.URL, `{"query":"{myAccount{id slug plan}}"}`); !strings.Contains(body, `"id":"`+acme.ID+`"`) || !strings.Contains(body, `"plan":"PRO"`) {
		t.Fatalf("myAccount: %s", body)
	}
	// tenantApiScopes lists the read-only subset.
	if _, body := postGQL(t, ht.URL, `{"query":"{tenantApiScopes}"}`); !strings.Contains(body, "signals:read") || strings.Contains(body, "sources:write") {
		t.Fatalf("tenantApiScopes: %s", body)
	}

	// Self-service key: create returns the raw key once and is scoped to the account.
	_, createBody := postGQL(t, ht.URL, `{"query":"mutation($i:CreateMyApiKeyInput!){createMyApiKey(input:$i){id accountId key scopes}}","variables":{"i":{"name":"prod","scopes":["signals:read","stats:read"],"rateLimitPerMin":0}}}`)
	if !strings.Contains(createBody, `"key":"wsk_`) || !strings.Contains(createBody, `"accountId":"`+acme.ID+`"`) {
		t.Fatalf("createMyApiKey: %s", createBody)
	}
	idm := firstIDRe.FindStringSubmatch(createBody)
	if idm == nil {
		t.Fatalf("no key id: %s", createBody)
	}
	ownID := idm[1]

	// myApiKeys lists only this tenant's keys and never leaks the secret.
	if _, body := postGQL(t, ht.URL, `{"query":"{myApiKeys{id name accountId}}"}`); !strings.Contains(body, `"name":"prod"`) || strings.Contains(body, "keyHash") {
		t.Fatalf("myApiKeys: %s", body)
	}

	// createMyApiKey validation: no name, forbidden scope, empty scopes.
	for _, c := range []struct{ q, want string }{
		{`{"query":"mutation($i:CreateMyApiKeyInput!){createMyApiKey(input:$i){id}}","variables":{"i":{"scopes":["signals:read"]}}}`, "name is required"},
		{`{"query":"mutation($i:CreateMyApiKeyInput!){createMyApiKey(input:$i){id}}","variables":{"i":{"name":"x","scopes":["sources:write"]}}}`, "not available to tenants"},
		{`{"query":"mutation($i:CreateMyApiKeyInput!){createMyApiKey(input:$i){id}}","variables":{"i":{"name":"x","scopes":[]}}}`, "at least one scope"},
	} {
		if _, body := postGQL(t, ht.URL, c.q); !strings.Contains(body, c.want) {
			t.Fatalf("want %q in %s", c.want, body)
		}
	}

	// A key belonging to another account cannot be revoked by this tenant.
	otherKey, err := d.CreateAPIKey(ctx, cuid.New(), db.CreateAPIKeyInput{AccountID: other.ID, Name: "o", Hash: "oh", Prefix: "wsk_o", Scopes: []string{"signals:read"}, RateLimitPerMin: 10})
	if err != nil {
		t.Fatalf("seed other key: %v", err)
	}
	if _, body := postGQL(t, ht.URL, `{"query":"mutation($id:ID!){revokeMyApiKey(id:$id)}","variables":{"id":"`+otherKey.ID+`"}}`); !strings.Contains(body, `"revokeMyApiKey":false`) {
		t.Fatalf("cross-account revoke should be false: %s", body)
	}
	// Revoking own key succeeds.
	if _, body := postGQL(t, ht.URL, `{"query":"mutation($id:ID!){revokeMyApiKey(id:$id)}","variables":{"id":"`+ownID+`"}}`); !strings.Contains(body, `"revokeMyApiKey":true`) {
		t.Fatalf("own revoke should succeed: %s", body)
	}

	// Platform staff (no account) are NOT tenants: the tenant surface is forbidden.
	admin, _ := dbtest.AuthToken(t, d, auth.RoleAdmin)
	bearer = admin
	for _, q := range []string{
		`{"query":"{myAccount{id}}"}`,
		`{"query":"{myApiKeys{id}}"}`,
		`{"query":"mutation($i:CreateMyApiKeyInput!){createMyApiKey(input:$i){id}}","variables":{"i":{"name":"x","scopes":["signals:read"]}}}`,
	} {
		if _, body := postGQL(t, ht.URL, q); !strings.Contains(body, "forbidden") {
			t.Fatalf("staff should be forbidden from tenant surface %s: %s", q, body)
		}
	}
}

// TestTenantSubscriptionCRUD covers the customer-console subscription resolvers:
// create (with channel/filter/config + validation), update own, delete own, and
// the DB-error branches.
func TestTenantSubscriptionCRUD(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ht, _ := newServer(t, d)
	ctx := context.Background()
	acme, _ := d.CreateAccount(ctx, cuid.New(), "Acme", "acme", "PRO")
	tenant, _ := dbtest.AuthTokenTenant(t, d, auth.RoleViewer, acme.ID)

	bearer = tenant
	defer func() { bearer = "" }()

	// Create with channel/filter/config.
	_, body := postGQL(t, ht.URL, `{"query":"mutation($i:CreateMySubscriptionInput!){createMySubscription(input:$i){id name channel}}","variables":{"i":{"name":"Alerts","channel":"WEBHOOK","filter":{"country":"US"},"config":{"url":"https://x"}}}}`)
	if !strings.Contains(body, `"name":"Alerts"`) {
		t.Fatalf("create: %s", body)
	}
	subID := firstIDRe.FindStringSubmatch(body)[1]

	// Name is required.
	if _, body := postGQL(t, ht.URL, `{"query":"mutation($i:CreateMySubscriptionInput!){createMySubscription(input:$i){id}}","variables":{"i":{"name":"  "}}}`); !strings.Contains(body, "name is required") {
		t.Fatalf("create validation: %s", body)
	}
	// Update own (name + filter + config).
	if _, body := postGQL(t, ht.URL, `{"query":"mutation($id:ID!,$i:UpdateSubscriptionInput!){updateMySubscription(id:$id,input:$i){name enabled}}","variables":{"id":"`+subID+`","i":{"name":"Renamed","enabled":false,"filter":{"country":"IN"},"config":{"url":"https://y"}}}}`); !strings.Contains(body, `"name":"Renamed"`) {
		t.Fatalf("update own: %s", body)
	}
	// mySubscriptions lists it.
	if _, body := postGQL(t, ht.URL, `{"query":"{mySubscriptions{id name}}"}`); !strings.Contains(body, `"name":"Renamed"`) {
		t.Fatalf("list: %s", body)
	}
	// Delete own → true.
	if _, body := postGQL(t, ht.URL, `{"query":"mutation($id:ID!){deleteMySubscription(id:$id)}","variables":{"id":"`+subID+`"}}`); !strings.Contains(body, `"deleteMySubscription":true`) {
		t.Fatalf("delete own: %s", body)
	}
	// Update/delete a non-existent subscription → forbidden (no owner).
	if _, body := postGQL(t, ht.URL, `{"query":"mutation($id:ID!,$i:UpdateSubscriptionInput!){updateMySubscription(id:$id,input:$i){id}}","variables":{"id":"ghost","i":{"enabled":true}}}`); !strings.Contains(body, "forbidden") {
		t.Fatalf("update ghost: %s", body)
	}

	// DB errors surface.
	if _, err := d.Pool.Exec(ctx, `ALTER TABLE "Subscription" RENAME TO "Subscription__h"`); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`{"query":"{mySubscriptions{id}}"}`,
		`{"query":"mutation($i:CreateMySubscriptionInput!){createMySubscription(input:$i){id}}","variables":{"i":{"name":"x"}}}`,
		`{"query":"mutation($id:ID!,$i:UpdateSubscriptionInput!){updateMySubscription(id:$id,input:$i){id}}","variables":{"id":"x","i":{"enabled":true}}}`,
		`{"query":"mutation($id:ID!){deleteMySubscription(id:$id)}","variables":{"id":"x"}}`,
	} {
		if _, body := postGQL(t, ht.URL, q); !strings.Contains(body, `"errors"`) {
			t.Fatalf("want db error for %s: %s", q, body)
		}
	}
	if _, err := d.Pool.Exec(ctx, `ALTER TABLE "Subscription__h" RENAME TO "Subscription"`); err != nil {
		t.Fatal(err)
	}
}

// TestTenantErrors covers the unauthenticated and DB-error branches of the
// tenant self-service resolvers.
func TestTenantErrors(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ht, _ := newServer(t, d)
	ctx := context.Background()
	acme, _ := d.CreateAccount(ctx, cuid.New(), "Acme", "acme", "FREE")
	tenant, _ := dbtest.AuthTokenTenant(t, d, auth.RoleViewer, acme.ID)

	// Unauthenticated → every tenant resolver rejects.
	bearer = ""
	for _, q := range []string{
		`{"query":"{myAccount{id}}"}`,
		`{"query":"{myApiKeys{id}}"}`,
		`{"query":"{tenantApiScopes}"}`,
		`{"query":"{mySubscriptions{id}}"}`,
		`{"query":"mutation($i:CreateMyApiKeyInput!){createMyApiKey(input:$i){id}}","variables":{"i":{"name":"x","scopes":["signals:read"]}}}`,
		`{"query":"mutation($id:ID!){revokeMyApiKey(id:$id)}","variables":{"id":"x"}}`,
		`{"query":"mutation($i:CreateMySubscriptionInput!){createMySubscription(input:$i){id}}","variables":{"i":{"name":"x"}}}`,
		`{"query":"mutation($id:ID!,$i:UpdateSubscriptionInput!){updateMySubscription(id:$id,input:$i){id}}","variables":{"id":"x","i":{"enabled":true}}}`,
		`{"query":"mutation($id:ID!){deleteMySubscription(id:$id)}","variables":{"id":"x"}}`,
	} {
		if _, body := postGQL(t, ht.URL, q); !strings.Contains(body, "unauthenticated") {
			t.Fatalf("want unauthenticated for %s: %s", q, body)
		}
	}

	// DB errors surface when the underlying tables are gone.
	bearer = tenant
	defer func() { bearer = "" }()
	if _, err := d.Pool.Exec(ctx, `ALTER TABLE "ApiKey" RENAME TO "ApiKey__h"`); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`{"query":"{myApiKeys{id}}"}`,
		`{"query":"mutation($i:CreateMyApiKeyInput!){createMyApiKey(input:$i){id}}","variables":{"i":{"name":"x","scopes":["signals:read"]}}}`,
		`{"query":"mutation($id:ID!){revokeMyApiKey(id:$id)}","variables":{"id":"x"}}`,
	} {
		if _, body := postGQL(t, ht.URL, q); !strings.Contains(body, `"errors"`) {
			t.Fatalf("want db error for %s: %s", q, body)
		}
	}
	if _, err := d.Pool.Exec(ctx, `ALTER TABLE "ApiKey__h" RENAME TO "ApiKey"`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(ctx, `ALTER TABLE "Account" RENAME TO "Account__h"`); err != nil {
		t.Fatal(err)
	}
	if _, body := postGQL(t, ht.URL, `{"query":"{myAccount{id}}"}`); !strings.Contains(body, `"errors"`) {
		t.Fatalf("want db error for myAccount: %s", body)
	}
	if _, err := d.Pool.Exec(ctx, `ALTER TABLE "Account__h" RENAME TO "Account"`); err != nil {
		t.Fatal(err)
	}
}

// TestCreateUserWithAccount covers binding a new user to a tenant on creation.
func TestCreateUserWithAccount(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ht, _ := newServer(t, d)
	admin, _ := dbtest.AuthToken(t, d, auth.RoleAdmin)
	acme, _ := d.CreateAccount(context.Background(), cuid.New(), "Acme", "acme", "FREE")

	bearer = admin
	defer func() { bearer = "" }()

	// Create a tenant user bound to the account.
	if _, body := postGQL(t, ht.URL, `{"query":"mutation($i:CreateUserInput!){createUser(input:$i){id email accountId}}","variables":{"i":{"email":"t@acme.com","password":"password123","role":"VIEWER","accountId":"`+acme.ID+`"}}}`); !strings.Contains(body, `"accountId":"`+acme.ID+`"`) {
		t.Fatalf("create tenant user: %s", body)
	}
	// Unknown account rejected.
	if _, body := postGQL(t, ht.URL, `{"query":"mutation($i:CreateUserInput!){createUser(input:$i){id}}","variables":{"i":{"email":"g@x.com","password":"password123","role":"VIEWER","accountId":"ghost"}}}`); !strings.Contains(body, "unknown account") {
		t.Fatalf("unknown account should be rejected: %s", body)
	}
	// Staff user (no account) still works.
	if _, body := postGQL(t, ht.URL, `{"query":"mutation($i:CreateUserInput!){createUser(input:$i){email accountId}}","variables":{"i":{"email":"s@staff.com","password":"password123","role":"EDITOR"}}}`); !strings.Contains(body, `"accountId":null`) {
		t.Fatalf("staff user should have null accountId: %s", body)
	}
}
