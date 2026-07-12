package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
)

func sd(seq int64) db.StreamDelivery { return db.StreamDelivery{Seq: seq, Payload: db.RawJSON(`{}`)} }

// TestStreamCore drives every branch of the shared feed loop with fakes.
func TestStreamCore(t *testing.T) {
	noHeartbeat := func() error { return nil }

	// feed error → return before emitting.
	streamCore(context.Background(), 0,
		func(int64) ([]db.StreamDelivery, error) { return nil, errors.New("boom") },
		func(db.StreamDelivery) error { t.Fatal("emit ran after feed error"); return nil },
		noHeartbeat, nil, time.Hour)

	// emit error → return.
	streamCore(context.Background(), 0,
		func(int64) ([]db.StreamDelivery, error) { return []db.StreamDelivery{sd(1)}, nil },
		func(db.StreamDelivery) error { return errors.New("write fail") },
		noHeartbeat, nil, time.Hour)

	// drain: a full batch re-queries immediately; the empty follow-up ends via ctx.
	full := make([]db.StreamDelivery, streamBatch)
	for i := range full {
		full[i] = sd(int64(i + 1))
	}
	ctx, cancel := context.WithCancel(context.Background())
	calls, emitted := 0, 0
	streamCore(ctx, 0,
		func(int64) ([]db.StreamDelivery, error) {
			calls++
			if calls == 1 {
				return full, nil
			}
			cancel()
			return nil, nil
		},
		func(db.StreamDelivery) error { emitted++; return nil },
		noHeartbeat, nil, time.Hour)
	if calls < 2 || emitted != streamBatch {
		t.Fatalf("drain: calls=%d emitted=%d", calls, emitted)
	}

	// wake path: first query empty + arm wake; the select unblocks and re-queries.
	wake := make(chan struct{}, 1)
	ctx2, cancel2 := context.WithCancel(context.Background())
	c2 := 0
	streamCore(ctx2, 0,
		func(int64) ([]db.StreamDelivery, error) {
			c2++
			if c2 == 1 {
				wake <- struct{}{}
				return nil, nil
			}
			cancel2()
			return nil, nil
		},
		func(db.StreamDelivery) error { return nil }, noHeartbeat, wake, time.Hour)
	if c2 < 2 {
		t.Fatalf("wake: expected re-query, calls=%d", c2)
	}

	// heartbeat fires and errors → return.
	streamCore(context.Background(), 0,
		func(int64) ([]db.StreamDelivery, error) { return nil, nil },
		func(db.StreamDelivery) error { return nil },
		func() error { return errors.New("hb") }, nil, time.Millisecond)

	// nil heartbeat + tick → loops until ctx expires (guards the nil-heartbeat case).
	ctx3, cancel3 := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel3()
	streamCore(ctx3, 0,
		func(int64) ([]db.StreamDelivery, error) { return nil, nil },
		func(db.StreamDelivery) error { return nil }, nil, nil, time.Millisecond)
}

// --- fake ResponseWriters for streamSSE edge cases ---------------------------

type noFlushWriter struct {
	h    http.Header
	code int
}

func (n *noFlushWriter) Header() http.Header {
	if n.h == nil {
		n.h = http.Header{}
	}
	return n.h
}
func (n *noFlushWriter) Write(b []byte) (int, error) { return len(b), nil }
func (n *noFlushWriter) WriteHeader(c int)           { n.code = c }

type failFlushWriter struct {
	h        http.Header
	okWrites int
	writes   int
}

func (f *failFlushWriter) Header() http.Header {
	if f.h == nil {
		f.h = http.Header{}
	}
	return f.h
}
func (f *failFlushWriter) WriteHeader(int) {}
func (f *failFlushWriter) Flush()          {}
func (f *failFlushWriter) Write(b []byte) (int, error) {
	f.writes++
	if f.writes > f.okWrites {
		return 0, errors.New("client gone")
	}
	return len(b), nil
}

func seedStreamSub(t *testing.T, d *db.DB, id string, withDelivery bool) {
	t.Helper()
	ctx := context.Background()
	ex := func(q string, a ...any) {
		if _, err := d.Pool.Exec(ctx, q, a...); err != nil {
			t.Fatal(err)
		}
	}
	ex(`INSERT INTO "Subscription"("id","name","channel","filter","config","createdAt") VALUES($1,'s','SSE','{}','{}',now())`, id)
	if withDelivery {
		ex(`INSERT INTO "Signal"("id","title","summary","status","severity","confidence","sourceCount","firstSeenAt","lastSeenAt","updatedAt") VALUES($1,'t','s','CONFIRMED','HIGH',0.9,1,now(),now(),now())`, id)
		ex(`INSERT INTO "DeliveryEvent"("id","subscriptionId","signalId","channel","status","payload","createdAt") VALUES($1,$1,$1,'SSE','SENT','{}',now())`, id)
	}
}

// TestStreamSSEWriterEdges covers the SSE handler paths a live client can't force
// deterministically: no Flusher, and writes failing at connect / event / ping.
func TestStreamSSEWriterEdges(t *testing.T) {
	d := dbtest.Connect(t)
	s := &Server{DB: d}
	req := func(id, extra string) *http.Request {
		return httptest.NewRequest("GET", "/v1/stream/sse?subscription="+id+extra, nil)
	}

	// No Flusher → 500.
	seedStreamSub(t, d, "sw-noflush", false)
	nf := &noFlushWriter{}
	s.streamSSE(nf, req("sw-noflush", ""))
	if nf.code != 500 {
		t.Fatalf("no-flusher want 500 got %d", nf.code)
	}

	// Write fails on the very first (connected) write → returns before streaming.
	seedStreamSub(t, d, "sw-conn", false)
	s.streamSSE(&failFlushWriter{okWrites: 0}, req("sw-conn", ""))

	// Write fails while emitting an event (connected ok, event write fails).
	seedStreamSub(t, d, "sw-emit", true)
	s.streamSSE(&failFlushWriter{okWrites: 1}, req("sw-emit", "&since=0"))

	// Write fails on a heartbeat ping (no events; tiny interval so the tick fires).
	old := SetStreamPollFallbackForTest(2 * time.Millisecond)
	defer SetStreamPollFallbackForTest(old)
	seedStreamSub(t, d, "sw-ping", false)
	s.streamSSE(&failFlushWriter{okWrites: 1}, req("sw-ping", ""))
}
