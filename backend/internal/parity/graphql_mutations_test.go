package parity_test

import (
	"context"
	"testing"

	"github.com/worldsignal/backend/internal/dbtest"
	"github.com/worldsignal/backend/internal/jsonx"
)

// gqlBody builds a GraphQL POST body from a query and raw JSON variables.
func gqlBody(query, variablesJSON string) []byte {
	q, _ := jsonx.Marshal(query)
	if variablesJSON == "" {
		variablesJSON = "null"
	}
	return []byte(`{"query":` + string(q) + `,"variables":` + variablesJSON + `}`)
}

// TestGraphQLMutationsMatchREST verifies, Go-internally, that each GraphQL
// mutation produces the same DB row as its REST counterpart. Combined with the
// REST row-parity vs TS and the fact that the TS GraphQL and REST mutations call
// the same Prisma create with the same defaults, this establishes GraphQL
// mutation parity even though the legacy TS /graphql POST is unreachable (it
// hangs on body parsing).
func TestGraphQLMutationsMatchREST(t *testing.T) {
	if testing.Short() {
		t.Skip("needs DB")
	}
	d := dbtest.Connect(t)
	go_ := goServer(t, d)
	ctx := context.Background()

	freshSeed := func() {
		dbtest.Reset(t, d)
		dbtest.SeedTaxonomy(t, d)
		insertFixtures(t, d)
	}
	sourceByURL := func(url string) string {
		srcs, err := d.ListSources(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range srcs {
			if s.URL == url {
				b, _ := jsonx.Marshal(s)
				return normalizeJSON(t, b)
			}
		}
		t.Fatalf("source %s not found", url)
		return ""
	}

	t.Run("createSource", func(t *testing.T) {
		const url = "https://gqlrest.example/feed"
		gqlQ := `mutation($i: CreateSourceInput!){ createSource(input:$i){ id } }`
		gvars := `{"i":{"name":"GR Feed","url":"` + url + `","type":"ATOM","country":"IN","priority":0,"crawlFrequency":120,"credibility":0.99}}`

		freshSeed()
		if _, err := go_.PostJSON("/graphql", gqlBody(gqlQ, gvars)); err != nil {
			t.Fatal(err)
		}
		viaGQL := sourceByURL(url)

		freshSeed()
		if _, err := go_.PostJSON("/v1/sources", []byte(`{"name":"GR Feed","url":"`+url+`","type":"ATOM","country":"IN","priority":0,"crawlFrequency":120,"credibility":0.99}`)); err != nil {
			t.Fatal(err)
		}
		viaREST := sourceByURL(url)

		if viaGQL != viaREST {
			t.Fatalf("createSource row differs:\nGQL:  %s\nREST: %s", viaGQL, viaREST)
		}
	})

	t.Run("setSourceEnabled_vs_patch", func(t *testing.T) {
		freshSeed()
		if _, err := go_.PostJSON("/graphql", gqlBody(`mutation($id:ID!){ setSourceEnabled(id:$id, enabled:false){ id } }`, `{"id":"src_alpha"}`)); err != nil {
			t.Fatal(err)
		}
		viaGQL := sourceByURL("https://alpha.example/feed")

		freshSeed()
		if _, err := go_.PatchJSON("/v1/sources/src_alpha", []byte(`{"enabled":false}`)); err != nil {
			t.Fatal(err)
		}
		viaREST := sourceByURL("https://alpha.example/feed")

		if viaGQL != viaREST {
			t.Fatalf("setSourceEnabled vs patch differs:\nGQL:  %s\nREST: %s", viaGQL, viaREST)
		}
	})

	t.Run("createSubscription", func(t *testing.T) {
		subByName := func(name string) string {
			subs, err := d.ListSubscriptionsBasic(ctx)
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range subs {
				if s.Name == name {
					b, _ := jsonx.Marshal(map[string]any{
						"subscriberId": s.SubscriberID, "name": s.Name, "channel": s.Channel,
						"filter": s.Filter, "config": s.Config, "enabled": s.Enabled,
					})
					return normalizeJSON(t, b)
				}
			}
			t.Fatalf("subscription %s not found", name)
			return ""
		}

		freshSeed()
		if _, err := go_.PostJSON("/graphql", gqlBody(`mutation($i: CreateSubscriptionInput!){ createSubscription(input:$i){ id } }`, `{"i":{"name":"X Sub","channel":"WEBHOOK","filter":{"tags":["DISASTER"]},"config":{"url":"https://h.example"}}}`)); err != nil {
			t.Fatal(err)
		}
		viaGQL := subByName("X Sub")

		freshSeed()
		if _, err := go_.PostJSON("/v1/subscriptions", []byte(`{"name":"X Sub","channel":"WEBHOOK","filter":{"tags":["DISASTER"]},"config":{"url":"https://h.example"}}`)); err != nil {
			t.Fatal(err)
		}
		viaREST := subByName("X Sub")

		if viaGQL != viaREST {
			t.Fatalf("createSubscription row differs:\nGQL:  %s\nREST: %s", viaGQL, viaREST)
		}
	})
}
