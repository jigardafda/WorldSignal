package httpapi

import (
	"context"
	"fmt"

	"github.com/worldsignal/backend/internal/auth"
	"github.com/worldsignal/backend/internal/gql"
	"github.com/worldsignal/backend/internal/llm"
)

// registerRelevanceResolvers adds the smart-signals feed to the GraphQL surface
// the admin panel uses: the ranked "For You" feed, interest editing, feedback,
// and AI draft-from-document. The public REST/streaming API exposes the same
// capabilities under API keys.
func (s *Server) registerRelevanceResolvers(q, m map[string]gql.FieldResolver) {
	q["subscriptionFeed"] = s.resolveSubscriptionFeed
	q["subscriptionInterests"] = s.resolveSubscriptionInterests
	m["setSubscriptionInterests"] = s.mutSetSubscriptionInterests
	m["recordSignalFeedback"] = s.mutRecordSignalFeedback
	m["draftProfileFromDocument"] = s.mutDraftProfileFromDocument
}

// authorizeSubscriptionRelevance authorizes a subscription-scoped relevance op
// on the GraphQL surface. A tenant (account-scoped identity) is authorized by
// OWNERSHIP — it may only touch its own subscriptions, and needs no operator
// permission to manage them — which both lets customers use their console and
// closes the cross-tenant IDOR. Platform staff are gated on staffPerm and may
// act on any subscription. This mirrors s.tenantOwnsSubscription on the REST API.
func (s *Server) authorizeSubscriptionRelevance(ctx context.Context, subID, staffPerm string) error {
	id, err := auth.Require(ctx)
	if err != nil {
		return err
	}
	if id.AccountID != nil {
		owner, err := s.DB.SubscriptionAccountID(ctx, subID)
		if err != nil {
			return err
		}
		if owner == "" || owner != *id.AccountID {
			return auth.ErrForbidden
		}
		return nil
	}
	return authz(ctx, staffPerm)
}

func (s *Server) resolveSubscriptionFeed(ctx context.Context, args map[string]any) (any, error) {
	if err := s.authorizeSubscriptionRelevance(ctx, strVal(args["id"]), auth.PermSignalsRead); err != nil {
		return nil, err
	}
	id := strVal(args["id"])
	limit := clampLimit(toInt(args["limit"], 30))
	sinceHours := toInt(args["sinceHours"], 72)
	minScore := toFloat(args["minScore"], 0)

	ranked, err := s.DB.RankedFeed(ctx, id, sinceHours, limit*3)
	if err != nil {
		return nil, err
	}
	out := make([]any, 0, limit)
	for _, sc := range ranked {
		if sc.Score < minScore {
			continue
		}
		reasons := make([]any, len(sc.Reasons))
		for i, r := range sc.Reasons {
			reasons[i] = r
		}
		out = append(out, map[string]any{
			"id": sc.ID, "title": sc.Title, "summary": sc.Summary, "eventType": sc.EventType,
			"country": sc.Country, "region": sc.Region, "sentiment": sc.Sentiment,
			"influence": sc.Influence, "severity": sc.Severity, "ageHours": round1(sc.AgeHours),
			"score": round1(sc.Score), "reasons": reasons,
		})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (s *Server) resolveSubscriptionInterests(ctx context.Context, args map[string]any) (any, error) {
	if err := s.authorizeSubscriptionRelevance(ctx, strVal(args["id"]), auth.PermSignalsRead); err != nil {
		return nil, err
	}
	p, err := s.DB.LoadProfile(ctx, strVal(args["id"]))
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	for k, v := range p.Interests {
		out[k] = v
	}
	return out, nil
}

func (s *Server) mutSetSubscriptionInterests(ctx context.Context, args map[string]any) (any, error) {
	if err := s.authorizeSubscriptionRelevance(ctx, strVal(args["id"]), auth.PermSubscriptionsWrite); err != nil {
		return nil, err
	}
	id := strVal(args["id"])
	interests := map[string]float64{}
	if raw, ok := args["interests"].(map[string]any); ok {
		for k, v := range raw {
			if f, ok := toNumber(v); ok {
				interests[k] = f
			}
		}
	}
	if err := s.DB.SetSubscriptionInterests(ctx, id, interests); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

func (s *Server) mutRecordSignalFeedback(ctx context.Context, args map[string]any) (any, error) {
	sub, sig, action := strVal(args["subscriptionId"]), strVal(args["signalId"]), strVal(args["action"])
	if err := s.authorizeSubscriptionRelevance(ctx, sub, auth.PermSubscriptionsWrite); err != nil {
		return nil, err
	}
	if sub == "" || sig == "" || !validFeedback[action] {
		return false, nil
	}
	if err := s.DB.RecordFeedback(ctx, sub, sig, action); err != nil {
		return nil, err
	}
	return true, nil
}

func (s *Server) mutDraftProfileFromDocument(ctx context.Context, args map[string]any) (any, error) {
	// Drafting a profile touches no stored subscription (pure LLM inference), so
	// any authenticated tenant may use it from the customer console; platform
	// staff are held to the operator write permission.
	id, err := auth.Require(ctx)
	if err != nil {
		return nil, err
	}
	if id.AccountID == nil {
		if err := authz(ctx, auth.PermSubscriptionsWrite); err != nil {
			return nil, err
		}
	}
	text := strVal(args["text"])
	if len(text) < 20 {
		return nil, fmt.Errorf("text (a document, at least 20 chars) required")
	}
	gw := llm.NewDynamicGateway(s.ResolveLLMKey)
	d := llm.DraftProfileFromDocument(ctx, gw, text)

	interests := make(map[string]any, len(d.Interests))
	for k, v := range d.Interests {
		interests[k] = v
	}
	reasons := make([]any, len(d.Reasons))
	for i, r := range d.Reasons {
		reasons[i] = map[string]any{"key": r.Key, "why": r.Why, "origin": r.Origin}
	}
	return map[string]any{
		"name": d.Name, "summary": d.Summary, "interests": interests,
		"minScore": d.MinScore, "minSeverity": d.MinSeverity,
		"reasons": reasons, "source": d.Source,
	}, nil
}

// toFloat/toNumber coerce GraphQL numeric args (which arrive as float64/int/json).
func toFloat(v any, def float64) float64 {
	if f, ok := toNumber(v); ok {
		return f
	}
	return def
}

func toNumber(v any) (float64, bool) {
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
