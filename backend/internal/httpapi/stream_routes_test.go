package httpapi_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
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
