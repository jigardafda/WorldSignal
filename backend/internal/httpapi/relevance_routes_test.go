package httpapi_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/worldsignal/backend/internal/dbtest"
)

// send issues an authenticated REST request (API key via authHeaders) and returns
// the status and body.
func send(t *testing.T, method, url, body string) (int, string) {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, _ := http.NewRequest(method, url, r)
	req.Header.Set("Content-Type", "application/json")
	authHeaders(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// TestRelevanceEndpoints covers the Phase-1 REST surface: set interests, ranked
// feed, feedback, and AI draft-from-document.
func TestRelevanceEndpoints(t *testing.T) {
	d := dbtest.Connect(t)
	seed(t, d)
	ht, _ := newServer(t, d)
	ctx := context.Background()
	ex := func(sql string, args ...any) {
		if _, err := d.Pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("exec: %v", err)
		}
	}

	// A profile (subscription) and two enriched signals.
	ex(`INSERT INTO "Subscription" ("id","name","channel","filter","config","enabled","createdAt") VALUES ('p9','For You','WEBHOOK','{}','{}',true,now())`)
	ex(`INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","eventType","severity","influence","relevance","confidence","sourceCount","metadata","updatedAt") VALUES ('q9','Quake hits coast','A quake struck.',now(),now(),'DISASTER.EARTHQUAKE','HIGH','HIGH',0.8,0.9,1,'{}',now())`)
	ex(`INSERT INTO "SignalAttribute" ("signalId","key","valueCode","valueText","confidence") VALUES ('q9','category','DISASTER.EARTHQUAKE','',0.9)`)
	ex(`INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","eventType","severity","influence","relevance","confidence","sourceCount","metadata","updatedAt") VALUES ('sp9','Cup final','Team wins.',now(),now(),'SPORTS.RESULT','LOW','LOW',0.3,0.4,1,'{}',now())`)
	ex(`INSERT INTO "SignalAttribute" ("signalId","key","valueCode","valueText","confidence") VALUES ('sp9','category','SPORTS.RESULT','',0.9)`)

	// 1. Set interests via PATCH.
	if code, body := send(t, "PATCH", ht.URL+"/v1/subscriptions/p9/interests", `{"interests":{"tag:DISASTER":5}}`); code != 200 {
		t.Fatalf("set interests want 200, got %d %s", code, body)
	}

	// 2. Ranked feed: the DISASTER signal must come first for a DISASTER profile.
	code, body := send(t, "GET", ht.URL+"/v1/subscriptions/p9/feed?limit=10", "")
	if code != 200 {
		t.Fatalf("feed want 200, got %d %s", code, body)
	}
	q := strings.Index(body, `"q9"`)
	sp := strings.Index(body, `"sp9"`)
	if q < 0 || (sp >= 0 && sp < q) {
		t.Fatalf("expected q9 ranked before sp9, body=%s", body)
	}
	if !strings.Contains(body, `"reasons"`) || !strings.Contains(body, `"score"`) {
		t.Fatalf("feed items should carry score + reasons, body=%s", body)
	}

	// 3. Feedback: valid action accepted, bad action rejected.
	if code, _ := send(t, "POST", ht.URL+"/v1/feedback", `{"subscriptionId":"p9","signalId":"q9","action":"UP"}`); code != 200 {
		t.Fatalf("feedback want 200, got %d", code)
	}
	if code, _ := send(t, "POST", ht.URL+"/v1/feedback", `{"subscriptionId":"p9","signalId":"q9","action":"BOGUS"}`); code != 400 {
		t.Fatalf("bad feedback action want 400, got %d", code)
	}

	// 4. AI draft-from-document (heuristic path — no LLM key in tests).
	doc := `{"text":"Nike media kit. Nike sponsors sprinter Marcus Vale. Focus on running, marathon and the championship season in the United States."}`
	code, body = send(t, "POST", ht.URL+"/v1/profiles/draft-from-document", doc)
	if code != 200 {
		t.Fatalf("draft want 200, got %d %s", code, body)
	}
	if !strings.Contains(body, `"interests"`) || !strings.Contains(body, `"source"`) {
		t.Fatalf("draft should return interests + source, body=%s", body)
	}
	// Too-short text is rejected.
	if code, _ := send(t, "POST", ht.URL+"/v1/profiles/draft-from-document", `{"text":"hi"}`); code != 400 {
		t.Fatalf("short draft want 400, got %d", code)
	}
}

// TestRelevanceEndpointEdges covers the parse/error branches of the REST feed API.
func TestRelevanceEndpointEdges(t *testing.T) {
	d := dbtest.Connect(t)
	seed(t, d)
	ht, _ := newServer(t, d)
	ex := func(sql string) {
		if _, err := d.Pool.Exec(context.Background(), sql); err != nil {
			t.Fatalf("exec: %v", err)
		}
	}
	ex(`INSERT INTO "Subscription" ("id","name","channel","filter","config","enabled","createdAt") VALUES ('pe','n','WEBHOOK','{}','{}',true,now())`)

	// Non-numeric query params fall back to defaults — still 200.
	if code, _ := send(t, "GET", ht.URL+"/v1/subscriptions/pe/feed?limit=abc&sinceHours=xyz&minScore=nope", ""); code != 200 {
		t.Fatalf("bad query params should default to 200, got %d", code)
	}
	// A valid numeric minScore is parsed (queryFloat success branch).
	if code, _ := send(t, "GET", ht.URL+"/v1/subscriptions/pe/feed?minScore=5.5", ""); code != 200 {
		t.Fatalf("valid minScore should be 200")
	}
	// Invalid JSON body → 400 on interests + feedback.
	if code, _ := send(t, "PATCH", ht.URL+"/v1/subscriptions/pe/interests", `{not json`); code != 400 {
		t.Fatalf("bad interests body want 400, got %d", code)
	}
	if code, _ := send(t, "POST", ht.URL+"/v1/feedback", `{not json`); code != 400 {
		t.Fatalf("bad feedback body want 400, got %d", code)
	}
	// Feedback for a non-existent signal violates the FK → 500.
	if code, _ := send(t, "POST", ht.URL+"/v1/feedback", `{"subscriptionId":"pe","signalId":"nope","action":"UP"}`); code != 500 {
		t.Fatalf("feedback FK error want 500, got %d", code)
	}
	// setInterests 500 when the Subscription table is gone.
	ex(`ALTER TABLE "Subscription" RENAME TO "Subscription__he"`)
	if code, _ := send(t, "PATCH", ht.URL+"/v1/subscriptions/pe/interests", `{"interests":{"tag:DISASTER":1}}`); code != 500 {
		t.Fatalf("setInterests DB error want 500, got %d", code)
	}
	ex(`ALTER TABLE "Subscription__he" RENAME TO "Subscription"`)
	// Feed 500 when the backing table is gone.
	ex(`ALTER TABLE "Signal" RENAME TO "Signal__he"`)
	if code, _ := send(t, "GET", ht.URL+"/v1/subscriptions/pe/feed", ""); code != 500 {
		t.Fatalf("feed DB error want 500, got %d", code)
	}
	ex(`ALTER TABLE "Signal__he" RENAME TO "Signal"`)
}
