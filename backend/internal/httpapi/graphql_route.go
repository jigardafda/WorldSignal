package httpapi

import (
	"context"
	"io"
	"net/http"

	"github.com/worldsignal/backend/internal/attributes"
	"github.com/worldsignal/backend/internal/auth"
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
	ctx := s.contextWithIdentity(r)
	out := gql.Execute(ctx, s.root(), req)
	writeRaw(w, http.StatusOK, out)
}

func (s *Server) root() gql.Root {
	q := map[string]gql.FieldResolver{
		"signals":             s.resolveSignals,
		"signal":              s.resolveSignal,
		"sources":             s.resolveSources,
		"sourceCount":         s.resolveSourceCount,
		"sourceCoverage":      s.resolveSourceCoverage,
		"subscriptions":       s.resolveSubscriptions,
		"taxonomy":            s.resolveTaxonomy,
		"stats":               s.resolveStats,
		"countries":           s.resolveCountries,
		"attributeDictionary": s.resolveAttributeDictionary,
	}
	m := s.mutationResolvers()
	s.registerAuthResolvers(q, m)      // login/logout/me + admin (users/teams)
	s.registerEntityResolvers(q, m)    // Phase B: articles, rawItems, deliveries, jobs, analytics, …
	s.registerLLMResolvers(q, m)       // LLM key management (settings:manage)
	s.registerConnectorResolvers(q, m) // email connectors (settings:manage)
	return gql.Root{Query: q, Mutation: m}
}

func (s *Server) resolveTaxonomy(ctx context.Context, _ map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSignalsRead); err != nil {
		return nil, err
	}
	return taxonomy.Taxonomy, nil
}

// resolveAttributeDictionary exposes the closed attribute dictionary so clients
// can render consistent filters, labels and value lists.
func (s *Server) resolveAttributeDictionary(ctx context.Context, _ map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSignalsRead); err != nil {
		return nil, err
	}
	defs := attributes.Definitions()
	out := make([]any, len(defs))
	for i, d := range defs {
		vals := make([]any, len(d.Values()))
		for j, v := range d.Values() {
			vals[j] = map[string]any{"code": v.Code, "label": v.Label}
		}
		out[i] = map[string]any{
			"key": d.Key, "label": d.Label, "kind": string(d.Kind),
			"description": d.Description, "indexed": d.Indexed, "values": vals,
		}
	}
	return out, nil
}

// --- query resolvers ---

// attributeMaps serializes the normalized dictionary attributes of a signal.
func attributeMaps(attrs []db.SignalAttrRow) []any {
	out := make([]any, len(attrs))
	for i, a := range attrs {
		out[i] = map[string]any{
			"key": a.Key, "valueCode": a.ValueCode, "valueText": a.ValueText,
			"valueNum": a.ValueNum, "confidence": a.Confidence,
		}
	}
	return out
}

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
		// Deep-enrichment attributes.
		"region": a.Region, "city": a.City, "locality": a.Locality, "geoScope": a.GeoScope,
		"sentiment": a.Sentiment, "sentimentScore": a.SentimentScore,
		"influence": a.Influence, "relevance": a.Relevance,
		"language": a.Language, "translated": isTranslated(a.Language),
		"originalTitle": a.OriginalTitle, "originalSummary": a.OriginalSummary,
		"attributes": attributeMaps(a.Attributes),
	}
}

// isTranslated reports whether the English narrative was translated from another
// language (a non-empty, non-English detected source language).
func isTranslated(lang *string) bool {
	return lang != nil && *lang != "" && *lang != "en"
}

func (s *Server) resolveSignals(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSignalsRead); err != nil {
		return nil, err
	}
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
		f.Region = strArg(filter, "region")
		f.GeoScope = strArg(filter, "geoScope")
		f.Sentiment = strArg(filter, "sentiment")
		f.Influence = strArg(filter, "influence")
		f.Industry = strArg(filter, "industry")
		if since, ok := filter["since"].(string); ok && since != "" {
			if ts, err := parseJSDate(since); err == nil {
				f.Since = &ts
			}
		}
		if mr, ok := toFloatOK(filter["minRelevance"]); ok {
			f.MinRelevance = &mr
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
	if err := authz(ctx, auth.PermSignalsRead); err != nil {
		return nil, err
	}
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

// sourceFilter reads the source list/count filter args.
func sourceFilter(args map[string]any) db.SourceFilter {
	f := db.SourceFilter{Limit: toInt(args["limit"], 100), Offset: toInt(args["offset"], 0)}
	f.Search = strArg(args, "search")
	f.Country = strArg(args, "country")
	f.Region = strArg(args, "region")
	f.Language = strArg(args, "language")
	f.Scope = strArg(args, "scope")
	f.Industry = strArg(args, "industry")
	f.OrgType = strArg(args, "orgType")
	f.SourceType = strArg(args, "sourceType")
	f.ValidationStatus = strArg(args, "validationStatus")
	f.Tag = strArg(args, "tag")
	f.PollStatus = strArg(args, "pollStatus")
	if v, ok := args["enabled"].(bool); ok {
		f.Enabled = &v
	}
	return f
}

func (s *Server) resolveSources(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSourcesRead); err != nil {
		return nil, err
	}
	rows, _, err := s.DB.ListSourcesFiltered(ctx, sourceFilter(args))
	if err != nil {
		return nil, err
	}
	out := make([]any, len(rows))
	for i, src := range rows {
		out[i] = sourceToGqlMap(src)
	}
	return out, nil
}

func (s *Server) resolveSourceCount(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSourcesRead); err != nil {
		return nil, err
	}
	return s.DB.CountSources(ctx, sourceFilter(args))
}

func (s *Server) resolveSourceCoverage(ctx context.Context, _ map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSourcesRead); err != nil {
		return nil, err
	}
	cov, err := s.DB.SourceCoverage(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	for k, bs := range cov {
		out[k] = buckets(bs)
	}
	return out, nil
}

func (s *Server) resolveSubscriptions(ctx context.Context, _ map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSubscriptionsRead); err != nil {
		return nil, err
	}
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
	if err := authz(ctx, auth.PermAnalyticsRead); err != nil {
		return nil, err
	}
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

// toInt coerces a GraphQL arg (int literal → int, JSON variable → float64) to int.
func toInt(v any, def int) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	}
	return def
}

// toFloatOK coerces a GraphQL arg (int literal → int, JSON variable → float64) to float64.
func toFloatOK(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	}
	return 0, false
}

func jsonUnmarshal(b []byte, v any) error {
	return jsonx.Unmarshal(b, v)
}
