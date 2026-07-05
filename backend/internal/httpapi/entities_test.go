package httpapi_test

import (
	"context"
	"strings"
	"testing"

	"github.com/worldsignal/backend/internal/auth"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
	"github.com/worldsignal/backend/internal/jobs"
)

func seedEntities(t *testing.T, d *db.DB) {
	t.Helper()
	dbtest.SeedTaxonomy(t, d)
	ctx := context.Background()
	ex := func(q string, a ...any) {
		if _, err := d.Pool.Exec(ctx, q, a...); err != nil {
			t.Fatalf("seed: %v\n%s", err, q)
		}
	}
	ex(`INSERT INTO "Source" ("id","name","url","country","priority","credibility","crawlFrequency","lastSuccessAt","updatedAt") VALUES ('src1','Src One','https://s1.example/feed','US',1,0.9,300,now(),now())`)
	ex(`INSERT INTO "RawItem" ("id","sourceId","sourceGuid","rawUrl","rawTitle","rawContent","contentHash","status","rawPayload") VALUES ('r1','src1','g1','https://s1.example/a','Quake','body text','h1','PARSED','{"k":"v"}')`)
	ex(`INSERT INTO "Article" ("id","rawItemId","sourceId","canonicalUrl","title","body","summary","contentHash","tokenSet","publishedAt") VALUES ('a1','r1','src1','https://s1.example/a','Quake hits','Full body','Summary','h1','quake hits',now())`)
	ex(`INSERT INTO "Signal" ("id","title","summary","status","severity","confidence","eventType","country","sourceCount","firstSeenAt","lastSeenAt","createdAt","updatedAt") VALUES ('sg1','Quake hits region','S','CONFIRMED','HIGH',0.8,'DISASTER.EARTHQUAKE','US',1,now(),now(),now(),now())`)
	ex(`INSERT INTO "SignalArticle" ("signalId","articleId","relationType","similarityScore") VALUES ('sg1','a1','PRIMARY',1)`)
	ex(`INSERT INTO "SignalTag" ("signalId","tagId","confidence") SELECT 'sg1',"id",0.9 FROM "TaxonomyTag" WHERE "code"='DISASTER.EARTHQUAKE'`)
	ex(`INSERT INTO "Subscriber" ("id","name","createdAt") VALUES ('__default__','Default',now())`)
	ex(`INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config","createdAt") VALUES ('sub1','__default__','All','POLLING','{}','{}',now())`)
	ex(`INSERT INTO "DeliveryEvent" ("id","subscriptionId","signalId","channel","status","payload","attempts","errorMessage","failedAt","createdAt") VALUES ('d1','sub1','sg1','POLLING','FAILED','{"event_id":"e"}',2,'boom',now(),now())`)

	// Jobs table + one row.
	q := jobs.New(d.Pool)
	if err := q.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	ex(`TRUNCATE TABLE ws_jobs`)
	ex(`INSERT INTO ws_jobs (id,queue,data,state,retry_count,retry_limit,last_error) VALUES ('j1','source.fetch','{"sourceId":"src1"}','failed',2,2,'oops')`)
}

func adminEntities(t *testing.T) (string, string, *recordEnqueuer) {
	t.Helper()
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	seedEntities(t, d)
	enq := &recordEnqueuer{}
	ht := newServerWith(t, d, enq)
	tok, _ := dbtest.AuthToken(t, d, auth.RoleAdmin)
	return ht, tok, enq
}

