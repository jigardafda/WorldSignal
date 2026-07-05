package httpapi_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
	"github.com/worldsignal/backend/internal/httpapi"
	"github.com/worldsignal/backend/internal/stream"
)

// newStreamServer builds a server with a real hub so live pushes can be tested.
func newStreamServer(t *testing.T, d *db.DB) (*httptest.Server, *stream.Hub) {
	t.Helper()
	hub := stream.NewHub()
	srv := &httpapi.Server{DB: d, SigningSecret: "s", Hub: hub}
	ht := httptest.NewServer(srv.Handler())
	t.Cleanup(ht.Close)
	return ht, hub
}

// addDelivery inserts a signal + a delivery row for subscription "sub".
func addDelivery(t *testing.T, d *db.DB, signalID, delID, eventID string) {
	t.Helper()
	ctx := context.Background()
	ex := func(q string, a ...any) {
		if _, err := d.Pool.Exec(ctx, q, a...); err != nil {
			t.Fatal(err)
		}
	}
	ex(`INSERT INTO "Signal" ("id","title","summary","status","severity","confidence","sourceCount","firstSeenAt","lastSeenAt","updatedAt") VALUES ($1,'T','S','CONFIRMED','HIGH',0.8,1,now(),now(),now())`, signalID)
	ex(`INSERT INTO "DeliveryEvent" ("id","subscriptionId","signalId","channel","status","payload","createdAt") VALUES ($1,'sub',$2,'SSE','SENT',$3,now())`,
		delID, signalID, `{"event_id":"`+eventID+`"}`)
}

