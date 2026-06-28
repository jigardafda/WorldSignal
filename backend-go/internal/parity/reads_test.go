package parity_test

import (
	"testing"

	"github.com/worldsignal/backend/internal/parity"
)

// compareGet asserts that a GET to the same path returns identical status and
// byte-identical bodies from both backends.
func compareGet(t *testing.T, ts, go_ *parity.Server, path string) {
	t.Helper()
	tr, err := ts.Get(path)
	if err != nil {
		t.Fatalf("TS GET %s: %v", path, err)
	}
	gr, err := go_.Get(path)
	if err != nil {
		t.Fatalf("Go GET %s: %v", path, err)
	}
	if tr.Status != gr.Status {
		t.Fatalf("%s status mismatch: TS=%d Go=%d\nTS body: %s\nGo body: %s", path, tr.Status, gr.Status, tr.Body, gr.Body)
	}
	if d := parity.DiffBytes(tr.Body, gr.Body); d != "" {
		t.Fatalf("%s body not byte-identical:\n%s", path, d)
	}
}

func TestReadParity_SimpleEndpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("skips server boot in -short mode")
	}
	_, ts, go_ := seededTS(t, 45810)

	for _, path := range []string{
		"/health",
		"/v1/stats",
		"/v1/taxonomy",
		"/v1/sources",
		"/v1/signals",
		"/v1/signals?country=US",
		"/v1/signals?status=CONFIRMED",
		"/v1/signals?minConfidence=0.5",
		"/v1/signals?since=2026-01-02T02:00:00.000Z",
		"/v1/signals?search=earthquake",
		"/v1/signals?search=Markets",
		"/v1/signals?tags=DISASTER.EARTHQUAKE",
		"/v1/signals?tags=DISASTER.EARTHQUAKE,ECONOMY.MARKETS",
		"/v1/signals?limit=1",
		"/v1/signals?limit=1&offset=1",
		"/v1/signals/sig_1",
		"/v1/signals/sig_2",
		"/v1/signals/sig_3",
		"/v1/signals/does-not-exist",
		"/v1/subscriptions",
		"/v1/deliveries",
		"/v1/deliveries?limit=1",
	} {
		t.Run(path, func(t *testing.T) {
			compareGet(t, ts, go_, path)
		})
	}
}
