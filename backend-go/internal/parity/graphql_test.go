package parity_test

import (
	"testing"

	"github.com/worldsignal/backend/internal/jsonx"
	"github.com/worldsignal/backend/internal/parity"
)

func varsJSON(t *testing.T, variables map[string]any) string {
	t.Helper()
	if variables == nil {
		return ""
	}
	b, err := jsonx.Marshal(variables)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func compareGraphQL(t *testing.T, ts, go_ *parity.Server, query string, variables map[string]any) {
	t.Helper()
	vj := varsJSON(t, variables)
	tr, err := ts.GetGraphQL(query, vj)
	if err != nil {
		t.Fatalf("TS GraphQL: %v", err)
	}
	gr, err := go_.GetGraphQL(query, vj)
	if err != nil {
		t.Fatalf("Go GraphQL: %v", err)
	}
	if tr.Status != gr.Status {
		t.Fatalf("status mismatch TS=%d Go=%d\nTS: %s\nGo: %s", tr.Status, gr.Status, tr.Body, gr.Body)
	}
	if d := parity.DiffBytes(tr.Body, gr.Body); d != "" {
		t.Fatalf("GraphQL body not byte-identical:\nquery: %s\n%s\nTS: %s\nGo: %s", query, d, tr.Body, gr.Body)
	}
}

const signalFields = `id title summary whatHappened whyItMatters status severity confidence eventType country sourceCount firstSeenAt lastSeenAt tags { code confidence } sources { publisher url publishedAt }`

func TestGraphQLReadParity(t *testing.T) {
	if testing.Short() {
		t.Skip("skips server boot in -short mode")
	}
	_, ts, go_ := seededTS(t, 45820)

	cases := []struct {
		name  string
		query string
		vars  map[string]any
	}{
		{"stats", `{ stats }`, nil},
		{"taxonomy", `{ taxonomy }`, nil},
		{"sources", `{ sources { id name type url country priority credibility enabled lastSuccessAt lastFailureAt failureCount } }`, nil},
		{"subscriptions", `{ subscriptions { id name channel enabled filter config createdAt } }`, nil},
		{"signals_all", `query($f: SignalFilter, $l: Int){ signals(filter:$f, limit:$l){ ` + signalFields + ` } }`, map[string]any{"f": map[string]any{}, "l": 50}},
		{"signals_default_limit", `{ signals { ` + signalFields + ` } }`, nil},
		{"signals_country", `query($f: SignalFilter){ signals(filter:$f){ ` + signalFields + ` } }`, map[string]any{"f": map[string]any{"country": "US"}}},
		{"signals_minconf", `query($f: SignalFilter){ signals(filter:$f){ ` + signalFields + ` } }`, map[string]any{"f": map[string]any{"minConfidence": 0.5}}},
		{"signals_search", `query($f: SignalFilter){ signals(filter:$f){ ` + signalFields + ` } }`, map[string]any{"f": map[string]any{"search": "earthquake"}}},
		{"signals_tags", `query($f: SignalFilter){ signals(filter:$f){ ` + signalFields + ` } }`, map[string]any{"f": map[string]any{"tags": []any{"DISASTER.EARTHQUAKE", "ECONOMY.MARKETS"}}}},
		{"signals_status", `query($f: SignalFilter){ signals(filter:$f){ ` + signalFields + ` } }`, map[string]any{"f": map[string]any{"status": "CONFIRMED"}}},
		{"signals_limit_offset", `query($l:Int,$o:Int){ signals(limit:$l, offset:$o){ id title } }`, map[string]any{"l": 1, "o": 1}},
		{"signal_by_id", `query($id: ID!){ signal(id:$id){ ` + signalFields + ` } }`, map[string]any{"id": "sig_1"}},
		{"signal_missing", `query($id: ID!){ signal(id:$id){ id title } }`, map[string]any{"id": "nope"}},
		// Field aliases WITHIN an object preserve selection order in both backends.
		// (Multiple top-level fields are ordered by promise completion in graphql-js
		// and are non-deterministic, but every real query has one top-level field.)
		{"nested_aliases", `{ sources { n: name t: type c: country } }`, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			compareGraphQL(t, ts, go_, c.query, c.vars)
		})
	}
}
