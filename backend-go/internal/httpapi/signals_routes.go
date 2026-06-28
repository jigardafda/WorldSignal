package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/worldsignal/backend/internal/db"
)

func (s *Server) registerSignalRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/signals", s.listSignals)
	mux.HandleFunc("GET /v1/signals/{id}", s.getSignal)
}

// restTag mirrors the REST tag shape {code,label,confidence}.
type restTag struct {
	Code       string  `json:"code"`
	Label      string  `json:"label"`
	Confidence float64 `json:"confidence"`
}

// restSource mirrors {publisher,url,publishedAt,relation}.
type restSource struct {
	Publisher   string         `json:"publisher"`
	URL         *string        `json:"url"`
	PublishedAt *db.PrismaTime `json:"publishedAt"`
	Relation    string         `json:"relation"`
}

// restSignal mirrors serializeSignal() in routes.ts (field order preserved).
type restSignal struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	Summary      string        `json:"summary"`
	WhatHappened *string       `json:"whatHappened"`
	WhyItMatters *string       `json:"whyItMatters"`
	Status       string        `json:"status"`
	Severity     string        `json:"severity"`
	Confidence   float64       `json:"confidence"`
	EventType    *string       `json:"eventType"`
	Country      *string       `json:"country"`
	SourceCount  int           `json:"sourceCount"`
	FirstSeenAt  db.PrismaTime `json:"firstSeenAt"`
	LastSeenAt   db.PrismaTime `json:"lastSeenAt"`
	Tags         []restTag     `json:"tags"`
	Sources      []restSource  `json:"sources"`
}

func serializeRESTSignal(a *db.SignalAggregate) restSignal {
	tags := make([]restTag, len(a.Tags))
	for i, t := range a.Tags {
		tags[i] = restTag{Code: t.Code, Label: t.Label, Confidence: t.Confidence}
	}
	sources := make([]restSource, len(a.Sources))
	for i, src := range a.Sources {
		sources[i] = restSource{
			Publisher:   src.Publisher,
			URL:         src.URL,
			PublishedAt: db.NewTimePtr(src.PublishedAt),
			Relation:    src.Relation,
		}
	}
	return restSignal{
		ID: a.ID, Title: a.Title, Summary: a.Summary,
		WhatHappened: a.WhatHappened, WhyItMatters: a.WhyItMatters,
		Status: a.Status, Severity: a.Severity, Confidence: a.Confidence,
		EventType: a.EventType, Country: a.Country, SourceCount: a.SourceCount,
		FirstSeenAt: db.NewTime(a.FirstSeenAt), LastSeenAt: db.NewTime(a.LastSeenAt),
		Tags: tags, Sources: sources,
	}
}

func (s *Server) listSignals(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := db.SignalFilter{Limit: 50, Offset: 0}
	if v := q.Get("country"); v != "" {
		f.Country = &v
	}
	if v := q.Get("status"); v != "" {
		f.Status = &v
	}
	if v := q.Get("minConfidence"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			f.MinConfidence = &n
		}
	}
	if v := q.Get("since"); v != "" {
		if ts, err := parseJSDate(v); err == nil {
			f.Since = &ts
		}
	}
	if v := q.Get("search"); v != "" {
		f.Search = &v
	}
	if v := q.Get("tags"); v != "" {
		f.Tags = strings.Split(v, ",")
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.Limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.Offset = n
		}
	}

	rows, err := s.DB.ListSignals(r.Context(), f)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := make([]restSignal, len(rows))
	for i, a := range rows {
		out[i] = serializeRESTSignal(a)
	}
	writeJSON(w, http.StatusOK, struct {
		Data []restSignal `json:"data"`
	}{out})
}

func (s *Server) getSignal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a, err := s.DB.GetSignal(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if a == nil {
		writeJSON(w, http.StatusNotFound, struct {
			Error string `json:"error"`
		}{"not found"})
		return
	}
	writeJSON(w, http.StatusOK, serializeRESTSignal(a))
}

// parseJSDate parses the formats new Date(string) commonly handles in our API.
func parseJSDate(v string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.000Z", "2006-01-02"} {
		if t, err := time.Parse(layout, v); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unparseable date %q", v)
}