func TestEntityReadResolvers(t *testing.T) {
	base, tok, _ := adminEntities(t)

	checks := []struct{ name, query, contains string }{
		{"source", `{"query":"query($id:ID!){source(id:$id){id name country crawlFrequency parserType}}","variables":{"id":"src1"}}`, `"name":"Src One"`},
		{"signalCount", `{"query":"{signalCount(filter:{status:\"CONFIRMED\"})}"}`, `"signalCount":1`},
		{"signalCount_allfilters", `{"query":"{signalCount(filter:{status:\"CONFIRMED\",minConfidence:0.5,search:\"Quake\",tags:[\"DISASTER.EARTHQUAKE\"]})}"}`, `"signalCount":1`},
		{"subscription_missing", `{"query":"query($id:ID!){subscription(id:$id){id}}","variables":{"id":"nope"}}`, `"subscription":null`},
		{"articles", `{"query":"{articles(limit:10){items{id title sourceName signalCount} total}}"}`, `"total":1`},
		{"article", `{"query":"query($id:ID!){article(id:$id){id title body signals{id title relationType}}}","variables":{"id":"a1"}}`, `"relationType":"PRIMARY"`},
		{"rawItems", `{"query":"{rawItems(status:\"PARSED\"){items{id status sourceName} total}}"}`, `"status":"PARSED"`},
		{"rawItem", `{"query":"query($id:ID!){rawItem(id:$id){id rawContent rawPayload}}","variables":{"id":"r1"}}`, `body text`},
		{"deliveries", `{"query":"{deliveries(status:\"FAILED\"){items{id status signalTitle subscriptionName} total}}"}`, `"status":"FAILED"`},
		{"delivery", `{"query":"query($id:ID!){delivery(id:$id){id payload errorMessage}}","variables":{"id":"d1"}}`, `boom`},
		{"subscribers", `{"query":"{subscribers{id name subscriptionCount}}"}`, `"subscriptionCount":1`},
		{"subscription", `{"query":"query($id:ID!){subscription(id:$id){id name channel}}","variables":{"id":"sub1"}}`, `"name":"All"`},
		{"taxonomyStats", `{"query":"{taxonomyStats{code count}}"}`, `DISASTER.EARTHQUAKE`},
		{"jobs", `{"query":"{jobs{items{id queue state lastError} total}}"}`, `"state":"failed"`},
		{"jobCounts", `{"query":"{jobCounts{key count}}"}`, `"key":"failed"`},
		{"analytics", `{"query":"{analytics}"}`, `signalsBySeverity`},
	}
	for _, c := range checks {
		if b := gql(t, base, tok, c.query); !strings.Contains(b, c.contains) {
			t.Fatalf("%s: want %q in %s", c.name, c.contains, b)
		}
	}

	// Missing-entity → null.
	if b := gql(t, base, tok, `{"query":"query($id:ID!){source(id:$id){id}}","variables":{"id":"nope"}}`); !strings.Contains(b, `"source":null`) {
		t.Fatalf("missing source: %s", b)
	}
	if b := gql(t, base, tok, `{"query":"query($id:ID!){article(id:$id){id}}","variables":{"id":"nope"}}`); !strings.Contains(b, `"article":null`) {
		t.Fatalf("missing article: %s", b)
	}
	if b := gql(t, base, tok, `{"query":"query($id:ID!){delivery(id:$id){id}}","variables":{"id":"nope"}}`); !strings.Contains(b, `"delivery":null`) {
		t.Fatalf("missing delivery: %s", b)
	}
}