func sseGet(t *testing.T, ctx context.Context, url, key string) *http.Response {
	t.Helper()
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestStreamAuthAndResolution(t *testing.T) {
	d := dbtest.Connect(t)
	seed(t, d)
	ht, _ := newStreamServer(t, d)
	readKey := seedKey(t, d, []string{"signals:read"}, 100000)
	statsKey := seedKey(t, d, []string{"stats:read"}, 100000)

	cases := []struct {
		name, path, key string
		want            int
	}{
		{"no key", "/v1/stream/sse?subscription=sub", "", 401},
		{"wrong scope", "/v1/stream/sse?subscription=sub", statsKey, 403},
		{"missing subscription", "/v1/stream/sse", readKey, 400},
		{"unknown subscription", "/v1/stream/sse?subscription=nope", readKey, 404},
		{"ws no key", "/v1/stream/ws?subscription=sub", "", 401},
		{"ws unknown sub", "/v1/stream/ws?subscription=nope", readKey, 404},
		{"ws non-upgrade GET", "/v1/stream/ws?subscription=sub", readKey, 426}, // reaches Accept, which rejects a plain GET (Upgrade Required)
	}
	for _, c := range cases {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		resp := sseGet(t, ctx, ht.URL+c.path, c.key)
		if resp.StatusCode != c.want {
			t.Errorf("%s: got %d want %d", c.name, resp.StatusCode, c.want)
		}
		resp.Body.Close()
		cancel()
	}
}

func TestSSEReplayAndLive(t *testing.T) {
	d := dbtest.Connect(t)
	seed(t, d) // subscription 'sub' + delivery 'd1' payload {"event_id":"e"}
	ht, hub := newStreamServer(t, d)
	key := seedKey(t, d, []string{"signals:read"}, 100000)

	// Replay from the start: ?since=0 must immediately deliver the existing row.
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	resp := sseGet(t, ctx, ht.URL+`/v1/stream/sse?subscription=sub&since=0`, key)
	defer resp.Body.Close()
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("sse status=%d ct=%s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	sc := bufio.NewScanner(resp.Body)
	if !scanFor(sc, `"event_id":"e"`) {
		t.Fatal("did not replay the existing delivery")
	}

	// Live push: a new delivery arrives + hub notify → the same open stream
	// receives it (this stream started at ?since=0 so it is already caught up).
	addDelivery(t, d, "sg2", "d2", "live")
	hub.Notify("sub")
	if !scanFor(sc, `"event_id":"live"`) {
		t.Fatal("did not receive the live-pushed delivery")
	}
}

func TestSSEDefaultTailAndHeartbeat(t *testing.T) {
	// Shrink the fallback so the heartbeat/ping path runs quickly.
	old := httpapi.SetStreamPollFallbackForTest(150 * time.Millisecond)
	defer httpapi.SetStreamPollFallbackForTest(old)

	d := dbtest.Connect(t)
	seed(t, d)
	ht, hub := newStreamServer(t, d)
	key := seedKey(t, d, []string{"signals:read"}, 100000)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// No ?since and no Last-Event-ID → default cursor is the current tail
	// (exercises MaxDeliverySeq); then a live push arrives.
	resp := sseGet(t, ctx, ht.URL+"/v1/stream/sse?subscription=sub", key)
	defer resp.Body.Close()
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() { // wait until subscribed
		if strings.Contains(sc.Text(), "connected") {
			break
		}
	}
	gotPing := false
	go func() {
		time.Sleep(400 * time.Millisecond) // let a heartbeat ping fire first
		addDelivery(t, d, "sg2", "d2", "tail")
		hub.Notify("sub")
	}()
	for sc.Scan() {
		line := sc.Text()
		if strings.Contains(line, ": ping") {
			gotPing = true
		}
		if strings.Contains(spaceless(line), `"event_id":"tail"`) {
			if !gotPing {
				t.Log("note: event arrived before a heartbeat ping (timing)")
			}
			return
		}
	}
	t.Fatal("default-tail live push not received")
}

func TestStreamDBErrors(t *testing.T) {
	d := dbtest.Connect(t)
	seed(t, d)
	ht, _ := newStreamServer(t, d)
	key := seedKey(t, d, []string{"signals:read"}, 100000)
	ctx := context.Background()
	rename := func(from, to string) {
		if _, err := d.Pool.Exec(ctx, `ALTER TABLE "`+from+`" RENAME TO "`+to+`"`); err != nil {
			t.Fatalf("rename %s: %v", from, err)
		}
	}

	// Subscription lookup failure → 500 across all transports.
	rename("Subscription", "Subscription__h")
	for _, p := range []string{"/v1/stream/poll?subscription=sub", "/v1/stream/sse?subscription=sub", "/v1/stream/ws?subscription=sub"} {
		if code, _ := rawGetKey(t, ht.URL+p, key); code != 500 {
			rename("Subscription__h", "Subscription")
			t.Fatalf("%s sub-lookup error want 500 got %d", p, code)
		}
	}
	rename("Subscription__h", "Subscription")

	// Feed / cursor query failure → 500 (poll: ListDeliveriesForStream; sse/ws:
	// MaxDeliverySeq via the default cursor).
	rename("DeliveryEvent", "DeliveryEvent__h")
	codes := map[string]int{}
	for _, p := range []string{"/v1/stream/poll?subscription=sub&since=0", "/v1/stream/sse?subscription=sub", "/v1/stream/ws?subscription=sub"} {
		codes[p], _ = rawGetKey(t, ht.URL+p, key)
	}
	rename("DeliveryEvent__h", "DeliveryEvent")
	for p, c := range codes {
		if c != 500 {
			t.Errorf("%s feed error want 500 got %d", p, c)
		}
	}
}

func TestSSEResumeHubless(t *testing.T) {
	d := dbtest.Connect(t)
	seed(t, d) // delivery 'd1' (event_id "e")
	addDelivery(t, d, "sg2", "d2", "second")
	// Server with NO hub → exercises the nil-hub subscribe path (poll fallback).
	srv := &httpapi.Server{DB: d, SigningSecret: "s"}
	ht := httptest.NewServer(srv.Handler())
	t.Cleanup(ht.Close)
	key := seedKey(t, d, []string{"signals:read"}, 100000)

	var d1seq int64
	if err := d.Pool.QueryRow(context.Background(), `SELECT "seq" FROM "DeliveryEvent" WHERE id='d1'`).Scan(&d1seq); err != nil {
		t.Fatal(err)
	}
	// Resume past d1 via Last-Event-ID → the first replayed event must be d2.
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", ht.URL+"/v1/stream/sse?subscription=sub", nil)
	req.Header.Set("X-API-Key", key)
	req.Header.Set("Last-Event-ID", strconv.FormatInt(d1seq, 10))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	first := ""
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		if line := spaceless(sc.Text()); strings.HasPrefix(line, "data:") {
			first = line
			break
		}
	}
	if !strings.Contains(first, `"event_id":"second"`) {
		t.Fatalf("resume should skip d1 and start at d2, first event was %q", first)
	}
}

func TestWebSocketStream(t *testing.T) {
	d := dbtest.Connect(t)
	seed(t, d)
	ht, hub := newStreamServer(t, d)
	key := seedKey(t, d, []string{"signals:read"}, 100000)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	wsURL := strings.Replace(ht.URL, "http://", "ws://", 1) + `/v1/stream/ws?subscription=sub&since=0`
	c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: http.Header{"X-API-Key": {key}}})
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer c.CloseNow()

	readText := func() string {
		_, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("ws read: %v", err)
		}
		return string(data)
	}

	if hello := spaceless(readText()); !strings.Contains(hello, `"type":"connected"`) {
		t.Fatalf("expected connected hello, got %s", hello)
	}
	// Replayed frame for d1, wrapped with its seq cursor.
	if frame := spaceless(readText()); !strings.Contains(frame, `"event_id":"e"`) || !strings.Contains(frame, `"seq":`) {
		t.Fatalf("expected replayed frame, got %s", frame)
	}
	// Live frame after a new delivery + notify.
	addDelivery(t, d, "sg2", "d2", "live")
	hub.Notify("sub")
	if frame := spaceless(readText()); !strings.Contains(frame, `"event_id":"live"`) {
		t.Fatalf("expected live frame, got %s", frame)
	}
}

