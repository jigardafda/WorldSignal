package httpapi_test

import (
	"context"
	"strings"
	"testing"

	"github.com/worldsignal/backend/internal/auth"
	"github.com/worldsignal/backend/internal/dbtest"
)

// TestRelevanceResolversGraphQL exercises the admin-panel GraphQL surface for the
// smart-signals feed: ranked feed, interests get/set, feedback, and AI draft.
func TestRelevanceResolversGraphQL(t *testing.T) {
	d := dbtest.Connect(t)
	seed(t, d)
	ht, _ := newServer(t, d)
	admin, _ := dbtest.AuthToken(t, d, auth.RoleAdmin)
	bearer = admin
	defer func() { bearer = "" }()

	ctx := context.Background()
	ex := func(sql string, args ...any) {
		if _, err := d.Pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("exec: %v", err)
		}
	}
	ex(`INSERT INTO "Subscriber" ("id","name","status","createdAt") VALUES ('sg2','Acme','ACTIVE',now()) ON CONFLICT DO NOTHING`)
	ex(`INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config","enabled","createdAt") VALUES ('pg','sg2','For You','WEBHOOK','{}','{}',true,now())`)
	ex(`INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","eventType","severity","influence","relevance","confidence","sourceCount","metadata","updatedAt") VALUES ('qg','Quake hits','A quake struck.',now(),now(),'DISASTER.EARTHQUAKE','HIGH','HIGH',0.8,0.9,1,'{}',now())`)
	ex(`INSERT INTO "SignalAttribute" ("signalId","key","valueCode","valueText","confidence") VALUES ('qg','category','DISASTER.EARTHQUAKE','',0.9)`)

	// setSubscriptionInterests
	if code, body := postGQL(t, ht.URL, `{"query":"mutation($id:ID!,$i:JSON){setSubscriptionInterests(id:$id,interests:$i){ok}}","variables":{"id":"pg","i":{"tag:DISASTER":5}}}`); code != 200 || !strings.Contains(body, `"ok":true`) {
		t.Fatalf("setSubscriptionInterests: %d %s", code, body)
	}
	// subscriptionInterests (leaf JSON)
	if code, body := postGQL(t, ht.URL, `{"query":"query($id:ID!){subscriptionInterests(id:$id)}","variables":{"id":"pg"}}`); code != 200 || !strings.Contains(body, "tag:DISASTER") {
		t.Fatalf("subscriptionInterests: %d %s", code, body)
	}
	// subscriptionFeed — the DISASTER signal ranks and carries score + reasons.
	if code, body := postGQL(t, ht.URL, `{"query":"query($id:ID!){subscriptionFeed(id:$id,limit:5,minScore:0){id score reasons eventType}}","variables":{"id":"pg"}}`); code != 200 || !strings.Contains(body, `"qg"`) || !strings.Contains(body, `"score"`) {
		t.Fatalf("subscriptionFeed: %d %s", code, body)
	}
	// A very high minScore filters everything out (the score<minScore continue).
	if code, body := postGQL(t, ht.URL, `{"query":"query{subscriptionFeed(id:\"pg\",limit:5,minScore:1000){id}}"}`); code != 200 || strings.Contains(body, `"qg"`) {
		t.Fatalf("high minScore should filter all: %d %s", code, body)
	}
	// limit:1 exercises the result-cap break.
	if code, _ := postGQL(t, ht.URL, `{"query":"query{subscriptionFeed(id:\"pg\",limit:1,minScore:0){id}}"}`); code != 200 {
		t.Fatalf("limit:1 feed should be 200")
	}
	// recordSignalFeedback (valid + invalid action)
	if code, body := postGQL(t, ht.URL, `{"query":"mutation{recordSignalFeedback(subscriptionId:\"pg\",signalId:\"qg\",action:\"UP\")}"}`); code != 200 || !strings.Contains(body, "true") {
		t.Fatalf("recordSignalFeedback: %d %s", code, body)
	}
	if _, body := postGQL(t, ht.URL, `{"query":"mutation{recordSignalFeedback(subscriptionId:\"pg\",signalId:\"qg\",action:\"NOPE\")}"}`); !strings.Contains(body, "false") {
		t.Fatalf("bad feedback action should return false: %s", body)
	}
	// draftProfileFromDocument (heuristic path — no LLM key in tests)
	if code, body := postGQL(t, ht.URL, `{"query":"mutation($t:String!){draftProfileFromDocument(text:$t){source interests}}","variables":{"t":"Nike media kit. Nike sponsors sprinter Marcus Vale in running and championship events across the United States."}}`); code != 200 || !strings.Contains(body, "interests") {
		t.Fatalf("draftProfileFromDocument: %d %s", code, body)
	}
	// too-short document is rejected
	if _, body := postGQL(t, ht.URL, `{"query":"mutation{draftProfileFromDocument(text:\"hi\"){source}}"}`); !strings.Contains(body, "errors") {
		t.Fatalf("short draft should error: %s", body)
	}
}

// TestRelevanceResolversForbidden confirms the resolvers enforce auth.
func TestRelevanceResolversForbidden(t *testing.T) {
	d := dbtest.Connect(t)
	seed(t, d)
	ht, _ := newServer(t, d)
	viewer, _ := dbtest.AuthToken(t, d, auth.RoleViewer)
	bearer = viewer
	defer func() { bearer = "" }()

	// A viewer lacks subscriptions:write → the mutation is rejected.
	if _, body := postGQL(t, ht.URL, `{"query":"mutation{recordSignalFeedback(subscriptionId:\"x\",signalId:\"y\",action:\"UP\")}"}`); !strings.Contains(body, "errors") {
		t.Fatalf("viewer should be forbidden from recording feedback: %s", body)
	}
}

// TestRelevanceResolversDBErrors covers the error-return branches of the feed
// resolvers when the backing queries fail.
func TestRelevanceResolversDBErrors(t *testing.T) {
	d := dbtest.Connect(t)
	seed(t, d)
	ht, _ := newServer(t, d)
	admin, _ := dbtest.AuthToken(t, d, auth.RoleAdmin)
	bearer = admin
	defer func() { bearer = "" }()
	ex := func(sql string) {
		if _, err := d.Pool.Exec(context.Background(), sql); err != nil {
			t.Fatalf("exec: %v", err)
		}
	}

	ex(`ALTER TABLE "Signal" RENAME TO "Signal__hg"`)
	if _, body := postGQL(t, ht.URL, `{"query":"query{subscriptionFeed(id:\"x\",limit:3){id}}"}`); !strings.Contains(body, "errors") {
		t.Fatalf("subscriptionFeed should error with Signal gone: %s", body)
	}
	ex(`ALTER TABLE "Signal__hg" RENAME TO "Signal"`)

	ex(`ALTER TABLE "Subscription" RENAME TO "Subscription__hg"`)
	if _, body := postGQL(t, ht.URL, `{"query":"query{subscriptionInterests(id:\"x\")}"}`); !strings.Contains(body, "errors") {
		t.Fatalf("subscriptionInterests should error with Subscription gone: %s", body)
	}
	if _, body := postGQL(t, ht.URL, `{"query":"mutation($i:JSON){setSubscriptionInterests(id:\"x\",interests:$i){ok}}","variables":{"i":{"tag:DISASTER":1}}}`); !strings.Contains(body, "errors") {
		t.Fatalf("setSubscriptionInterests should error with Subscription gone: %s", body)
	}
	ex(`ALTER TABLE "Subscription__hg" RENAME TO "Subscription"`)
}
