package httpapi_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
	"github.com/worldsignal/backend/internal/httpapi"
)

type recordEnqueuer struct{ ids []string }

func (e *recordEnqueuer) EnqueueFetchSource(id string) error { e.ids = append(e.ids, id); return nil }

func newServer(t *testing.T, d *db.DB) (*httptest.Server, *recordEnqueuer) {
	t.Helper()
	enq := &recordEnqueuer{}
	srv := &httpapi.Server{DB: d, Enqueue: enq, SigningSecret: "s"}
	ht := httptest.NewServer(srv.Handler())
	t.Cleanup(ht.Close)
	return ht, enq
}

func seed(t *testing.T, d *db.DB) {
	t.Helper()
	dbtest.Reset(t, d)
	dbtest.SeedTaxonomy(t, d)
	ctx := context.Background()
	ex := func(q string, a ...any) {
		if _, err := d.Pool.Exec(ctx, q, a...); err != nil {
			t.Fatal(err)
		}
	}
	ex(`INSERT INTO "Source" ("id","name","url","lastSuccessAt","updatedAt") VALUES ('s1','S','https://s.example/feed',now(),now())`)
	ex(`INSERT INTO "Signal" ("id","title","summary","status","severity","confidence","country","sourceCount","firstSeenAt","lastSeenAt","updatedAt") VALUES ('sg','T','S','CONFIRMED','HIGH',0.8,'US',1,now(),now(),now())`)
	ex(`INSERT INTO "Article" ("id","sourceId","canonicalUrl","title","publishedAt") VALUES ('a1','s1','https://s.example/a','A',now())`)
	ex(`INSERT INTO "SignalArticle" ("signalId","articleId","relationType","similarityScore") VALUES ('sg','a1','PRIMARY',1)`)
	ex(`INSERT INTO "SignalTag" ("signalId","tagId","confidence") SELECT 'sg',"id",0.9 FROM "TaxonomyTag" WHERE "code"='DISASTER.EARTHQUAKE'`)
	ex(`INSERT INTO "Subscriber" ("id","name","createdAt") VALUES ('__default__','D',now())`)
	ex(`INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config","createdAt") VALUES ('sub','__default__','Sub','POLLING','{}','{}',now())`)
	ex(`INSERT INTO "DeliveryEvent" ("id","subscriptionId","signalId","channel","status","payload","createdAt") VALUES ('d1','sub','sg','POLLING','SENT','{"event_id":"e"}',now())`)
}

