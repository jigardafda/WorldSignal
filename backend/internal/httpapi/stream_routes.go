package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/coder/websocket"
	"github.com/worldsignal/backend/internal/db"
)

// streamPollFallback re-queries the delivery feed even without a hub wakeup, so
// streams still progress if a notification is missed (or in a split api/worker
// deployment where the in-process hub isn't shared). Doubles as an idle ping.
const streamPollFallback = 15 * time.Second

func (s *Server) registerStreamRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/stream/sse", s.streamAuth("signals:read", s.streamSSE))
	mux.HandleFunc("GET /v1/stream/ws", s.streamAuth("signals:read", s.streamWS))
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
	ticker := time.NewTicker(streamPollFallback)
	defer ticker.Stop()

	for {
		rows, err := s.DB.ListDeliveriesForStream(ctx, sub.ID, cursor, 200)
		if err != nil {
			return
		}
		for _, e := range rows {
			if _, err := w.Write([]byte("id: " + strconv.FormatInt(e.Seq, 10) + "\nevent: signal\ndata: " + string(e.Payload) + "\n\n")); err != nil {
				return
			}
			cursor = e.Seq
		}
		if len(rows) > 0 {
			flusher.Flush()
		}
		if len(rows) == 200 {
			continue // more may be buffered; drain before waiting
		}
		select {
		case <-ctx.Done():
			return
		case <-wake:
		case <-ticker.C:
			if _, err := w.Write([]byte(": ping\n\n")); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// streamWS serves the same feed over a WebSocket. Frames are
// {"seq":<n>,"payload":<envelope>}; clients may send {"ack":<seq>} (advisory —
// the server already advances as it writes). The read loop also detects
// disconnects and answers pings.
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

	// Reader: surfaces disconnects (cancels ctx) and processes control frames.
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
	ticker := time.NewTicker(streamPollFallback)
	defer ticker.Stop()

	_ = c.Write(ctx, websocket.MessageText, []byte(`{"type":"connected","subscription":"`+sub.ID+`"}`))

	for {
		rows, err := s.DB.ListDeliveriesForStream(ctx, sub.ID, cursor, 200)
		if err != nil {
			return
		}
		for _, e := range rows {
			frame := append([]byte(`{"seq":`+strconv.FormatInt(e.Seq, 10)+`,"payload":`), e.Payload...)
			frame = append(frame, '}')
			if err := c.Write(ctx, websocket.MessageText, frame); err != nil {
				return
			}
			cursor = e.Seq
		}
		if len(rows) == 200 {
			continue
		}
		select {
		case <-ctx.Done():
			_ = c.Close(websocket.StatusNormalClosure, "")
			return
		case <-wake:
		case <-ticker.C:
			if err := c.Ping(ctx); err != nil {
				return
			}
		}
	}
}
