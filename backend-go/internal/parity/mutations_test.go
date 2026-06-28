package parity_test

import (
	"testing"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
	"github.com/worldsignal/backend/internal/parity"
)

// mutationCase issues the same write to TS then Go, each on a freshly
// reset+seeded DB, and compares status + normalized (volatile-blanked) response.
type mutationCase struct {
	name   string
	method string
	path   string
	body   string
}

func runMutationParity(t *testing.T, d *db.DB, ts, go_ *parity.Server, c mutationCase) {
	t.Helper()
	do := func(srv *parity.Server) *parity.Response {
		dbtest.Reset(t, d)
		dbtest.SeedTaxonomy(t, d)
		insertFixtures(t, d)
		var resp *parity.Response
		var err error
		switch c.method {
		case "POST":
			if c.body == "" {
				resp, err = srv.Post(c.path) // bodyless action endpoint
			} else {
				resp, err = srv.PostJSON(c.path, []byte(c.body))
			}
		case "PATCH":
			resp, err = srv.PatchJSON(c.path, []byte(c.body))
		}
		if err != nil {
			t.Fatalf("%s %s: %v", c.method, c.path, err)
		}
		return resp
	}
	tr := do(ts)
	gr := do(go_)
	if tr.Status != gr.Status {
		t.Fatalf("%s: status TS=%d Go=%d\nTS: %s\nGo: %s", c.name, tr.Status, gr.Status, tr.Body, gr.Body)
	}
	tn := normalizeJSON(t, tr.Body)
	gn := normalizeJSON(t, gr.Body)
	if tn != gn {
		t.Fatalf("%s: normalized response mismatch\nTS: %s\nGo: %s", c.name, tn, gn)
	}
}

func TestRESTMutationRowParity(t *testing.T) {
	if testing.Short() {
		t.Skip("skips server boot in -short mode")
	}
	d := dbtest.Connect(t)
	ts, err := parity.StartTS(45840, dbtest.URL())
	if err != nil {
		t.Fatalf("start TS: %v", err)
	}
	t.Cleanup(ts.Stop)
	go_ := goServer(t, d)

	cases := []mutationCase{
		{"create_source_minimal", "POST", "/v1/sources", `{"name":"New Feed","url":"https://new.example/feed"}`},
		{"create_source_full", "POST", "/v1/sources", `{"name":"Full Feed","url":"https://full.example/feed","type":"ATOM","country":"IN","priority":0,"crawlFrequency":120,"credibility":0.99}`},
		{"create_source_missing_url", "POST", "/v1/sources", `{"name":"No URL"}`},
		{"create_source_missing_name", "POST", "/v1/sources", `{"url":"https://x.example/feed"}`},
		{"create_source_duplicate", "POST", "/v1/sources", `{"name":"Dup","url":"https://alpha.example/feed"}`},
		{"patch_source_enabled", "PATCH", "/v1/sources/src_alpha", `{"enabled":false}`},
		{"patch_source_priority", "PATCH", "/v1/sources/src_zeta", `{"priority":4,"crawlFrequency":600}`},
		{"fetch_source", "POST", "/v1/sources/src_alpha/fetch", ``},
		{"create_subscription_minimal", "POST", "/v1/subscriptions", `{"name":"My Sub"}`},
		{"create_subscription_full", "POST", "/v1/subscriptions", `{"name":"Hook Sub","channel":"WEBHOOK","filter":{"tags":["DISASTER"],"minConfidence":0.5},"config":{"url":"https://h.example/in"}}`},
		{"create_subscription_missing_name", "POST", "/v1/subscriptions", `{"channel":"POLLING"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			runMutationParity(t, d, ts, go_, c)
		})
	}
}