func get(t *testing.T, base, path string) (int, string) {
	t.Helper()
	resp, err := http.Get(base + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestHappyPaths(t *testing.T) {
	d := dbtest.Connect(t)
	seed(t, d)
	ht, _ := newServer(t, d)

	checks := []struct{ path, contains string }{
		{"/health", `"status":"ok"`},
		{"/v1/stats", `"sources":1`},
		{"/v1/taxonomy", `"POLITICS"`},
		{"/v1/sources", `"id":"s1"`},
		{"/v1/signals", `"id":"sg"`},
		{"/v1/signals?country=US&status=CONFIRMED&minConfidence=0.5&since=2020-01-01&search=T&tags=DISASTER.EARTHQUAKE&limit=5&offset=0", `"id":"sg"`},
		{"/v1/signals/sg", `"id":"sg"`},
		{"/v1/subscriptions", `"id":"sub"`},
		{"/v1/deliveries?limit=10", `"id":"d1"`},
	}
	for _, c := range checks {
		st, body := get(t, ht.URL, c.path)
		if st != 200 || !strings.Contains(body, c.contains) {
			t.Fatalf("GET %s -> %d %s (want %s)", c.path, st, body, c.contains)
		}
	}

	if st, _ := get(t, ht.URL, "/v1/signals/nope"); st != 404 {
		t.Fatalf("missing signal want 404 got %d", st)
	}
}

func TestGraphQLOverHTTP(t *testing.T) {
	d := dbtest.Connect(t)
	seed(t, d)
	ht, enq := newServer(t, d)

	// GET with variables.
	st, body := get(t, ht.URL, `/graphql?query=`+url(`query($f:SignalFilter){signals(filter:$f){id}}`)+`&variables=`+url(`{"f":{"minConfidence":0.1}}`))
	if st != 200 || !strings.Contains(body, `"id":"sg"`) {
		t.Fatalf("graphql GET: %d %s", st, body)
	}

	// GET with malformed variables JSON → variables ignored, still resolves.
	if st, b := get(t, ht.URL, `/graphql?query=`+url(`{stats}`)+`&variables=notjson`); st != 200 || !strings.Contains(b, `"sources"`) {
		t.Fatalf("graphql GET bad variables: %d %s", st, b)
	}

	// POST query.
	post := func(b string) (int, string) {
		resp, err := http.Post(ht.URL+"/graphql", "application/json", strings.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(body)
	}
	if _, b := post(`{"query":"{ stats }"}`); !strings.Contains(b, `"sources":1`) {
		t.Fatalf("graphql POST stats: %s", b)
	}
	if _, b := post(`{"query":"{ sources { id } subscriptions { id } taxonomy }"}`); !strings.Contains(b, `"id":"s1"`) {
		t.Fatalf("graphql POST multi: %s", b)
	}
	// Inline integer arguments exercise the int (non-float) coercion paths.
	if _, b := post(`{"query":"{ signals(filter:{minConfidence:1, country:\"US\"}, limit:5, offset:0){ id } }"}`); !strings.Contains(b, `"signals"`) {
		t.Fatalf("graphql inline ints: %s", b)
	}
	// All filter branches (status/search/tags) + limit/offset as JSON-number
	// variables (float64 coercion path).
	if _, b := post(`{"query":"query($f:SignalFilter,$l:Int,$o:Int){signals(filter:$f,limit:$l,offset:$o){id}}","variables":{"f":{"status":"CONFIRMED","search":"T","tags":["DISASTER.EARTHQUAKE"]},"l":10,"o":0}}`); !strings.Contains(b, `"id":"sg"`) {
		t.Fatalf("graphql full filter: %s", b)
	}
	if _, b := post(`{"query":"query($id:ID!){signal(id:$id){id}}","variables":{"id":"sg"}}`); !strings.Contains(b, `"id":"sg"`) {
		t.Fatalf("graphql signal: %s", b)
	}
	if _, b := post(`{"query":"query($id:ID!){signal(id:$id){id}}","variables":{"id":"x"}}`); !strings.Contains(b, `"signal":null`) {
		t.Fatalf("graphql missing signal: %s", b)
	}
	// Mutations.
	if _, b := post(`{"query":"mutation($i:CreateSourceInput!){createSource(input:$i){id name}}","variables":{"i":{"name":"N","url":"https://new.example/f","priority":2,"credibility":0.7,"crawlFrequency":600,"type":"ATOM","country":"US"}}}`); !strings.Contains(b, `"name":"N"`) {
		t.Fatalf("createSource: %s", b)
	}
	if _, b := post(`{"query":"mutation($id:ID!){setSourceEnabled(id:$id,enabled:false){id}}","variables":{"id":"s1"}}`); !strings.Contains(b, `"id":"s1"`) {
		t.Fatalf("setSourceEnabled: %s", b)
	}
	if _, b := post(`{"query":"mutation($id:ID!){triggerFetch(id:$id)}","variables":{"id":"s1"}}`); !strings.Contains(b, `"triggerFetch":true`) {
		t.Fatalf("triggerFetch: %s", b)
	}
	if len(enq.ids) == 0 {
		t.Fatal("triggerFetch should enqueue")
	}
	if _, b := post(`{"query":"mutation($i:CreateSubscriptionInput!){createSubscription(input:$i){id name}}","variables":{"i":{"name":"GS","channel":"WEBHOOK","filter":{"tags":["X"]},"config":{"url":"u"}}}}`); !strings.Contains(b, `"name":"GS"`) {
		t.Fatalf("createSubscription: %s", b)
	}
	// Bad body.
	if _, b := post(`not json`); !strings.Contains(b, `"errors"`) {
		t.Fatalf("bad body should error: %s", b)
	}
}

func TestRESTMutationsAndEdges(t *testing.T) {
	d := dbtest.Connect(t)
	seed(t, d)
	ht, enq := newServer(t, d)

	post := func(path, body string) (int, string) {
		resp, err := http.Post(ht.URL+path, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}

	if st, _ := post("/v1/sources", `{"name":"X","url":"https://x.example/f"}`); st != 201 {
		t.Fatalf("create source want 201 got %d", st)
	}
	if len(enq.ids) == 0 {
		t.Fatal("REST create should enqueue fetch")
	}
	if st, _ := post("/v1/sources", `{"name":"X"}`); st != 400 {
		t.Fatalf("missing url want 400 got %d", st)
	}
	if st, _ := post("/v1/sources", `{"name":"Dup","url":"https://s.example/feed"}`); st != 409 {
		t.Fatalf("dup want 409 got %d", st)
	}
	if st, _ := post("/v1/subscriptions", `{"name":"S2","channel":"WEBHOOK","filter":{"a":1},"config":{"b":2}}`); st != 201 {
		t.Fatalf("create sub want 201 got %d", st)
	}
	if st, _ := post("/v1/subscriptions", `{}`); st != 400 {
		t.Fatalf("missing name want 400 got %d", st)
	}
	// Malformed JSON body → 400 (readJSON error path).
	if st, _ := post("/v1/sources", `{bad json`); st != 400 {
		t.Fatalf("bad json want 400 got %d", st)
	}
	// Unparseable `since` is ignored (filter skipped), still 200.
	if st, _ := get(t, ht.URL, "/v1/signals?since=not-a-date"); st != 200 {
		t.Fatalf("bad since want 200 got %d", st)
	}

	// PATCH + fetch action.
	req, _ := http.NewRequest("PATCH", ht.URL+"/v1/sources/s1", strings.NewReader(`{"enabled":false,"priority":3,"crawlFrequency":120}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("patch want 200 got %d", resp.StatusCode)
	}
	if st, b := post("/v1/sources/s1/fetch", ``); st != 200 || !strings.Contains(b, `"queued":true`) {
		t.Fatalf("fetch action: %d %s", st, b)
	}

	// OPTIONS preflight.
	oreq, _ := http.NewRequest("OPTIONS", ht.URL+"/v1/sources", nil)
	ores, _ := http.DefaultClient.Do(oreq)
	if ores.StatusCode != 204 {
		t.Fatalf("OPTIONS want 204 got %d", ores.StatusCode)
	}
	ores.Body.Close()
}

type failEnqueuer struct{}

func (failEnqueuer) EnqueueFetchSource(string) error { return context.DeadlineExceeded }

func TestTriggerFetchEnqueueError(t *testing.T) {
	d := dbtest.Connect(t)
	seed(t, d)
	srv := &httpapi.Server{DB: d, Enqueue: failEnqueuer{}, SigningSecret: "s"}
	ht := httptest.NewServer(srv.Handler())
	defer ht.Close()
	resp, _ := http.Post(ht.URL+"/graphql", "application/json",
		strings.NewReader(`{"query":"mutation($id:ID!){triggerFetch(id:$id)}","variables":{"id":"s1"}}`))
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(b), `"errors"`) {
		t.Fatalf("enqueue error should surface: %s", b)
	}
}

func TestEmptyBodyCreateSource(t *testing.T) {
	d := dbtest.Connect(t)
	seed(t, d)
	ht, _ := newServer(t, d)
	resp, err := http.Post(ht.URL+"/v1/sources", "application/json", strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 { // readJSON returns nil; missing name → 400
		t.Fatalf("empty body want 400 got %d", resp.StatusCode)
	}
}

func TestErrorPathsWithClosedDB(t *testing.T) {
	d := dbtest.Connect(t)
	d.Close() // force every query to error
	enq := &recordEnqueuer{}
	srv := &httpapi.Server{DB: d, Enqueue: enq, SigningSecret: "s"}
	ht := httptest.NewServer(srv.Handler())
	defer ht.Close()

	// signal(id) resolver error branch.
	sr, _ := http.Post(ht.URL+"/graphql", "application/json", strings.NewReader(`{"query":"query($id:ID!){signal(id:$id){id}}","variables":{"id":"x"}}`))
	sb, _ := io.ReadAll(sr.Body)
	sr.Body.Close()
	if !strings.Contains(string(sb), `"errors"`) {
		t.Fatalf("closed DB signal(id) should error: %s", sb)
	}
	for _, p := range []string{"/v1/stats", "/v1/sources", "/v1/signals", "/v1/signals/x", "/v1/subscriptions", "/v1/deliveries"} {
		if st, _ := get(t, ht.URL, p); st != 500 {
			t.Fatalf("GET %s with closed DB want 500 got %d", p, st)
		}
	}
	// GraphQL resolver errors surface as an errors envelope.
	resp, _ := http.Post(ht.URL+"/graphql", "application/json", strings.NewReader(`{"query":"{ stats sources { id } signals { id } subscriptions { id } }"}`))
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(b), `"errors"`) {
		t.Fatalf("closed DB graphql should error: %s", b)
	}
	// GraphQL mutations also surface resolver errors on a closed DB.
	for _, m := range []string{
		`{"query":"mutation($i:CreateSourceInput!){createSource(input:$i){id}}","variables":{"i":{"name":"n","url":"u"}}}`,
		`{"query":"mutation($id:ID!){setSourceEnabled(id:$id,enabled:false){id}}","variables":{"id":"s1"}}`,
		`{"query":"mutation($i:CreateSubscriptionInput!){createSubscription(input:$i){id}}","variables":{"i":{"name":"n"}}}`,
	} {
		mr, _ := http.Post(ht.URL+"/graphql", "application/json", strings.NewReader(m))
		mb, _ := io.ReadAll(mr.Body)
		mr.Body.Close()
		if !strings.Contains(string(mb), `"errors"`) {
			t.Fatalf("closed DB mutation should error: %s", mb)
		}
	}
	// REST writes error.
	r1, _ := http.Post(ht.URL+"/v1/sources", "application/json", strings.NewReader(`{"name":"n","url":"u"}`))
	if r1.StatusCode != 500 {
		t.Fatalf("create with closed DB want 500 got %d", r1.StatusCode)
	}
	r1.Body.Close()
	r2, _ := http.Post(ht.URL+"/v1/subscriptions", "application/json", strings.NewReader(`{"name":"n"}`))
	if r2.StatusCode != 500 {
		t.Fatalf("create sub closed DB want 500 got %d", r2.StatusCode)
	}
	r2.Body.Close()
	// PATCH on closed DB → 500.
	preq, _ := http.NewRequest("PATCH", ht.URL+"/v1/sources/x", strings.NewReader(`{"enabled":true}`))
	preq.Header.Set("Content-Type", "application/json")
	pres, _ := http.DefaultClient.Do(preq)
	if pres.StatusCode != 500 {
		t.Fatalf("patch closed DB want 500 got %d", pres.StatusCode)
	}
	pres.Body.Close()
}

func url(s string) string {
	return strings.NewReplacer(" ", "%20", "{", "%7B", "}", "%7D", `"`, "%22", ":", "%3A", ",", "%2C", "(", "%28", ")", "%29", "$", "%24").Replace(s)
}
