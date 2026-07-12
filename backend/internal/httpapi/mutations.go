package httpapi

import (
	"context"
	"fmt"

	"github.com/worldsignal/backend/internal/auth"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/gql"
	"github.com/worldsignal/backend/internal/jsonx"
)

// sourceToGqlMap projects a Source onto the GraphQL Source type fields.
func sourceToGqlMap(src *db.Source) map[string]any {
	return map[string]any{
		"id": src.ID, "name": src.Name, "type": src.Type, "url": src.URL,
		"country": src.Country, "priority": src.Priority, "credibility": src.Credibility,
		"enabled": src.Enabled, "lastSuccessAt": timePtr(src.LastSuccessAt),
		"lastFailureAt": timePtr(src.LastFailureAt), "failureCount": src.FailureCount,
		// Rich metadata surfaced in the list view.
		"region": src.Region, "language": src.Language, "languages": strList(src.Languages),
		"geographicScope": src.GeographicScope, "industry": src.Industry,
		"category": src.Category, "publisher": src.Publisher, "orgType": src.OrgType,
		"sourceType": src.SourceType, "officialFeed": src.OfficialFeed,
		"healthScore": intPtr(src.HealthScore), "validationStatus": src.ValidationStatus,
		"tags": strList(src.Tags), "lastValidatedAt": timePtr(src.LastValidatedAt),
		"cooldownUntil": timePtr(src.CooldownUntil),
	}
}

// strList renders a (possibly nil) []string as a non-nil []any for GraphQL.
func strList(in []string) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}

// intPtr renders a *int as a value or nil.
func intPtr(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func (s *Server) mutationResolvers() map[string]gql.FieldResolver {
	return map[string]gql.FieldResolver{
		"createSource":       s.mutCreateSource,
		"setSourceEnabled":   s.mutSetSourceEnabled,
		"triggerFetch":       s.mutTriggerFetch,
		"createSubscription": s.mutCreateSubscription,
	}
}

func (s *Server) mutCreateSource(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSourcesWrite); err != nil {
		return nil, err
	}
	input, _ := args["input"].(map[string]any)
	if input == nil {
		return nil, fmt.Errorf("input required")
	}
	in := db.CreateSourceInput{Type: "RSS", Priority: 5, CrawlFrequency: 900, Credibility: 0.5}
	if v, ok := input["name"].(string); ok {
		in.Name = v
	}
	if v, ok := input["url"].(string); ok {
		in.URL = v
	}
	if v, ok := input["type"].(string); ok {
		in.Type = v
	}
	if v, ok := input["country"].(string); ok {
		c := v
		in.Country = &c
	}
	if v, ok := toFloatOK(input["priority"]); ok {
		in.Priority = int(v)
	}
	if v, ok := toFloatOK(input["crawlFrequency"]); ok {
		in.CrawlFrequency = int(v)
	}
	if v, ok := toFloatOK(input["credibility"]); ok {
		in.Credibility = v
	}
	// GraphQL createSource does NOT enqueue a fetch (only the REST route does).
	src, err := s.DB.CreateSource(ctx, in)
	if err != nil {
		return nil, err
	}
	s.audit(ctx, "SOURCE_CREATED", "source", src.ID, map[string]any{"name": src.Name, "url": src.URL})
	return sourceToGqlMap(src), nil
}

func (s *Server) mutSetSourceEnabled(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSourcesWrite); err != nil {
		return nil, err
	}
	id, _ := args["id"].(string)
	enabled, _ := args["enabled"].(bool)
	src, err := s.DB.SetSourceEnabled(ctx, id, enabled)
	if err != nil {
		return nil, err
	}
	return sourceToGqlMap(src), nil
}

func (s *Server) mutTriggerFetch(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSourcesWrite); err != nil {
		return nil, err
	}
	id, _ := args["id"].(string)
	if err := s.Enqueue.EnqueueFetchSource(id); err != nil {
		return nil, err
	}
	return true, nil
}

func (s *Server) mutCreateSubscription(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSubscriptionsWrite); err != nil {
		return nil, err
	}
	input, _ := args["input"].(map[string]any)
	if input == nil {
		return nil, fmt.Errorf("input required")
	}
	in := db.CreateSubscriptionInput{}
	if v, ok := input["name"].(string); ok {
		in.Name = v
	}
	if v, ok := input["channel"].(string); ok {
		in.Channel = v
	}
	if v, ok := input["filter"]; ok && v != nil {
		in.Filter = jsonRaw(v)
	}
	if v, ok := input["config"]; ok && v != nil {
		in.Config = jsonRaw(v)
	}
	sub, err := s.DB.CreateSubscription(ctx, in)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id": sub.ID, "name": sub.Name, "channel": sub.Channel, "enabled": sub.Enabled,
		"filter": sub.Filter, "config": sub.Config, "createdAt": sub.CreatedAt,
	}, nil
}

// jsonRaw marshals an arbitrary GraphQL JSON value to raw bytes for storage.
func jsonRaw(v any) db.RawJSON {
	b, err := jsonx.Marshal(v)
	if err != nil {
		return nil
	}
	return db.RawJSON(b)
}