func TestEntityMutations(t *testing.T) {
	base, tok, enq := adminEntities(t)

	if b := gql(t, base, tok, `{"query":"mutation($id:ID!,$i:UpdateSourceInput!){updateSource(id:$id,input:$i){priority enabled credibility}}","variables":{"id":"src1","i":{"priority":3,"enabled":false,"credibility":0.7,"crawlFrequency":600,"country":"GB","name":"Renamed"}}}`); !strings.Contains(b, `"priority":3`) {
		t.Fatalf("updateSource: %s", b)
	}
	if b := gql(t, base, tok, `{"query":"mutation($n:String!){createSubscriber(name:$n){id name}}","variables":{"n":"New Sub"}}`); !strings.Contains(b, `"name":"New Sub"`) {
		t.Fatalf("createSubscriber: %s", b)
	}
	if b := gql(t, base, tok, `{"query":"mutation{createSubscriber(name:\"\"){id}}"}`); !strings.Contains(b, "validation") {
		t.Fatalf("empty subscriber name: %s", b)
	}
	if b := gql(t, base, tok, `{"query":"mutation($id:ID!,$i:UpdateSubscriptionInput!){updateSubscription(id:$id,input:$i){name enabled}}","variables":{"id":"sub1","i":{"name":"Renamed","enabled":false,"filter":{"tags":["X"]},"config":{"url":"u"}}}}`); !strings.Contains(b, `"name":"Renamed"`) {
		t.Fatalf("updateSubscription: %s", b)
	}
	// testSubscription pushes a test event built from the latest signal.
	if b := gql(t, base, tok, `{"query":"mutation($id:ID!){testSubscription(id:$id){ok channel message}}","variables":{"id":"sub1"}}`); !strings.Contains(b, `"ok":true`) || !strings.Contains(b, `"channel":"POLLING"`) {
		t.Fatalf("testSubscription: %s", b)
	}
	// testSubscription on a missing subscription errors.
	if b := gql(t, base, tok, `{"query":"mutation($id:ID!){testSubscription(id:$id){ok}}","variables":{"id":"nope"}}`); !strings.Contains(b, "not found") {
		t.Fatalf("testSubscription missing: %s", b)
	}
	// retryDelivery resets + enqueues.
	if b := gql(t, base, tok, `{"query":"mutation($id:ID!){retryDelivery(id:$id)}","variables":{"id":"d1"}}`); !strings.Contains(b, `"retryDelivery":true`) {
		t.Fatalf("retryDelivery: %s", b)
	}
	if len(enq.deliveryIDs) != 1 || enq.deliveryIDs[0] != "d1" {
		t.Fatalf("retryDelivery should enqueue d1, got %v", enq.deliveryIDs)
	}
	// retry missing delivery → false (no enqueue).
	if b := gql(t, base, tok, `{"query":"mutation($id:ID!){retryDelivery(id:$id)}","variables":{"id":"nope"}}`); !strings.Contains(b, `"retryDelivery":false`) {
		t.Fatalf("retry missing: %s", b)
	}
	// retryJob.
	if b := gql(t, base, tok, `{"query":"mutation($id:ID!){retryJob(id:$id)}","variables":{"id":"j1"}}`); !strings.Contains(b, `"retryJob":true`) {
		t.Fatalf("retryJob: %s", b)
	}
	// delete subscription + subscriber + source.
	if b := gql(t, base, tok, `{"query":"mutation($id:ID!){deleteSubscription(id:$id)}","variables":{"id":"sub1"}}`); !strings.Contains(b, `"deleteSubscription":true`) {
		t.Fatalf("deleteSubscription: %s", b)
	}
	if b := gql(t, base, tok, `{"query":"mutation($id:ID!){deleteSubscriber(id:$id)}","variables":{"id":"__default__"}}`); !strings.Contains(b, `"deleteSubscriber":true`) {
		t.Fatalf("deleteSubscriber: %s", b)
	}
	if b := gql(t, base, tok, `{"query":"mutation($id:ID!){deleteSource(id:$id)}","variables":{"id":"src1"}}`); !strings.Contains(b, `"deleteSource":true`) {
		t.Fatalf("deleteSource: %s", b)
	}
}

func TestTestSubscriptionBranches(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	ex := func(q string, a ...any) {
		if _, err := d.Pool.Exec(ctx, q, a...); err != nil {
			t.Fatalf("seed: %v\n%s", err, q)
		}
	}
	ex(`INSERT INTO "Subscriber" ("id","name","createdAt") VALUES ('__default__','Default',now())`)
	ex(`INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config","createdAt") VALUES ('hook','__default__','Hook','WEBHOOK','{}','{"url":"https://x/y"}',now())`)
	enq := &recordEnqueuer{}
	base := newServerWith(t, d, enq)
	tok, _ := dbtest.AuthToken(t, d, auth.RoleAdmin)

	// No signals yet → ok:false, nothing enqueued.
	if b := gql(t, base, tok, `{"query":"mutation($id:ID!){testSubscription(id:$id){ok message}}","variables":{"id":"hook"}}`); !strings.Contains(b, `"ok":false`) {
		t.Fatalf("no-signal testSubscription: %s", b)
	}
	if len(enq.deliveryIDs) != 0 {
		t.Fatalf("no delivery should be enqueued yet, got %v", enq.deliveryIDs)
	}

	// With a signal → ok:true and the webhook delivery is enqueued for sending.
	ex(`INSERT INTO "Signal" ("id","title","summary","status","severity","confidence","country","sourceCount","firstSeenAt","lastSeenAt","createdAt","updatedAt") VALUES ('sig','T','S','CONFIRMED','HIGH',0.9,'US',1,now(),now(),now(),now())`)
	if b := gql(t, base, tok, `{"query":"mutation($id:ID!){testSubscription(id:$id){ok channel}}","variables":{"id":"hook"}}`); !strings.Contains(b, `"ok":true`) {
		t.Fatalf("testSubscription: %s", b)
	}
	if len(enq.deliveryIDs) != 1 {
		t.Fatalf("webhook test should enqueue one delivery, got %v", enq.deliveryIDs)
	}
}

