package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/coder/websocket"
	"github.com/worldsignal/backend/internal/db"
)

// streamPollFallback re-queries the delivery feed even without a hub wakeup, so
// streams still progress if a notification is missed (or in a split api/worker
// deployment where the in-process hub isn't shared). Doubles as an idle ping.
// A var (not const) so tests can shrink it to exercise the heartbeat path.
var streamPollFallback = 15 * time.Second

func (s *Server) registerStreamRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/stream/sse", s.streamAuth("signals:read", s.streamSSE))
	mux.HandleFunc("GET /v1/stream/ws", s.streamAuth("signals:read", s.streamWS))
	mux.HandleFunc("GET /v1/stream/poll", s.streamAuth("signals:read", s.streamPoll))
}

// streamPoll is the stateless pull transport: one request returns the events
// after ?since=<cursor> (default 0 = from the start) plus the next cursor. The
// client persists the cursor and polls again — no connection held open.
func (s *Server) streamPoll(w http.ResponseWriter, r *http.Request) {
	sub := s.resolveStreamSub(w, r)
	if sub == nil {
		return
	}
	var since int64
	if v := r.URL.Query().Get("since"); v != "" {
		since, _ = strconv.ParseInt(v, 10, 64)
	}
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	rows, err := s.DB.ListDeliveriesForStream(r.Context(), sub.ID, since, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	cursor := since
	events := make([]map[string]any, 0, len(rows))
	for _, e := range rows {
		events = append(events, map[string]any{"seq": e.Seq, "payload": json.RawMessage(e.Payload)})
		cursor = e.Seq
	}
	writeJSON(w, http.StatusOK, map[string]any{"cursor": cursor, "events": events})
}

// streamAuth is requireAPIKey plus a browser convenience: EventSource and the
// WebSocket API can't set request headers, so the key may arrive as ?api_key=.
// Server clients should prefer the Authorization header (URLs may be logged).
func (s *Server) streamAuth(scope string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if keyFromRequest(r) == "" {
			if k := r.URL.Query().Get("api_key"); k != "" {
				r.Header.Set("X-API-Key", k)
			}
		}
		s.requireAPIKey(scope, next)(w, r)
	}
}

// resolveStreamSub validates ?subscription=<id>, writing an error and returning
// nil when it is missing or unknown.
func (s *Server) resolveStreamSub(w http.ResponseWriter, r *http.Request) *db.StreamSubscription {
	id := r.URL.Query().Get("subscription")
	if id == "" {
		apiKeyError(w, http.StatusBadRequest, "missing ?subscription=<id>")
		return nil
	}
	sub, err := s.DB.GetStreamSubscription(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return nil
	}
	if sub == nil {
		apiKeyError(w, http.StatusNotFound, "subscription not found")
		return nil
	}
	return sub
}

// startCursor picks where the feed begins: an explicit resume point (SSE
// Last-Event-ID header or ?since=) else the current tail so a fresh connection
// streams only new events. ?since=0 replays the full backlog.
func (s *Server) startCursor(r *http.Request, subID string) (int64, error) {
	if v := r.Header.Get("Last-Event-ID"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n, nil
		}
	}
	if v := r.URL.Query().Get("since"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n, nil
		}
	}
	return s.DB.MaxDeliverySeq(r.Context(), subID)
}

// subscribeHub returns a wakeup channel + cancel; when no hub is wired the
// channel is nil (never ready) and the loop relies on the poll fallback.
func (s *Server) subscribeHub(subID string) (<-chan struct{}, func()) {
	if s.Hub == nil {
		return nil, func() {}
	}
	return s.Hub.Subscribe(subID)
}

// streamBatch caps one feed query; a full batch means more may be buffered, so
// the loop drains again before waiting.
const streamBatch = 200

