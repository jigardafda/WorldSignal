// Package httpapi implements the REST surface (/health, /v1/*) and mounts the
// GraphQL endpoint, byte-compatible with the Fastify + graphql-yoga backend.
package httpapi

import (
	"net/http"
	"sync"
	"time"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/jsonx"
	"github.com/worldsignal/backend/internal/taxonomy"
)

// Enqueuer abstracts the job queue so the API can enqueue work without
// depending on the worker implementation.
type Enqueuer interface {
	EnqueueFetchSource(sourceID string) error
	EnqueueSendDelivery(deliveryID string) error
}

// Server holds dependencies for the HTTP handlers.
type Server struct {
	DB            *db.DB
	Enqueue       Enqueuer
	SigningSecret string
	SessionTTL    time.Duration // session lifetime; defaults to 7 days when zero
	// LLM: the system key/model from the environment. Admin-managed DB keys
	// (encrypted with SigningSecret) take precedence at runtime.
	OpenAIAPIKey string
	OpenAIModel  string

	// llmCache memoizes the resolved active key for a short TTL so per-article
	// enrichment doesn't hit the DB on every call.
	llmCacheMu    sync.Mutex
	llmCacheKey   string
	llmCacheModel string
	llmCacheExp   time.Time
}

func (s *Server) sessionTTL() time.Duration {
	if s.SessionTTL <= 0 {
		return 7 * 24 * time.Hour
	}
	return s.SessionTTL
}

// Handler returns the root http.Handler with CORS applied.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health) // open: liveness probe
	mux.HandleFunc("GET /v1/stats", s.requireAPIKey("stats:read", s.stats))
	mux.HandleFunc("GET /v1/taxonomy", s.requireAPIKey("signals:read", s.taxonomy))
	s.registerSignalRoutes(mux)
	s.registerSourceRoutes(mux)
	s.registerSubscriptionRoutes(mux)
	s.registerGraphQL(mux)
	return cors(mux)
}

// cors mirrors the permissive onRequest hook in routes.ts.
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// writeJSON writes v as JSON.stringify-compatible bytes with Fastify's content type.
func writeJSON(w http.ResponseWriter, status int, v any) {
	b, err := jsonx.Marshal(v)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal"}`))
		return
	}
	writeRaw(w, status, b)
}

// writeRaw writes pre-serialized JSON bytes.
func writeRaw(w http.ResponseWriter, status int, b []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, struct {
		Status  string `json:"status"`
		Service string `json:"service"`
	}{"ok", "worldsignal"})
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	st, err := s.DB.GetStats(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Sources           int `json:"sources"`
		Articles          int `json:"articles"`
		Signals           int `json:"signals"`
		DeliveriesSent    int `json:"deliveriesSent"`
		DeliveriesPending int `json:"deliveriesPending"`
	}{st.Sources, st.Articles, st.Signals, st.DeliveriesSent, st.DeliveriesPending})
}

func (s *Server) taxonomy(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, taxonomy.Taxonomy)
}