func TestEntityAuthz(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	seedEntities(t, d)
	ht := newServerWith(t, d, &recordEnqueuer{})
	viewerTok, _ := dbtest.AuthToken(t, d, auth.RoleViewer)
	editorTok, _ := dbtest.AuthToken(t, d, auth.RoleEditor)

	// Viewer can read content + analytics.
	if b := gql(t, ht, viewerTok, `{"query":"{analytics}"}`); !strings.Contains(b, "signalsBySeverity") {
		t.Fatalf("viewer analytics: %s", b)
	}
	// Viewer cannot perform any write mutation.
	for _, op := range []string{
		`{"query":"mutation($id:ID!){deleteSource(id:$id)}","variables":{"id":"src1"}}`,
		`{"query":"mutation($id:ID!,$i:UpdateSourceInput!){updateSource(id:$id,input:$i){id}}","variables":{"id":"src1","i":{"name":"x"}}}`,
		`{"query":"mutation($id:ID!){retryDelivery(id:$id)}","variables":{"id":"d1"}}`,
		`{"query":"mutation($id:ID!){retryJob(id:$id)}","variables":{"id":"j1"}}`,
		`{"query":"mutation($n:String!){createSubscriber(name:$n){id}}","variables":{"n":"x"}}`,
		`{"query":"mutation($id:ID!){deleteSubscriber(id:$id)}","variables":{"id":"__default__"}}`,
		`{"query":"mutation($id:ID!){deleteSubscription(id:$id)}","variables":{"id":"sub1"}}`,
		`{"query":"mutation($id:ID!,$i:UpdateSubscriptionInput!){updateSubscription(id:$id,input:$i){id}}","variables":{"id":"sub1","i":{"name":"x"}}}`,
		`{"query":"mutation($id:ID!){testSubscription(id:$id){ok}}","variables":{"id":"sub1"}}`,
	} {
		if b := gql(t, ht, viewerTok, op); !strings.Contains(b, "forbidden") {
			t.Fatalf("viewer should be forbidden: op=%s got=%s", op, b)
		}
	}
	// Editor can retry a delivery + a job (deliveries:retry, jobs:manage).
	if b := gql(t, ht, editorTok, `{"query":"mutation($id:ID!){retryJob(id:$id)}","variables":{"id":"j1"}}`); !strings.Contains(b, `"retryJob":true`) {
		t.Fatalf("editor retryJob: %s", b)
	}
}

// TestEntityUnauthenticated covers the authz-fail branch of every read resolver.
func TestEntityUnauthenticated(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	seedEntities(t, d)
	ht := newServerWith(t, d, &recordEnqueuer{})
	for _, op := range []string{
		`{"query":"{stats}"}`, `{"query":"{analytics}"}`, `{"query":"{jobCounts{key count}}"}`,
		`{"query":"{jobs{total}}"}`, `{"query":"{signalCount}"}`, `{"query":"{taxonomyStats{code}}"}`,
		`{"query":"{articles{total}}"}`, `{"query":"{rawItems{total}}"}`, `{"query":"{deliveries{total}}"}`,
		`{"query":"{subscribers{id}}"}`, `{"query":"query($id:ID!){source(id:$id){id}}","variables":{"id":"src1"}}`,
		`{"query":"query($id:ID!){article(id:$id){id}}","variables":{"id":"a1"}}`,
		`{"query":"query($id:ID!){rawItem(id:$id){id}}","variables":{"id":"r1"}}`,
		`{"query":"query($id:ID!){delivery(id:$id){id}}","variables":{"id":"d1"}}`,
		`{"query":"query($id:ID!){subscription(id:$id){id}}","variables":{"id":"sub1"}}`,
	} {
		if b := gql(t, ht, "", op); !strings.Contains(b, "unauthenticated") {
			t.Fatalf("unauthenticated should be rejected: op=%s got=%s", op, b)
		}
	}
}

