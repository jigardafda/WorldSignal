package pipeline

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/worldsignal/backend/internal/crawl"
	"github.com/worldsignal/backend/internal/dbtest"
	"github.com/worldsignal/backend/internal/llm"
)

// fakeCrawler returns canned page text, recording the URL it was asked to fetch.
type fakeCrawler struct {
	text       string
	lastURL    string
	shouldFail bool
}

func (f *fakeCrawler) Fetch(_ context.Context, url string) crawl.Result {
	f.lastURL = url
	if f.shouldFail {
		return crawl.Result{URL: url, Err: "boom"}
	}
	return crawl.Result{URL: url, Text: f.text, Chars: len(f.text)}
}

func TestEnrichSignalExtractsAttributes(t *testing.T) {
	d := conn(t)
	ctx := context.Background()
	dbtest.SeedTaxonomy(t, d)
	gw := llm.NewOpenAIGateway("", "") // heuristic path (deterministic)

	mustExec(t, d, `INSERT INTO "Source" ("id","name","url","credibility","updatedAt") VALUES ('s1','Primary','https://s1.example',0.9,now())`)
	mustExec(t, d, `INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","sourceCount","metadata","updatedAt") VALUES ('sg','old','old',now(),now(),1,'{"tokenSet":"x"}',now())`)
	mustExec(t, d, `INSERT INTO "Article" ("id","sourceId","title","body","canonicalUrl") VALUES ('a1','s1','Bank cyberattack','A ransomware cyberattack hit a major bank.','https://s1.example/story')`)
	mustExec(t, d, `INSERT INTO "SignalArticle" ("signalId","articleId","relationType","addedAt") VALUES ('sg','a1','PRIMARY',now())`)

	cr := &fakeCrawler{text: "Extended page: the banking sector and cybersecurity firms responded to the breach."}
	if err := EnrichSignal(ctx, d, gw, cr, "sg", time.Now()); err != nil {
		t.Fatal(err)
	}
	if cr.lastURL != "https://s1.example/story" {
		t.Fatalf("crawler should have been called with the canonical url, got %q", cr.lastURL)
	}

	// crawled flag + attribute source recorded in metadata.
	var crawled *bool
	mustScan(t, d, `SELECT (metadata->>'crawled')::bool FROM "Signal" WHERE id='sg'`, &crawled)
	if crawled == nil || !*crawled {
		t.Fatal("expected crawled=true in metadata")
	}

	// sentiment column populated (heuristic: severity cue 'cyberattack' -> NEGATIVE).
	var sentiment *string
	mustScan(t, d, `SELECT sentiment FROM "Signal" WHERE id='sg'`, &sentiment)
	if sentiment == nil || *sentiment != "NEGATIVE" {
		t.Fatalf("sentiment = %v", sentiment)
	}

	// industries + categories landed in SignalAttribute.
	attrs, err := d.SignalAttributes(ctx, "sg")
	if err != nil {
		t.Fatal(err)
	}
	hasIndustry, hasCategory := false, false
	for _, a := range attrs {
		if a.Key == "industry" && (a.ValueCode == "BANKING" || a.ValueCode == "CYBERSECURITY") {
			hasIndustry = true
		}
		if a.Key == "category" {
			hasCategory = true
		}
	}
	if !hasIndustry {
		t.Fatalf("expected banking/cybersecurity industry attributes, got %+v", attrs)
	}
	if !hasCategory {
		t.Fatalf("expected category attributes mirrored from taxonomy, got %+v", attrs)
	}
}

// dualGateway returns enrichment JSON for the narrative prompt and attribute
// JSON for the attribute prompt, keyed off a marker in the system prompt.
type dualGateway struct{}

func (dualGateway) Enabled() bool { return true }
func (dualGateway) JSONCompletion(_ context.Context, system, _ string, _ int) ([]byte, error) {
	if strings.Contains(system, "Taxonomy:") {
		return []byte(`{"title":"Quake","summary":"S","whatHappened":"W","whyItMatters":"Y","severity":"HIGH","confidence":0.8,"tags":[{"code":"DISASTER.EARTHQUAKE","confidence":0.9}]}`), nil
	}
	return []byte(`{"country":"India","region":"Maharashtra","city":"Mumbai","geoScope":"local","sentiment":"negative","sentimentScore":-0.7,"influence":"high","relevance":0.9,"industries":["energy"],"entities":[{"name":"NDMA","type":"government"}]}`), nil
}

