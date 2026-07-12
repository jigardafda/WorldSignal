package httpapi_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/worldsignal/backend/internal/auth"
	"github.com/worldsignal/backend/internal/crypto"
	"github.com/worldsignal/backend/internal/cuid"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
)

// seedAccountKey inserts an enabled API key bound to an account, returning the raw key.
func seedAccountKey(t *testing.T, d *db.DB, accountID string, scopes []string) string {
	t.Helper()
	raw := "wsk_" + cuid.New()
	_, err := d.Pool.Exec(context.Background(),
		`INSERT INTO "ApiKey" ("id","accountId","name","keyHash","keyPrefix","scopes","rateLimitPerMin","enabled") VALUES ($1,$2,$3,$4,$5,$6,100000,true)`,
		cuid.New(), accountID, "k", crypto.SHA256Hex(raw), "wsk_x", scopes)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// rawReq issues an arbitrary method with an X-API-Key header and optional body.
func rawReq(t *testing.T, method, base, path, key, body string) (int, string) {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, _ := http.NewRequest(method, base+path, r)
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// TestMultiTenantSecurity is the adversarial isolation suite: it proves one
// tenant can never read or mutate another tenant's data (GraphQL + REST), that
// tenants cannot reach operator/admin surfaces, and that operators retain the
// cross-tenant view.
func TestMultiTenantSecurity(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ht, _ := newServer(t, d)
	apiKey = "" // don't auto-attach the default full key
	ctx := context.Background()

	ex := func(q string, a ...any) {
		if _, err := d.Pool.Exec(ctx, q, a...); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	ex(`INSERT INTO "Signal" ("id","title","summary","status","severity","confidence","country","sourceCount","firstSeenAt","lastSeenAt","updatedAt") VALUES ('sg','T','S','CONFIRMED','HIGH',0.8,'US',1,now(),now(),now())`)

	accA, _ := d.CreateAccount(ctx, cuid.New(), "Acme", "acct-a", "PRO")
	accB, _ := d.CreateAccount(ctx, cuid.New(), "Globex", "acct-b", "FREE")
	subA, _ := d.CreateSubscription(ctx, db.CreateSubscriptionInput{Name: "A-sub", AccountID: accA.ID})
	subB, _ := d.CreateSubscription(ctx, db.CreateSubscriptionInput{Name: "B-sub", AccountID: accB.ID})
	ex(`INSERT INTO "DeliveryEvent" ("id","subscriptionId","signalId","channel","status","payload","createdAt") VALUES ('dA',$1,'sg','WEBHOOK','SENT','{}',now())`, subA.ID)
	ex(`INSERT INTO "DeliveryEvent" ("id","subscriptionId","signalId","channel","status","payload","createdAt") VALUES ('dB',$1,'sg','WEBHOOK','SENT','{}',now())`, subB.ID)

	scopes := []string{"subscriptions:read", "subscriptions:write", "signals:read", "deliveries:read", "stats:read"}
	keyA := seedAccountKey(t, d, accA.ID, scopes)
	keyB := seedAccountKey(t, d, accB.ID, scopes)
	tokenA, _ := dbtest.AuthTokenTenant(t, d, auth.RoleAdmin, accA.ID)

	// ============ GraphQL: tenant A cannot see/touch tenant B ============
	bearer = tokenA
	// mySubscriptions returns ONLY A's subscription.
	if _, body := postGQL(t, ht.URL, `{"query":"{mySubscriptions{id name}}"}`); !strings.Contains(body, `"name":"A-sub"`) || strings.Contains(body, "B-sub") || strings.Contains(body, subB.ID) {
		t.Fatalf("mySubscriptions leaked another tenant: %s", body)
	}
	// Updating / deleting B's subscription is forbidden (cross-tenant IDOR blocked).
	if _, body := postGQL(t, ht.URL, `{"query":"mutation($id:ID!,$i:UpdateSubscriptionInput!){updateMySubscription(id:$id,input:$i){id}}","variables":{"id":"`+subB.ID+`","i":{"enabled":false}}}`); !strings.Contains(body, "forbidden") {
		t.Fatalf("cross-tenant update should be forbidden: %s", body)
	}
	if _, body := postGQL(t, ht.URL, `{"query":"mutation($id:ID!){deleteMySubscription(id:$id)}","variables":{"id":"`+subB.ID+`"}}`); !strings.Contains(body, "forbidden") {
		t.Fatalf("cross-tenant delete should be forbidden: %s", body)
	}
	// A can manage its OWN subscription.
	if _, body := postGQL(t, ht.URL, `{"query":"mutation($id:ID!,$i:UpdateSubscriptionInput!){updateMySubscription(id:$id,input:$i){enabled}}","variables":{"id":"`+subA.ID+`","i":{"enabled":false}}}`); !strings.Contains(body, `"enabled":false`) {
		t.Fatalf("owner update should succeed: %s", body)
	}
	if _, body := postGQL(t, ht.URL, `{"query":"mutation($i:CreateMySubscriptionInput!){createMySubscription(input:$i){id name}}","variables":{"i":{"name":"A-new"}}}`); !strings.Contains(body, `"name":"A-new"`) {
		t.Fatalf("owner create should succeed: %s", body)
	}

	// ============ GraphQL: tenant cannot reach operator/admin surfaces ============
	for _, q := range []string{
		`{"query":"{subscriptions{id}}"}`, // operator cross-tenant list
		`{"query":"mutation($i:CreateSubscriptionInput!){createSubscription(input:$i){id}}","variables":{"i":{"name":"x"}}}`, // operator create
		`{"query":"{users{id}}"}`,    // user admin
		`{"query":"{accounts{id}}"}`, // account admin
		`{"query":"{sources{id}}"}`,  // ingestion
		`{"query":"{apiKeys{id}}"}`,  // operator API keys (all accounts)
		`{"query":"mutation($i:CreateAccountInput!){createAccount(input:$i){id}}","variables":{"i":{"name":"x"}}}`,                                         // create tenant
		`{"query":"mutation($i:CreateUserInput!){createUser(input:$i){id}}","variables":{"i":{"email":"e@x.io","password":"password123","role":"ADMIN"}}}`, // create user
	} {
		if _, body := postGQL(t, ht.URL, q); !strings.Contains(body, "forbidden") {
			t.Fatalf("tenant must be forbidden from operator surface %s: %s", q, body)
		}
	}
	// A tenant cannot revoke ANOTHER account's API key (scoped delete returns false).
	var keyBID string
	if err := d.Pool.QueryRow(ctx, `SELECT "id" FROM "ApiKey" WHERE "keyHash"=$1`, crypto.SHA256Hex(keyB)).Scan(&keyBID); err != nil {
		t.Fatal(err)
	}
	if _, body := postGQL(t, ht.URL, `{"query":"mutation($id:ID!){revokeMyApiKey(id:$id)}","variables":{"id":"`+keyBID+`"}}`); !strings.Contains(body, `"revokeMyApiKey":false`) {
		t.Fatalf("cross-account key revoke should be false: %s", body)
	}
	bearer = ""

	// ============ REST: API key A is confined to account A ============
	// Subscriptions list is account-scoped.
	if code, body := rawReq(t, "GET", ht.URL, "/v1/subscriptions", keyA, ""); code != 200 || !strings.Contains(body, subA.ID) || strings.Contains(body, subB.ID) {
		t.Fatalf("key A /v1/subscriptions leaked B: %d %s", code, body)
	}
	// Feed on own subscription works; on another tenant's → 404 (no existence leak).
	if code, _ := rawReq(t, "GET", ht.URL, "/v1/subscriptions/"+subA.ID+"/feed", keyA, ""); code != 200 {
		t.Fatalf("key A feed on own sub want 200 got %d", code)
	}
	if code, body := rawReq(t, "GET", ht.URL, "/v1/subscriptions/"+subB.ID+"/feed", keyA, ""); code != 404 {
		t.Fatalf("key A feed on B's sub want 404 got %d %s", code, body)
	}
	// Interests + feedback on another tenant's subscription → 404.
	if code, _ := rawReq(t, "PATCH", ht.URL, "/v1/subscriptions/"+subB.ID+"/interests", keyA, `{"interests":{"tag:X":5}}`); code != 404 {
		t.Fatalf("key A interests on B's sub want 404 got %d", code)
	}
	if code, _ := rawReq(t, "POST", ht.URL, "/v1/feedback", keyA, `{"subscriptionId":"`+subB.ID+`","signalId":"sg","action":"UP"}`); code != 404 {
		t.Fatalf("key A feedback on B's sub want 404 got %d", code)
	}
	// Deliveries are account-scoped.
	if code, body := rawReq(t, "GET", ht.URL, "/v1/deliveries", keyA, ""); code != 200 || !strings.Contains(body, `"id":"dA"`) || strings.Contains(body, `"id":"dB"`) {
		t.Fatalf("key A /v1/deliveries leaked B: %d %s", code, body)
	}
	// /v1/account resolves to the key's own account, never another.
	if code, body := rawReq(t, "GET", ht.URL, "/v1/account", keyA, ""); code != 200 || !strings.Contains(body, accA.ID) {
		t.Fatalf("key A /v1/account: %d %s", code, body)
	}
	if code, body := rawReq(t, "GET", ht.URL, "/v1/account", keyB, ""); code != 200 || !strings.Contains(body, accB.ID) {
		t.Fatalf("key B /v1/account: %d %s", code, body)
	}

	// ============ Operator retains the cross-tenant view ============
	admin, _ := dbtest.AuthToken(t, d, auth.RoleAdmin)
	bearer = admin
	if _, body := postGQL(t, ht.URL, `{"query":"{subscriptions{id name}}"}`); !strings.Contains(body, "A-sub") || !strings.Contains(body, "B-sub") {
		t.Fatalf("operator should see all tenants' subscriptions: %s", body)
	}
	bearer = ""

	// ============ Unauthenticated is rejected everywhere ============
	if _, body := postGQL(t, ht.URL, `{"query":"{mySubscriptions{id}}"}`); !strings.Contains(body, "unauthenticated") {
		t.Fatalf("unauth mySubscriptions: %s", body)
	}
	if code, _ := rawReq(t, "GET", ht.URL, "/v1/subscriptions", "", ""); code != 401 {
		t.Fatalf("unauth REST want 401 got %d", code)
	}

	// ============ REST error paths surface (tables gone → 500) ============
	if _, err := d.Pool.Exec(ctx, `ALTER TABLE "Subscription" RENAME TO "Subscription__h"`); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		method, path, body string
	}{
		{"GET", "/v1/subscriptions", ""},
		{"POST", "/v1/subscriptions", `{"name":"x"}`},
		{"GET", "/v1/deliveries", ""},
		{"GET", "/v1/subscriptions/" + subA.ID + "/feed", ""},
		{"PATCH", "/v1/subscriptions/" + subA.ID + "/interests", `{"interests":{}}`},
		{"POST", "/v1/feedback", `{"subscriptionId":"` + subA.ID + `","signalId":"sg","action":"UP"}`},
	} {
		if code, _ := rawReq(t, tc.method, ht.URL, tc.path, keyA, tc.body); code != 500 {
			t.Fatalf("%s %s want 500 got %d", tc.method, tc.path, code)
		}
	}
	if _, err := d.Pool.Exec(ctx, `ALTER TABLE "Subscription__h" RENAME TO "Subscription"`); err != nil {
		t.Fatal(err)
	}
}
