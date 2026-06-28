package parity_test

import (
	"testing"

	"github.com/worldsignal/backend/internal/dbtest"
	"github.com/worldsignal/backend/internal/jsonx"
	"github.com/worldsignal/backend/internal/parity"
	"github.com/worldsignal/backend/internal/taxonomy"
)

// TestTS_TaxonomyByteParity boots the real TypeScript server and verifies that
// GET /v1/taxonomy is byte-identical to the Go taxonomy serialization. This is an
// end-to-end check of the harness AND of taxonomy parity through the actual HTTP
// stack, before the Go server exists.
func TestTS_TaxonomyByteParity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping server-boot parity test in -short mode")
	}
	// Ensure the test DB is reachable; the TS server needs a valid DATABASE_URL.
	dbtest.Connect(t)

	ts, err := parity.StartTS(45801, dbtest.URL())
	if err != nil {
		t.Fatalf("start TS server: %v", err)
	}
	defer ts.Stop()

	health, err := ts.Get("/health")
	if err != nil {
		t.Fatal(err)
	}
	if health.Status != 200 {
		t.Fatalf("health status %d", health.Status)
	}

	resp, err := ts.Get("/v1/taxonomy")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != 200 {
		t.Fatalf("taxonomy status %d: %s", resp.Status, resp.Body)
	}

	want, _ := jsonx.Marshal(taxonomy.Taxonomy)
	if d := parity.DiffBytes(want, resp.Body); d != "" {
		t.Fatalf("taxonomy not byte-identical to TS:\n%s", d)
	}
}
