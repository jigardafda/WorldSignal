package httpapi

import (
	"context"
	"time"

	"github.com/worldsignal/backend/internal/auth"
	"github.com/worldsignal/backend/internal/cuid"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/sources"
)

// mutRevalidateSource re-fetches and re-validates a source's feed on demand,
// persisting the outcome (status, health score, validation log).
func (s *Server) mutRevalidateSource(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSourcesWrite); err != nil {
		return nil, err
	}
	id := strVal(args["id"])
	src, err := s.DB.GetSource(ctx, id)
	if err != nil || src == nil {
		return nil, err
	}

	v := sources.NewValidator(sources.DefaultConfig())
	vctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	r := v.ValidateCandidate(vctx, sources.Candidate{FeedURL: src.URL})

	out := db.ValidationOutcome{
		OK: r.OK, HTTPStatus: r.HTTPStatus, ResponseMs: r.ResponseMs,
		ItemCount: r.ItemCount, NewestItemAt: r.NewestItem, RedirectedTo: r.RedirectedTo,
		HealthScore: r.HealthScore, Error: r.Error,
	}
	if err := s.DB.RecordValidation(ctx, src.ID, cuid.New(), out); err != nil {
		return nil, err
	}

	updated, err := s.DB.GetSource(ctx, id)
	if err != nil || updated == nil {
		return nil, err
	}
	m := sourceDetailMap(updated)
	logs, err := s.DB.ListValidationLogs(ctx, id, 50)
	if err != nil {
		return nil, err
	}
	m["validationLogs"] = validationLogMaps(logs)
	return m, nil
}