func TestEnrichSignalLLMPathPersistsGeoAndEntities(t *testing.T) {
	d := conn(t)
	ctx := context.Background()
	dbtest.SeedTaxonomy(t, d)

	mustExec(t, d, `INSERT INTO "Source" ("id","name","url","credibility","updatedAt") VALUES ('s1','P','https://s1.example',0.9,now())`)
	// Signal has no country yet → LLM-detected country should fill it.
	mustExec(t, d, `INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","sourceCount","metadata","updatedAt") VALUES ('sg','o','o',now(),now(),1,'{"tokenSet":"x"}',now())`)
	mustExec(t, d, `INSERT INTO "Article" ("id","sourceId","title","body","canonicalUrl") VALUES ('a1','s1','Quake','An earthquake struck.','https://s1.example/q')`)
	mustExec(t, d, `INSERT INTO "SignalArticle" ("signalId","articleId","relationType","addedAt") VALUES ('sg','a1','PRIMARY',now())`)

	if err := EnrichSignal(ctx, d, dualGateway{}, &fakeCrawler{text: "page"}, "sg", time.Now()); err != nil {
		t.Fatal(err)
	}
	var country, region, geoScope *string
	mustScan(t, d, `SELECT country, region, "geoScope" FROM "Signal" WHERE id='sg'`, &country, &region, &geoScope)
	if country == nil || *country != "IN" || region == nil || *region != "Maharashtra" || geoScope == nil || *geoScope != "LOCAL" {
		t.Fatalf("geo not persisted: %v %v %v", country, region, geoScope)
	}
	attrs, err := d.SignalAttributes(ctx, "sg")
	if err != nil {
		t.Fatal(err)
	}
	var hasEntity, hasEnergy bool
	for _, a := range attrs {
		if a.Key == "entity" && a.ValueText == "NDMA" && a.ValueCode == "GOVERNMENT" {
			hasEntity = true
		}
		if a.Key == "industry" && a.ValueCode == "ENERGY" {
			hasEnergy = true
		}
	}
	if !hasEntity || !hasEnergy {
		t.Fatalf("expected entity + industry attrs, got %+v", attrs)
	}
}

func TestBuildAttributeRows(t *testing.T) {
	rows := buildAttributeRows(
		[]llm.TagConf{{Code: "DISASTER.FLOOD", Confidence: 0.7}},
		llm.AttributeResult{
			Industries: []string{"ENERGY", "BANKING"},
			Entities:   []llm.Entity{{Name: "Acme", Type: "ORGANIZATION", Confidence: 0.8}},
		},
	)
	if len(rows) != 4 {
		t.Fatalf("expected 4 rows (1 category + 2 industry + 1 entity), got %d: %+v", len(rows), rows)
	}
	got := map[string]int{}
	for _, r := range rows {
		got[r.Key]++
	}
	if got["category"] != 1 || got["industry"] != 2 || got["entity"] != 1 {
		t.Fatalf("row distribution wrong: %v", got)
	}
	// empty input yields no rows.
	if r := buildAttributeRows(nil, llm.AttributeResult{}); len(r) != 0 {
		t.Fatalf("expected no rows, got %+v", r)
	}
}

func TestEnrichSignalCrawlFailureIsNonFatal(t *testing.T) {
	d := conn(t)
	ctx := context.Background()
	dbtest.SeedTaxonomy(t, d)
	gw := llm.NewOpenAIGateway("", "")

	mustExec(t, d, `INSERT INTO "Source" ("id","name","url","credibility","updatedAt") VALUES ('s1','P','https://s1.example',0.9,now())`)
	mustExec(t, d, `INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","sourceCount","metadata","updatedAt") VALUES ('sg','o','o',now(),now(),1,'{"tokenSet":"x"}',now())`)
	mustExec(t, d, `INSERT INTO "Article" ("id","sourceId","title","body","canonicalUrl") VALUES ('a1','s1','Quake','An earthquake struck.','https://s1.example/q')`)
	mustExec(t, d, `INSERT INTO "SignalArticle" ("signalId","articleId","relationType","addedAt") VALUES ('sg','a1','PRIMARY',now())`)

	cr := &fakeCrawler{shouldFail: true}
	if err := EnrichSignal(ctx, d, gw, cr, "sg", time.Now()); err != nil {
		t.Fatalf("crawl failure should not fail enrichment: %v", err)
	}
	// No crawled flag when the crawl produced nothing usable.
	var crawled *bool
	mustScan(t, d, `SELECT (metadata->>'crawled')::bool FROM "Signal" WHERE id='sg'`, &crawled)
	if crawled != nil {
		t.Fatalf("crawled flag should be absent on failure, got %v", *crawled)
	}
}