// streamCore is the transport-agnostic feed loop shared by SSE and WebSocket. It
// drains rows after `cursor` via feed, emits each via emit, then blocks on a
// wake / heartbeat tick / ctx cancel and repeats. It returns when a query fails,
// emit or heartbeat errors, or ctx is done. Kept free of HTTP/DB types so it can
// be unit-tested with fakes.
func streamCore(
	ctx context.Context,
	cursor int64,
	feed func(cursor int64) ([]db.StreamDelivery, error),
	emit func(db.StreamDelivery) error,
	heartbeat func() error,
	wake <-chan struct{},
	interval time.Duration,
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		rows, err := feed(cursor)
		if err != nil {
			return
		}
		for _, e := range rows {
			if err := emit(e); err != nil {
				return
			}
			cursor = e.Seq
		}
		if len(rows) == streamBatch {
			continue // more may be buffered; drain before waiting
		}
		select {
		case <-ctx.Done():
			return
		case <-wake:
		case <-ticker.C:
			if heartbeat != nil {
				if err := heartbeat(); err != nil {
					return
				}
			}
		}
	}
}

// feedFor binds a subscription's keyset feed query for streamCore.
func (s *Server) feedFor(ctx context.Context, subID string) func(int64) ([]db.StreamDelivery, error) {
	return func(cursor int64) ([]db.StreamDelivery, error) {
		return s.DB.ListDeliveriesForStream(ctx, subID, cursor, streamBatch)
	}
}

// streamSSE serves a subscription's delivery feed as Server-Sent Events: replay
// from the cursor, then live. Each event carries `id: <seq>` so a reconnecting
// client resumes via Last-Event-ID.
func (s *Server) streamSSE(w http.ResponseWriter, r *http.Request) {
	sub := s.resolveStreamSub(w, r)
	if sub == nil {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
		return
	}
	cursor, err := s.startCursor(r, sub.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable proxy buffering
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(": connected " + sub.ID + "\n\n")); err != nil {
		return
	}
	flusher.Flush()

	ctx := r.Context()
	wake, cancel := s.subscribeHub(sub.ID)
	defer cancel()
	emit := func(e db.StreamDelivery) error {
		if _, err := w.Write([]byte("id: " + strconv.FormatInt(e.Seq, 10) + "\nevent: signal\ndata: " + string(e.Payload) + "\n\n")); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}
	heartbeat := func() error {
		if _, err := w.Write([]byte(": ping\n\n")); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}
	streamCore(ctx, cursor, s.feedFor(ctx, sub.ID), emit, heartbeat, wake, streamPollFallback)
}

// streamWS serves the same feed over a WebSocket. Frames are
// {"seq":<n>,"payload":<envelope>}; a reader goroutine surfaces disconnects and
// processes control frames while the write loop streams.
func (s *Server) streamWS(w http.ResponseWriter, r *http.Request) {
	sub := s.resolveStreamSub(w, r)
	if sub == nil {
		return
	}
	cursor, err := s.startCursor(r, sub.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// Allow any Origin: the connection is authorized by a scoped API-key bearer
	// (query/header), NOT by an ambient cookie, so a cross-origin site cannot
	// forge an authenticated stream — Origin is not the security boundary here.
	// (This is Origin/CSRF control, unrelated to TLS.)
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
	if err != nil {
		return
	}
	defer c.CloseNow()

	ctx, cancelCtx := context.WithCancel(r.Context())
	defer cancelCtx()
	go func() {
		for {
			if _, _, err := c.Read(ctx); err != nil {
				cancelCtx()
				return
			}
		}
	}()

	wake, cancel := s.subscribeHub(sub.ID)
	defer cancel()
	_ = c.Write(ctx, websocket.MessageText, []byte(`{"type":"connected","subscription":"`+sub.ID+`"}`))
	emit := func(e db.StreamDelivery) error {
		frame := append([]byte(`{"seq":`+strconv.FormatInt(e.Seq, 10)+`,"payload":`), e.Payload...)
		return c.Write(ctx, websocket.MessageText, append(frame, '}'))
	}
	streamCore(ctx, cursor, s.feedFor(ctx, sub.ID), emit, func() error { return c.Ping(ctx) }, wake, streamPollFallback)
}
