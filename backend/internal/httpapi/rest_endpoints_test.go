package httpapi_test

import (
	"context"
	"testing"

	"github.com/worldsignal/backend/internal/dbtest"
)

// TestRESTFilteringAndErrors exercises the authenticated /v1 REST surface that
// PR #24 gated behind scoped API keys. It covers two things the existing tests
// skip: (1) the optional signal filter parameters on the happy path, and (2) the
// DB-error branches of the list/get handlers — forced by renaming the backing
// table so the query fails, the same technique TestAPIKeyDBErrors uses.
func TestRESTFilteringAndErrors(t *testing.T) {
	d := dbtest.Connect(t)
	seed(t, d) // newServer's seedFullAPIKey provisions an all-scopes key for get()
	ht, _ := newServer(t, d)
	ctx := context.Background()

	// Every optional filter is parsed. region/geoScope/sentiment/influence map to
	// plain column equality and minRelevance to a float comparison, so unknown
	// values simply match nothing — the request still succeeds with 200.
	if code, body := get(t, ht.URL,
		"/v1/signals?region=NorthAmerica&geoScope=country&sentiment=neutral&influence=high&minRelevance=0.3&industry=tech&entity=Acme"); code != 200 {
		t.Fatalf("filtered /v1/signals want 200 got %d %s", code, body)
	}

	rename := func(from, to string) {
		if _, err := d.Pool.Exec(ctx, `ALTER TABLE "`+from+`" RENAME TO "`+to+`"`); err != nil {
			t.Fatalf("rename %s->%s: %v", from, to, err)
		}
	}
	// withTableGone renames a table away, runs fn, then always renames it back so
	// later assertions (and other tests sharing the DB) still see a valid schema.
	withTableGone := func(table string, fn func()) {
		hidden := table + "__hidden"
		rename(table, hidden)
		defer rename(hidden, table)
		fn()
	}

	// listSignals + getSignal surface a 500 when their query fails.
	withTableGone("Signal", func() {
		if code, _ := get(t, ht.URL, "/v1/signals"); code != 500 {
			t.Fatalf("listSignals DB error want 500 got %d", code)
		}
		if code, _ := get(t, ht.URL, "/v1/signals/sg"); code != 500 {
			t.Fatalf("getSignal DB error want 500 got %d", code)
		}
	})
	// listSources.
	withTableGone("Source", func() {
		if code, _ := get(t, ht.URL, "/v1/sources"); code != 500 {
			t.Fatalf("listSources DB error want 500 got %d", code)
		}
	})
	// listSubscriptions.
	withTableGone("Subscription", func() {
		if code, _ := get(t, ht.URL, "/v1/subscriptions"); code != 500 {
			t.Fatalf("listSubscriptions DB error want 500 got %d", code)
		}
	})
	// listDeliveries.
	withTableGone("DeliveryEvent", func() {
		if code, _ := get(t, ht.URL, "/v1/deliveries"); code != 500 {
			t.Fatalf("listDeliveries DB error want 500 got %d", code)
		}
	})
}
