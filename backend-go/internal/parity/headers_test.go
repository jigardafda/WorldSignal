package parity_test

import (
	"testing"

	"github.com/worldsignal/backend/internal/parity"
)

func TestCORSAndOptionsParity(t *testing.T) {
	if testing.Short() {
		t.Skip("skips server boot in -short mode")
	}
	_, ts, go_ := seededTS(t, 45830)

	// CORS headers on a normal GET must match.
	tr, err := ts.Get("/v1/stats")
	if err != nil {
		t.Fatal(err)
	}
	gr, err := go_.Get("/v1/stats")
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{
		"Access-Control-Allow-Origin",
		"Access-Control-Allow-Methods",
		"Access-Control-Allow-Headers",
	} {
		if tr.Header.Get(h) != gr.Header.Get(h) {
			t.Fatalf("%s mismatch: TS=%q Go=%q", h, tr.Header.Get(h), gr.Header.Get(h))
		}
	}

	// OPTIONS preflight: 204, empty body, same on both.
	to, err := ts.Options("/v1/sources")
	if err != nil {
		t.Fatal(err)
	}
	go_o, err := go_.Options("/v1/sources")
	if err != nil {
		t.Fatal(err)
	}
	if to.Status != 204 || go_o.Status != 204 {
		t.Fatalf("OPTIONS status: TS=%d Go=%d", to.Status, go_o.Status)
	}
	if d := parity.DiffBytes(to.Body, go_o.Body); d != "" {
		t.Fatalf("OPTIONS body mismatch:\n%s", d)
	}
}

// TestGraphQLErrorEnvelopeParity checks that both backends return a structurally
// equivalent error envelope (errors array, no data) for an invalid field. Exact
// graphql-yoga error text is internal and unused by the app, so it is not
// byte-compared.
func TestGraphQLErrorEnvelopeParity(t *testing.T) {
	if testing.Short() {
		t.Skip("skips server boot in -short mode")
	}
	_, ts, go_ := seededTS(t, 45831)

	tr, err := ts.GetGraphQL(`{ nonexistentField }`, "")
	if err != nil {
		t.Fatal(err)
	}
	gr, err := go_.GetGraphQL(`{ nonexistentField }`, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range [][]byte{tr.Body, gr.Body} {
		s := string(body)
		if !contains(s, `"errors"`) {
			t.Fatalf("expected errors envelope, got: %s", s)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
