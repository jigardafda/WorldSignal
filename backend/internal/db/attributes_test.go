package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
)

func ptrS(s string) *string   { return &s }
func ptrF(f float64) *float64 { return &f }

// seedSignalWithArticle creates a minimal source/article/signal so enrichment
// has something to update, returning the signal id.
func seedSignalWithArticle(t *testing.T, d *db.DB) string {
	t.Helper()
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		if _, err := d.Pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("exec %q: %v", sql, err)
		}
	}
	exec(`INSERT INTO "Source" ("id","name","url","credibility","updatedAt") VALUES ('src','S','https://s.example',0.8,now())`)
	exec(`INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","country","sourceCount","metadata","updatedAt") VALUES ('sg','t','s',now(),now(),'IN',1,'{"tokenSet":"x"}',now())`)
	exec(`INSERT INTO "Article" ("id","sourceId","title","body") VALUES ('a','src','t','b')`)
	exec(`INSERT INTO "SignalArticle" ("signalId","articleId","relationType","addedAt") VALUES ('sg','a','PRIMARY',now())`)
	return "sg"
}

func TestApplyEnrichmentPersistsAttributes(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	dbtest.SeedTaxonomy(t, d)
	ctx := context.Background()
	id := seedSignalWithArticle(t, d)

	upd := db.EnrichmentUpdate{
		Title: "Quake", Summary: "A quake", Severity: "HIGH", Confidence: 0.7,
		Status: "DEVELOPING", PublishedAt: time.Now(), Metadata: map[string]any{"k": "v"},
		Region: ptrS("Maharashtra"), City: ptrS("Mumbai"), Locality: ptrS("Bandra"),
		GeoScope: ptrS("LOCAL"), Sentiment: ptrS("NEGATIVE"), SentimentScore: ptrF(-0.6),
		Influence: ptrS("HIGH"), Relevance: ptrF(0.85),
		Attributes: []db.SignalAttr{
			{Key: "industry", ValueCode: "TECHNOLOGY", Confidence: 1},
			{Key: "category", ValueCode: "DISASTER.EARTHQUAKE", Confidence: 0.9},
			{Key: "entity", ValueCode: "ORGANIZATION", ValueText: "Acme Corp", Confidence: 0.8},
		},
	}
	if err := d.ApplyEnrichment(ctx, id, upd); err != nil {
		t.Fatal(err)
	}

	agg, err := d.GetSignal(ctx, id)
	if err != nil || agg == nil {
		t.Fatalf("get signal: %v", err)
	}
	if agg.Country == nil || *agg.Country != "IN" {
		t.Errorf("country should be preserved (COALESCE): %v", agg.Country)
	}
	if agg.Region == nil || *agg.Region != "Maharashtra" || agg.City == nil || *agg.City != "Mumbai" {
		t.Errorf("region/city wrong: %v %v", agg.Region, agg.City)
	}
	if agg.GeoScope == nil || *agg.GeoScope != "LOCAL" {
		t.Errorf("geoScope wrong: %v", agg.GeoScope)
	}
	if agg.Sentiment == nil || *agg.Sentiment != "NEGATIVE" || agg.SentimentScore == nil || *agg.SentimentScore != -0.6 {
		t.Errorf("sentiment wrong: %v %v", agg.Sentiment, agg.SentimentScore)
	}
	if agg.Influence == nil || *agg.Influence != "HIGH" || agg.Relevance == nil || *agg.Relevance != 0.85 {
		t.Errorf("influence/relevance wrong: %v %v", agg.Influence, agg.Relevance)
	}
	if len(agg.Attributes) != 3 {
		t.Fatalf("expected 3 attributes, got %d: %+v", len(agg.Attributes), agg.Attributes)
	}
	// ordered by key,valueCode,valueText: category, entity, industry
	if agg.Attributes[0].Key != "category" || agg.Attributes[0].ValueCode != "DISASTER.EARTHQUAKE" {
		t.Errorf("attr[0] = %+v", agg.Attributes[0])
	}
	if agg.Attributes[1].Key != "entity" || agg.Attributes[1].ValueText != "Acme Corp" {
		t.Errorf("attr[1] = %+v", agg.Attributes[1])
	}
}

func TestApplyEnrichmentCoalesceAndReplace(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	dbtest.SeedTaxonomy(t, d)
	ctx := context.Background()
	id := seedSignalWithArticle(t, d)

	// First pass sets sentiment + one industry.
	if err := d.ApplyEnrichment(ctx, id, db.EnrichmentUpdate{
		Title: "t", Summary: "s", Severity: "LOW", Confidence: 0.5, Status: "UNVERIFIED",
		PublishedAt: time.Now(), Metadata: map[string]any{}, Sentiment: ptrS("POSITIVE"),
		Attributes: []db.SignalAttr{{Key: "industry", ValueCode: "BANKING", Confidence: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	// Second pass: nil Sentiment must NOT erase it; attributes fully replaced.
	if err := d.ApplyEnrichment(ctx, id, db.EnrichmentUpdate{
		Title: "t2", Summary: "s2", Severity: "LOW", Confidence: 0.5, Status: "UNVERIFIED",
		PublishedAt: time.Now(), Metadata: map[string]any{},
		Attributes: []db.SignalAttr{{Key: "industry", ValueCode: "ENERGY", Confidence: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	agg, err := d.GetSignal(ctx, id)
	if err != nil || agg == nil {
		t.Fatal(err)
	}
	if agg.Sentiment == nil || *agg.Sentiment != "POSITIVE" {
		t.Errorf("sentiment should be preserved via COALESCE: %v", agg.Sentiment)
	}
	if len(agg.Attributes) != 1 || agg.Attributes[0].ValueCode != "ENERGY" {
		t.Errorf("attributes should be replaced, got %+v", agg.Attributes)
	}
}
