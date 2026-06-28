package httpapi

import (
	"context"

	"github.com/worldsignal/backend/internal/auth"
	"github.com/worldsignal/backend/internal/countries"
)

// resolveCountries returns the ISO 3166-1 reference list (name, flag, currency,
// capital + capital geolocation) for country dropdowns. Available to any
// authenticated user.
func (s *Server) resolveCountries(ctx context.Context, _ map[string]any) (any, error) {
	if _, err := auth.Require(ctx); err != nil {
		return nil, err
	}
	all := countries.All()
	out := make([]any, len(all))
	for i, c := range all {
		out[i] = map[string]any{
			"code": c.Code, "name": c.Name, "flag": c.Flag,
			"currency": c.Currency, "capital": c.Capital,
			"capitalLat": c.CapitalLat, "capitalLng": c.CapitalLng,
		}
	}
	return out, nil
}
