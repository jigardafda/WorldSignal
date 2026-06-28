package httpapi

import (
	"context"
	"testing"

	"github.com/worldsignal/backend/internal/auth"
)

func TestResolveCountries(t *testing.T) {
	s := &Server{}
	ctx := auth.WithIdentity(context.Background(), &auth.Identity{UserID: "u", Role: auth.RoleViewer})
	res, err := s.resolveCountries(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	list := res.([]any)
	if len(list) < 180 {
		t.Fatalf("expected full country list, got %d", len(list))
	}
	first := list[0].(map[string]any)
	for _, k := range []string{"code", "name", "flag", "currency", "capital", "capitalLat", "capitalLng"} {
		if _, ok := first[k]; !ok {
			t.Fatalf("country map missing %q: %+v", k, first)
		}
	}
	// Unauthenticated is rejected.
	if _, err := s.resolveCountries(context.Background(), nil); err == nil {
		t.Fatal("unauthenticated should be rejected")
	}
}