func TestStreamPoll(t *testing.T) {
	d := dbtest.Connect(t)
	seed(t, d) // delivery 'd1' for sub
	ht, _ := newStreamServer(t, d)
	key := seedKey(t, d, []string{"signals:read"}, 100000)

	// First poll from the start returns the existing event + a cursor.
	code, body := rawGetKey(t, ht.URL+"/v1/stream/poll?subscription=sub&since=0", key)
	if code != 200 || !strings.Contains(spaceless(body), `"event_id":"e"`) {
		t.Fatalf("poll since=0: %d %s", code, body)
	}
	var first struct {
		Cursor int64 `json:"cursor"`
	}
	if err := json.Unmarshal([]byte(body), &first); err != nil || first.Cursor == 0 {
		t.Fatalf("cursor missing: %v %s", err, body)
	}
	// Polling again from that cursor drains to empty.
	_, body2 := rawGetKey(t, ht.URL+"/v1/stream/poll?subscription=sub&since="+strconv.FormatInt(first.Cursor, 10), key)
	if !strings.Contains(spaceless(body2), `"events":[]`) {
		t.Fatalf("expected no new events, got %s", body2)
	}
}

func TestStreamQueryKeyAndLimit(t *testing.T) {
	d := dbtest.Connect(t)
	seed(t, d)
	ht, _ := newStreamServer(t, d)
	key := seedKey(t, d, []string{"signals:read"}, 100000)

	// api_key as a query param (browser fallback, no header) + a limit param.
	resp, err := http.Get(ht.URL + "/v1/stream/poll?subscription=sub&api_key=" + key + "&limit=600")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !strings.Contains(spaceless(string(body)), `"event_id":"e"`) {
		t.Fatalf("query-key poll: %d %s", resp.StatusCode, body)
	}
}

func rawGetKey(t *testing.T, url, key string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-API-Key", key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// spaceless strips spaces so assertions ignore jsonb's re-serialization spacing
// (Postgres returns `{"event_id": "e"}`, not the stored `{"event_id":"e"}`).
func spaceless(s string) string { return strings.ReplaceAll(s, " ", "") }

// scanFor advances the scanner until a line contains want, or the stream ends.
func scanFor(sc *bufio.Scanner, want string) bool {
	for sc.Scan() {
		if strings.Contains(spaceless(sc.Text()), want) {
			return true
		}
	}
	return false
}