// TestEntityResolverDBErrors covers the DB-error branch of entity resolvers by
// keeping auth intact (User/Session) and hiding the target table.
func TestEntityResolverDBErrors(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	seedEntities(t, d)
	base := newServerWith(t, d, &recordEnqueuer{})
	tok, _ := dbtest.AuthToken(t, d, auth.RoleAdmin)

	hide := func(tbl string) func() {
		ctx := context.Background()
		if _, err := d.Pool.Exec(ctx, `ALTER TABLE "`+tbl+`" RENAME TO "`+tbl+`__h"`); err != nil {
			t.Fatalf("hide %s: %v", tbl, err)
		}
		return func() { d.Pool.Exec(ctx, `ALTER TABLE "`+tbl+`__h" RENAME TO "`+tbl+`"`) }
	}

	cases := []struct {
		table string
		ops   []string
	}{
		{"Article", []string{`{"query":"{articles{total}}"}`, `{"query":"query($id:ID!){article(id:$id){id}}","variables":{"id":"a1"}}`}},
		{"RawItem", []string{`{"query":"{rawItems{total}}"}`, `{"query":"query($id:ID!){rawItem(id:$id){id}}","variables":{"id":"r1"}}`, `{"query":"{analytics}"}`}},
		{"DeliveryEvent", []string{`{"query":"{deliveries{total}}"}`, `{"query":"query($id:ID!){delivery(id:$id){id}}","variables":{"id":"d1"}}`, `{"query":"mutation($id:ID!){retryDelivery(id:$id)}","variables":{"id":"d1"}}`, `{"query":"{analytics}"}`}},
		{"Source", []string{`{"query":"{sources{id}}"}`, `{"query":"query($id:ID!){source(id:$id){id}}","variables":{"id":"src1"}}`, `{"query":"mutation($id:ID!,$i:UpdateSourceInput!){updateSource(id:$id,input:$i){id}}","variables":{"id":"src1","i":{"name":"x"}}}`, `{"query":"mutation($id:ID!){deleteSource(id:$id)}","variables":{"id":"src1"}}`, `{"query":"{analytics}"}`}},
		{"Signal", []string{`{"query":"{signalCount}"}`, `{"query":"{analytics}"}`}},
		{"Subscription", []string{`{"query":"{subscriptions{id}}"}`, `{"query":"query($id:ID!){subscription(id:$id){id}}","variables":{"id":"sub1"}}`, `{"query":"mutation($id:ID!,$i:UpdateSubscriptionInput!){updateSubscription(id:$id,input:$i){id}}","variables":{"id":"sub1","i":{"name":"x"}}}`, `{"query":"mutation($id:ID!){deleteSubscription(id:$id)}","variables":{"id":"sub1"}}`, `{"query":"mutation($id:ID!){testSubscription(id:$id){ok}}","variables":{"id":"sub1"}}`}},
		{"Subscriber", []string{`{"query":"{subscribers{id}}"}`, `{"query":"mutation{createSubscriber(name:\"x\"){id}}"}`, `{"query":"mutation($id:ID!){deleteSubscriber(id:$id)}","variables":{"id":"__default__"}}`}},
		{"TaxonomyTag", []string{`{"query":"{taxonomyStats{code count}}"}`}},
		{"ws_jobs", []string{`{"query":"{jobs{total}}"}`, `{"query":"{jobCounts{key count}}"}`, `{"query":"mutation($id:ID!){retryJob(id:$id)}","variables":{"id":"j1"}}`}},
	}
	for _, c := range cases {
		done := hide(c.table)
		for _, op := range c.ops {
			if b := gql(t, base, tok, op); !strings.Contains(b, `"errors"`) {
				done()
				t.Fatalf("hidden %s should error: op=%s got=%s", c.table, op, b)
			}
		}
		done()
	}
}
