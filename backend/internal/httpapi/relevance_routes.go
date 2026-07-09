package httpapi

import (
	"net/http"
	"strconv"

	"github.com/worldsignal/backend/internal/llm"
)

// registerRelevanceRoutes wires the smart-signals feed: a personalized ranked
// feed per profile, feedback, interest editing, and AI draft-from-document.
//
// SECURITY — tenant scoping (TODO with the brand/ownership model): these handlers
// take a subscription id from the path/body and operate on it after only a
// scope check, exactly like the existing subscription REST API (which is not
// owner-scoped — a valid key sees all subscriptions in the deployment). This is
// acceptable only while the platform is single-tenant per deployment. When the
// multi-brand model lands (Subscription.brandId + API keys bound to an owner /
// set of brands), EVERY handler here MUST verify the subscription belongs to a
// brand the caller may access — otherwise this becomes an IDOR. Enforce it in one
// place (a helper that resolves+authorizes the subscription for the identity).
func (s *Server) registerRelevanceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/subscriptions/{id}/feed", s.requireAPIKey("signals:read", s.subscriptionFeed))
	mux.HandleFunc("PATCH /v1/subscriptions/{id}/interests", s.requireAPIKey("subscriptions:write", s.setInterests))
	mux.HandleFunc("POST /v1/feedback", s.requireAPIKey("subscriptions:write", s.postFeedback))
	mux.HandleFunc("POST /v1/profiles/draft-from-document", s.requireAPIKey("subscriptions:write", s.draftProfile))
}

// feedItem is the API shape of one ranked signal.
type feedItem struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Summary   string   `json:"summary"`
	EventType string   `json:"eventType"`
	Country   string   `json:"country"`
	Region    string   `json:"region"`
	Sentiment string   `json:"sentiment"`
	Influence string   `json:"influence"`
	Severity  string   `json:"severity"`
	AgeHours  float64  `json:"ageHours"`
	Score     float64  `json:"score"`
	Reasons   []string `json:"reasons"`
}

// maxFeedLimit bounds the requested page size so a user-supplied `limit` can't
// drive an oversized slice allocation (CodeQL: excessive-size allocation).
const maxFeedLimit = 200

// clampLimit constrains a requested page size to a safe, positive range.
func clampLimit(n int) int {
	if n <= 0 {
		return 30
	}
	if n > maxFeedLimit {
		return maxFeedLimit
	}
	return n
}

func (s *Server) subscriptionFeed(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	limit := clampLimit(queryInt(r, "limit", 30))
	sinceHours := queryInt(r, "sinceHours", 72)
	minScore := queryFloat(r, "minScore", 0)

	ranked, err := s.DB.RankedFeed(r.Context(), id, sinceHours, limit*3)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := make([]feedItem, 0, limit)
	for _, sc := range ranked {
		if sc.Score < minScore {
			continue
		}
		out = append(out, feedItem{
			ID: sc.ID, Title: sc.Title, Summary: sc.Summary, EventType: sc.EventType,
			Country: sc.Country, Region: sc.Region, Sentiment: sc.Sentiment,
			Influence: sc.Influence, Severity: sc.Severity, AgeHours: round1(sc.AgeHours),
			Score: round1(sc.Score), Reasons: sc.Reasons,
		})
		if len(out) >= limit {
			break
		}
	}
	writeJSON(w, http.StatusOK, struct {
		Data []feedItem `json:"data"`
	}{out})
}

func (s *Server) setInterests(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Interests map[string]float64 `json:"interests"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if err := s.DB.SetSubscriptionInterests(r.Context(), id, body.Interests); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "interests": body.Interests})
}

var validFeedback = map[string]bool{"OPEN": true, "UP": true, "DOWN": true}

func (s *Server) postFeedback(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SubscriptionID string `json:"subscriptionId"`
		SignalID       string `json:"signalId"`
		Action         string `json:"action"`
	}
	if err := readJSON(r, &body); err != nil || body.SubscriptionID == "" || body.SignalID == "" || !validFeedback[body.Action] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "subscriptionId, signalId and action (OPEN|UP|DOWN) required"})
		return
	}
	if err := s.DB.RecordFeedback(r.Context(), body.SubscriptionID, body.SignalID, body.Action); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) draftProfile(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text string `json:"text"`
	}
	if err := readJSON(r, &body); err != nil || len(body.Text) < 20 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "text (a document, ≥20 chars) required"})
		return
	}
	gw := llm.NewDynamicGateway(s.ResolveLLMKey)
	draft := llm.DraftProfileFromDocument(r.Context(), gw, body.Text)
	writeJSON(w, http.StatusOK, draft)
}

func queryInt(r *http.Request, key string, def int) int {
	if v := r.URL.Query().Get(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func queryFloat(r *http.Request, key string, def float64) float64 {
	if v := r.URL.Query().Get(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}
