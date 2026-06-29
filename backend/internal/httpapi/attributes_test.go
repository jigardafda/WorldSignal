package httpapi_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
)

// seedEnrichedSignal seeds a signal and applies a deep enrichment with geo +
// sentiment + industry/category/entity attributes.
func seedEnrichedSignal(t *testing.T, d *db.DB) {
	t.Helper()
	dbtest.Reset(t, d)
	dbtest.SeedTaxonomy(t, d)
	ctx := context.Background()
	ex := func(q string, a ...any) {
		if _, err := d.Pool.Exec(ctx, q, a...); err != nil {
			t.Fatal(err)
		}
	}
	ex(`INSERT INTO "Source" ("id","name","url","updatedAt") VALUES ('s1','S','https://s.example/feed',now())`)
	ex(`INSERT INTO "Signal" ("id","title","summary","status","severity","confidence","country","sourceCount","firstSeenAt","lastSeenAt","updatedAt") VALUES ('sg','T','S','CONFIRMED','HIGH',0.8,'US',1,now(),now(),now())`)
	ex(`INSERT INTO "Article" ("id","sourceId","canonicalUrl","title","publishedAt") VALUES ('a1','s1','https://s.example/a','A',now())`)
	ex(`INSERT INTO "SignalArticle" ("signalId","articleId","relationType","similarityScore") VALUES ('sg','a1','PRIMARY',1)`)

	region, scope, sent, infl, lang := "California", "LOCAL", "NEGATIVE", "HIGH", "fr"
	origT, origS := "Séisme", "Un séisme a frappé."
	score, rel := -0.6, 0.9
	if err := d.ApplyEnrichment(ctx, "sg", db.EnrichmentUpdate{
		Title: "T", Summary: "S", Severity: "HIGH", Confidence: 0.8, Status: "CONFIRMED",
		PublishedAt: time.Now(), Metadata: map[string]any{},
		Region: &region, GeoScope: &scope, Sentiment: &sent, SentimentScore: &score,
		Influence: &infl, Relevance: &rel, Language: &lang,
		OriginalTitle: &origT, OriginalSummary: &origS,
		Attributes: []db.SignalAttr{
			{Key: "industry", ValueCode: "CYBERSECURITY", Confidence: 1},
			{Key: "category", ValueCode: "DISASTER.EARTHQUAKE", Confidence: 0.9},
			{Key: "entity", ValueCode: "ORGANIZATION", ValueText: "Acme", Confidence: 0.8},
		},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestGraphQLSignalAttributes(t *testing.T) {
	d := dbtest.Connect(t)
	seedEnrichedSignal(t, d)
	ht, _ := newServer(t, d)
	tok, _ := dbtest.AuthToken(t, d, "VIEWER")
	bearer = tok
	defer func() { bearer = "" }()

	_, body := postGQL(t, ht.URL, `{"query":"{ signals { id geoScope sentiment sentimentScore influence relevance region language translated originalTitle originalSummary attributes { key valueCode valueText } } }"}`)
	for _, want := range []string{`"geoScope":"LOCAL"`, `"sentiment":"NEGATIVE"`, `"influence":"HIGH"`, `"region":"California"`, `"language":"fr"`, `"translated":true`, `"originalTitle":"Séisme"`, `"valueCode":"CYBERSECURITY"`, `"valueText":"Acme"`} {
		if !strings.Contains(body, want) {
			t.Errorf("signals response missing %s: %s", want, body)
		}
	}
}

func TestGraphQLAttributeDictionary(t *testing.T) {
	d := dbtest.Connect(t)
	seedEnrichedSignal(t, d)
	ht, _ := newServer(t, d)
	tok, _ := dbtest.AuthToken(t, d, "VIEWER")
	bearer = tok
	defer func() { bearer = "" }()

	_, body := postGQL(t, ht.URL, `{"query":"{ attributeDictionary { key kind values { code label } } }"}`)
	for _, want := range []string{`"key":"sentiment"`, `"key":"industry"`, `"kind":"ENUM"`, `"code":"GLOBAL"`, `"code":"CYBERSECURITY"`} {
		if !strings.Contains(body, want) {
			t.Errorf("dictionary missing %s: %s", want, body)
		}
	}
}

func TestGraphQLSignalAttributeFilters(t *testing.T) {
	d := dbtest.Connect(t)
	seedEnrichedSignal(t, d)
	ht, _ := newServer(t, d)
	tok, _ := dbtest.AuthToken(t, d, "VIEWER")
	bearer = tok
	defer func() { bearer = "" }()

	// Matching filters return the signal.
	for _, q := range []string{
		`{ signals(filter:{sentiment:\"NEGATIVE\"}){ id } }`,
		`{ signals(filter:{geoScope:\"LOCAL\"}){ id } }`,
		`{ signals(filter:{industry:\"CYBERSECURITY\"}){ id } }`,
		`{ signals(filter:{influence:\"HIGH\"}){ id } }`,
		`{ signals(filter:{minRelevance:0.5}){ id } }`,
		`{ signals(filter:{region:\"California\"}){ id } }`,
	} {
		_, body := postGQL(t, ht.URL, `{"query":"`+q+`"}`)
		if !strings.Contains(body, `"id":"sg"`) {
			t.Errorf("filter %q should match: %s", q, body)
		}
	}
	// Non-matching filters exclude it.
	for _, q := range []string{
		`{ signals(filter:{sentiment:\"POSITIVE\"}){ id } }`,
		`{ signals(filter:{industry:\"BANKING\"}){ id } }`,
		`{ signals(filter:{minRelevance:0.95}){ id } }`,
	} {
		_, body := postGQL(t, ht.URL, `{"query":"`+q+`"}`)
		if strings.Contains(body, `"id":"sg"`) {
			t.Errorf("filter %q should NOT match: %s", q, body)
		}
	}
}

func TestGraphQLSignalByID(t *testing.T) {
	d := dbtest.Connect(t)
	seedEnrichedSignal(t, d)
	ht, _ := newServer(t, d)
	tok, _ := dbtest.AuthToken(t, d, "VIEWER")
	bearer = tok
	defer func() { bearer = "" }()

	_, found := postGQL(t, ht.URL, `{"query":"{ signal(id:\"sg\"){ id geoScope attributes { valueCode } } }"}`)
	if !strings.Contains(found, `"geoScope":"LOCAL"`) || !strings.Contains(found, `"valueCode":"CYBERSECURITY"`) {
		t.Errorf("signal(id) should return attributes: %s", found)
	}
	_, missing := postGQL(t, ht.URL, `{"query":"{ signal(id:\"nope\"){ id } }"}`)
	if !strings.Contains(missing, `"signal":null`) {
		t.Errorf("missing signal should be null: %s", missing)
	}
}

func TestGraphQLSignalsSinceFilter(t *testing.T) {
	d := dbtest.Connect(t)
	seedEnrichedSignal(t, d)
	ht, _ := newServer(t, d)
	tok, _ := dbtest.AuthToken(t, d, "VIEWER")
	bearer = tok
	defer func() { bearer = "" }()

	// A window starting in the past includes the (now-stamped) signal.
	_, recent := postGQL(t, ht.URL, `{"query":"{ signals(filter:{since:\"2020-01-01T00:00:00Z\"}){ id } }"}`)
	if !strings.Contains(recent, `"id":"sg"`) {
		t.Errorf("past since should include the signal: %s", recent)
	}
	// A window starting in the future excludes it.
	_, future := postGQL(t, ht.URL, `{"query":"{ signals(filter:{since:\"2999-01-01T00:00:00Z\"}){ id } }"}`)
	if strings.Contains(future, `"id":"sg"`) {
		t.Errorf("future since should exclude the signal: %s", future)
	}
}

func TestAttributeDictionaryRequiresAuth(t *testing.T) {
	d := dbtest.Connect(t)
	seedEnrichedSignal(t, d)
	ht, _ := newServer(t, d)
	bearer = ""
	_, body := postGQL(t, ht.URL, `{"query":"{ attributeDictionary { key } }"}`)
	if !strings.Contains(body, "error") {
		t.Errorf("unauthenticated dictionary query should error: %s", body)
	}
}

func TestRESTSignalAttributes(t *testing.T) {
	d := dbtest.Connect(t)
	seedEnrichedSignal(t, d)
	ht, _ := newServer(t, d)

	_, body := get(t, ht.URL, "/v1/signals/sg")
	for _, want := range []string{`"geoScope":"LOCAL"`, `"sentiment":"NEGATIVE"`, `"relevance":0.9`, `"language":"fr"`, `"translated":true`, `"valueCode":"CYBERSECURITY"`} {
		if !strings.Contains(body, want) {
			t.Errorf("REST signal missing %s: %s", want, body)
		}
	}
	// REST list filter by industry.
	_, list := get(t, ht.URL, "/v1/signals?industry=CYBERSECURITY")
	if !strings.Contains(list, `"id":"sg"`) {
		t.Errorf("REST industry filter should match: %s", list)
	}
	_, none := get(t, ht.URL, "/v1/signals?sentiment=POSITIVE")
	if strings.Contains(none, `"id":"sg"`) {
		t.Errorf("REST sentiment filter should exclude: %s", none)
	}
}
