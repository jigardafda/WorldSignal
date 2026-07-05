package pipeline

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
)

func f64(v float64) *float64 { return &v }
func sptr(s string) *string  { return &s }

func TestMatchesFilter(t *testing.T) {
	base := &db.SignalForMatch{
		Confidence: 0.7, Severity: "HIGH", Country: sptr("US"),
		Region: sptr("California"), Sentiment: sptr("NEGATIVE"), Influence: sptr("MEDIUM"),
		Relevance: f64(0.8), TagCodes: []string{"DISASTER.EARTHQUAKE"},
		Entities: []string{"FEMA", "Governor Smith"},
		Title:    "Major quake hits coast", Summary: "A strong earthquake struck.",
	}
	bare := &db.SignalForMatch{Severity: "LOW"} // no optional attributes populated
	cases := []struct {
		name string
		f    subscriptionFilter
		sig  *db.SignalForMatch
		want bool
	}{
		{"empty matches", subscriptionFilter{}, base, true},
		{"minConfidence pass", subscriptionFilter{MinConfidence: f64(0.5)}, base, true},
		{"minConfidence fail", subscriptionFilter{MinConfidence: f64(0.9)}, base, false},
		{"minSeverity pass", subscriptionFilter{MinSeverity: sptr("MEDIUM")}, base, true},
		{"minSeverity fail", subscriptionFilter{MinSeverity: sptr("CRITICAL")}, base, false},
		{"country pass", subscriptionFilter{Countries: []string{"US", "GB"}}, base, true},
		{"country fail", subscriptionFilter{Countries: []string{"IN"}}, base, false},
		{"country nil signal", subscriptionFilter{Countries: []string{"US"}}, bare, false},
		{"tag exact", subscriptionFilter{Tags: []string{"DISASTER.EARTHQUAKE"}}, base, true},
		{"tag prefix", subscriptionFilter{Tags: []string{"DISASTER"}}, base, true},
		{"tag miss", subscriptionFilter{Tags: []string{"ECONOMY"}}, base, false},
		{"minRelevance pass", subscriptionFilter{MinRelevance: f64(0.5)}, base, true},
		{"minRelevance fail", subscriptionFilter{MinRelevance: f64(0.9)}, base, false},
		{"minRelevance nil signal", subscriptionFilter{MinRelevance: f64(0.5)}, bare, false},
		{"minInfluence pass", subscriptionFilter{MinInfluence: sptr("MEDIUM")}, base, true},
		{"minInfluence fail", subscriptionFilter{MinInfluence: sptr("HIGH")}, base, false},
		{"minInfluence nil signal", subscriptionFilter{MinInfluence: sptr("LOW")}, bare, false},
		{"regions pass fold", subscriptionFilter{Regions: []string{"california"}}, base, true},
		{"regions fail", subscriptionFilter{Regions: []string{"Texas"}}, base, false},
		{"regions nil signal", subscriptionFilter{Regions: []string{"California"}}, bare, false},
		{"sentiment pass", subscriptionFilter{Sentiment: []string{"NEGATIVE"}}, base, true},
		{"sentiment fail", subscriptionFilter{Sentiment: []string{"POSITIVE"}}, base, false},
		{"sentiment nil signal", subscriptionFilter{Sentiment: []string{"NEGATIVE"}}, bare, false},
		{"entities pass fold", subscriptionFilter{Entities: []string{"fema"}}, base, true},
		{"entities fail", subscriptionFilter{Entities: []string{"Acme"}}, base, false},
		{"entities none on signal", subscriptionFilter{Entities: []string{"FEMA"}}, bare, false},
		{"keyword in title", subscriptionFilter{Keyword: "quake"}, base, true},
		{"keyword in summary", subscriptionFilter{Keyword: "earthquake"}, base, true},
		{"keyword miss", subscriptionFilter{Keyword: "flood"}, base, false},
		{"combined pass", subscriptionFilter{MinSeverity: sptr("HIGH"), Countries: []string{"US"}, Sentiment: []string{"NEGATIVE"}, MinRelevance: f64(0.5)}, base, true},
		{"combined one fails", subscriptionFilter{MinSeverity: sptr("HIGH"), Countries: []string{"US"}, Sentiment: []string{"POSITIVE"}}, base, false},
	}
	for _, c := range cases {
		if got := matchesFilter(c.f, c.sig); got != c.want {
			t.Fatalf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestSignPayload(t *testing.T) {
	body := []byte(`{"a":1}`)
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write(body)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if got := SignPayload("secret", body); got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestItoaPipeline(t *testing.T) {
	if itoa(0) != "0" || itoa(42) != "42" {
		t.Fatal("itoa")
	}
}

func TestMatchSubscriptionsCreatesDeliveries(t *testing.T) {
	d := conn(t)
	ctx := context.Background()
	dbtest.SeedTaxonomy(t, d)
	ex := func(q string, a ...any) { mustExec(t, d, q, a...) }
	ex(`INSERT INTO "Signal" ("id","title","summary","severity","confidence","country","sourceCount","firstSeenAt","lastSeenAt","updatedAt") VALUES ('sg','T','S','HIGH',0.8,'US',1,now(),now(),now())`)
	ex(`INSERT INTO "SignalTag" ("signalId","tagId","confidence") SELECT 'sg',"id",0.9 FROM "TaxonomyTag" WHERE "code"='DISASTER.EARTHQUAKE'`)
	ex(`INSERT INTO "Subscriber" ("id","name","createdAt") VALUES ('__default__','D',now())`)
	ex(`INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config","createdAt") VALUES ('m','__default__','match','POLLING','{"tags":["DISASTER"]}','{}',now())`)
	ex(`INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config","createdAt") VALUES ('nm','__default__','nomatch','WEBHOOK','{"tags":["ECONOMY"]}','{}',now())`)

	ids, err := MatchSubscriptions(ctx, d, "sg", time.Now(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 {
		t.Fatalf("expected 1 matched delivery, got %d", len(ids))
	}
}

type captureHook struct {
	mu   sync.Mutex
	code int
	sig  string
	body string
	hit  bool
}

func (h *captureHook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	h.mu.Lock()
	h.hit = true
	h.sig = r.Header.Get("X-WorldSignal-Signature")
	h.body = string(b)
	code := h.code
	h.mu.Unlock()
	if code == 0 {
		code = 200
	}
	w.WriteHeader(code)
}

func TestSendDeliveryPaths(t *testing.T) {
	d := conn(t)
	ctx := context.Background()
	now := time.Now()
	hook := &captureHook{code: 200}
	srv := httptest.NewServer(hook)
	defer srv.Close()

	seedDel := func(channel, config string) {
		dbtest.Reset(t, d)
		ex := func(q string, a ...any) { mustExec(t, d, q, a...) }
		ex(`INSERT INTO "Subscriber" ("id","name","createdAt") VALUES ('__default__','D',now())`)
		ex(`INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config","createdAt") VALUES ('sub','__default__','S',$1::"DeliveryChannel",'{}',$2,now())`, channel, config)
		ex(`INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","updatedAt") VALUES ('sg','T','S',now(),now(),now())`)
		ex(`INSERT INTO "DeliveryEvent" ("id","subscriptionId","signalId","channel","status","payload","attempts","createdAt") VALUES ('del','sub','sg',$1::"DeliveryChannel",'PENDING','{"event_id":"e","x":1}',0,now())`, channel)
	}
	status := func() (string, *string) {
		var s string
		var m *string
		mustScan(t, d, `SELECT status,"errorMessage" FROM "DeliveryEvent" WHERE id='del'`, &s, &m)
		return s, m
	}

	// POLLING → SENT, no HTTP.
	seedDel("POLLING", "{}")
	if err := SendDelivery(ctx, d, srv.Client(), "secret", "del", false, now); err != nil {
		t.Fatal(err)
	}
	if s, _ := status(); s != "SENT" {
		t.Fatalf("polling status %s", s)
	}

	// WEBHOOK success → SENT, with a valid signature header.
	hook.code = 200
	seedDel("WEBHOOK", `{"url":"`+srv.URL+`"}`)
	if err := SendDelivery(ctx, d, srv.Client(), "secret", "del", false, now); err != nil {
		t.Fatal(err)
	}
	if s, _ := status(); s != "SENT" {
		t.Fatalf("webhook status %s", s)
	}
	if !hook.hit || hook.sig != SignPayload("secret", []byte(hook.body)) {
		t.Fatalf("signature mismatch: %s", hook.sig)
	}

	// WEBHOOK no url → FAILED.
	seedDel("WEBHOOK", `{}`)
	if err := SendDelivery(ctx, d, srv.Client(), "secret", "del", false, now); err != nil {
		t.Fatal(err)
	}
	if s, m := status(); s != "FAILED" || m == nil {
		t.Fatalf("no-url status %s msg %v", s, m)
	}

	// WEBHOOK 500 non-final → RETRYING (and SendDelivery returns the error).
	hook.code = 500
	seedDel("WEBHOOK", `{"url":"`+srv.URL+`"}`)
	if err := SendDelivery(ctx, d, srv.Client(), "secret", "del", false, now); err == nil {
		t.Fatal("expected error on non-final failure")
	}
	if s, _ := status(); s != "RETRYING" {
		t.Fatalf("retry status %s", s)
	}

	// WEBHOOK 500 final → DEAD_LETTERED (no error returned).
	seedDel("WEBHOOK", `{"url":"`+srv.URL+`"}`)
	if err := SendDelivery(ctx, d, srv.Client(), "secret", "del", true, now); err != nil {
		t.Fatalf("final failure should not return error: %v", err)
	}
	if s, _ := status(); s != "DEAD_LETTERED" {
		t.Fatalf("dead-letter status %s", s)
	}
}
