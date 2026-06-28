package httpapi

import (
	"context"
	"io"
	"net/http"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/gql"
	"github.com/worldsignal/backend/internal/jsonx"
	"github.com/worldsignal/backend/internal/taxonomy"
)

func (s *Server) registerGraphQL(mux *http.ServeMux) {
	mux.HandleFunc("POST /graphql", s.handleGraphQL)
	mux.HandleFunc("GET /graphql", s.handleGraphQL)
}

func (s *Server) handleGraphQL(w http.ResponseWriter, r *http.Request) {
	var req gql.Request
	if r.Method == http.MethodGet {
		q := r.URL.Query()
		req.Query = q.Get("query")
		req.OperationName = q.Get("operationName")
		if v := q.Get("variables"); v != "" {
			_ = jsonUnmarshal([]byte(v), &req.Variables)
		}
	} else {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeRaw(w, http.StatusOK, []byte(`{"errors":[{"message":"could not read body"}]}`))
			return
		}
		if err := jsonUnmarshal(body, &req); err != nil {
			writeRaw(w, http.StatusOK, []byte(`{"errors":[{"message":"invalid json body"}]}`))
			return
		}
	}
	out := gql.Execute(r.Context(), s.root(), req)
	writeRaw(w, http.StatusOK, out)
}

func (s *Server) root() gql.Root {
	return gql.Root{
		Query: map[string]gql.FieldResolver{
			"signals":       s.resolveSignals,
			"signal":        s.resolveSignal,
			"sources":       s.resolveSources,
			"subscriptions": s.resolveSubscriptions,
			"taxonomy":      func(context.Context, map[string]any) (any, error) { return taxonomy.Taxonomy, nil },
			"stats":         s.resolveStats,
		},
		Mutation: s.mutationResolvers(),
	}
}

// --- query resolvers ---

func signalToMap(a *db.SignalAggregate) map[string]any {
	tags := make([]any, len(a.Tags))
	for i, t := range a.Tags {
		tags[i] = map[string]any{"code": t.Code, "confidence": t.Confidence}
	}
	sources := make([]any, len(a.Sources))
	for i, src := range a.Sources {
		sources[i] = map[string]any{"publisher": src.Publisher, "url": src.URL, "publishedAt": src.PublishedAt}
	}
	return map[string]any{
		"id": a.ID, "title": a.Title, "summary": a.Summary,
		"whatHappened": a.WhatHappened, "whyItMatters": a.WhyItMatters,
		"status": a.Status, "severity": a.Severity, "confidence": a.Confidence,
		"eventType": a.EventType, "country": a.Country, "sourceCount": a.SourceCount,
		"firstSeenAt": a.FirstSeenAt, "lastSeenAt": a.LastSeenAt,
		"tags": tags, "sources": sources,
	}
}

func (s *Server) resolveSignals(ctx context.Context, args map[string]any) (any, error) {
	f := db.SignalFilter{Limit: toInt(args["limit"], 50), Offset: toInt(args["offset"], 0)}
	if filter, ok := args["filter"].(map[string]any); ok {
		if v, ok := filter["country"].(string); ok {
			f.Country = &v
		}
		if v, ok := filter["status"].(string); ok {
			f.Status = &v
		}
		if v, ok := filter["search"].(string); ok {
			f.Search = &v
		}
		if mc, ok := toFloatOK(filter["minConfidence"]); ok {
			f.MinConfidence = &mc
		}
		if tags, ok := filter["tags"].([]any); ok {
			for _, t := range tags {
				if ts, ok := t.(string); ok {
					f.Tags = append(f.Tags, ts)
				}
			}
		}
	}
	rows, err := s.DB.ListSignals(ctx, f)
	if err != nil {
		return nil, err
	}
	out := make([]any, len(rows))
	for i, a := range rows {
		out[i] = signalToMap(a)
	}
	return out, nil
}

func (s *Server) resolveSignal(ctx context.Context, args map[string]any) (any, error) {
	id, _ := args["id"].(string)
	a, err := s.DB.GetSignal(ctx, id)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, nil
	}
	return signalToMap(a), nil
}

func (s *Server) resolveSources(ctx context.Context, _ map[string]any) (any, error) {
	rows, err := s.DB.ListSources(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]any, len(rows))
	for i, src := range rows {
		out[i] = sourceToGqlMap(src)
	}
	return out, nil
}

func (s *Server) resolveSubscriptions(ctx context.Context, _ map[string]any) (any, error) {
	rows, err := s.DB.ListSubscriptionsBasic(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]any, len(rows))
	for i, sub := range rows {
		out[i] = map[string]any{
			"id": sub.ID, "name": sub.Name, "channel": sub.Channel, "enabled": sub.Enabled,
			"filter": sub.Filter, "config": sub.Config, "createdAt": sub.CreatedAt,
		}
	}
	return out, nil
}

func (s *Server) resolveStats(ctx context.Context, _ map[string]any) (any, error) {
	st, err := s.DB.GetStats(ctx)
	if err != nil {
		return nil, err
	}
	// JSON scalar; ordered struct preserves key order sources,articles,signals,deliveriesSent.
	return struct {
		Sources        int `json:"sources"`
		Articles       int `json:"articles"`
		Signals        int `json:"signals"`
		DeliveriesSent int `json:"deliveriesSent"`
	}{st.Sources, st.Articles, st.Signals, st.DeliveriesSent}, nil
}

// --- helpers ---

func timePtr(t *db.PrismaTime) any {
	if t == nil {
		return nil
	}
	return t.Time
}

func toInt(v any, def int) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return def
}

func toFloatOK(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

func jsonUnmarshal(b []byte, v any) error {
	return jsonx.Unmarshal(b, v)
}
