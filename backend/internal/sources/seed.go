package sources

import (
	"context"
	"encoding/json"
	"time"

	"github.com/worldsignal/backend/internal/cuid"
	"github.com/worldsignal/backend/internal/db"
)

// SeedSummary reports the outcome of seeding validated sources.
type SeedSummary struct {
	Inserted int
	Updated  int
	Logs     int
	Skipped  int // results that did not pass validation
}

// SeedValid upserts every passing Result into the Source table (with full
// metadata) and writes a SourceValidationLog row per attempt. Failing results
// are skipped (they have no source row to attach a log to). Safe to re-run:
// existing URLs are updated in place.
func SeedValid(ctx context.Context, d *db.DB, results []Result) (SeedSummary, error) {
	var sum SeedSummary
	for _, r := range results {
		if !r.OK {
			sum.Skipped++
			continue
		}
		c := r.Candidate

		legacyType := "RSS"
		parser := "rss"
		if c.SourceType == "ATOM" {
			legacyType, parser = "ATOM", "atom"
		}
		lang := "en"
		if len(c.Languages) > 0 {
			lang = c.Languages[0]
		}
		meta := map[string]any{}
		for k, v := range c.Metadata {
			meta[k] = v
		}
		meta["validation"] = map[string]any{
			"itemCount": r.ItemCount, "responseMs": r.ResponseMs,
			"httpStatus": r.HTTPStatus, "redirectedTo": r.RedirectedTo,
		}
		if len(c.Keywords) > 0 {
			meta["keywords"] = c.Keywords
		}
		metaJSON, _ := json.Marshal(meta)

		var newest *time.Time
		if r.NewestItem != nil {
			newest = r.NewestItem
		}

		id := cuid.New()
		var sourceID string
		var inserted bool
		// lastValidatedAt is timestamptz (new) while lastSuccessAt/lastFetchedAt are
		// Prisma's timestamp(3) — using the SQL now() for all three avoids binding a
		// single Go time.Time to columns of differing types.
		err := d.Pool.QueryRow(ctx, `
INSERT INTO "Source" (
  "id","name","type","url","websiteUrl","country","region","language","languages",
  "geographicScope","category","industry","subcategory","publisher","orgType","sourceType",
  "officialFeed","priority","credibility","crawlFrequency","parserType","enabled","tags",
  "healthScore","validationStatus","lastValidatedAt","lastSuccessAt","lastFetchedAt",
  "avgResponseMs","metadata","createdAt","updatedAt"
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,true,$22,
  $23,'VALID',now(),now(),now(),$24,$25,now(),now()
)
ON CONFLICT ("url") DO UPDATE SET
  "name"=EXCLUDED."name","websiteUrl"=EXCLUDED."websiteUrl","country"=EXCLUDED."country",
  "region"=EXCLUDED."region","language"=EXCLUDED."language","languages"=EXCLUDED."languages",
  "geographicScope"=EXCLUDED."geographicScope","category"=EXCLUDED."category",
  "industry"=EXCLUDED."industry","subcategory"=EXCLUDED."subcategory","publisher"=EXCLUDED."publisher",
  "orgType"=EXCLUDED."orgType","sourceType"=EXCLUDED."sourceType","officialFeed"=EXCLUDED."officialFeed",
  "tags"=EXCLUDED."tags","healthScore"=EXCLUDED."healthScore","validationStatus"='VALID',
  "lastValidatedAt"=now(),"lastSuccessAt"=now(),
  "lastValidationError"=NULL,"avgResponseMs"=EXCLUDED."avgResponseMs","metadata"=EXCLUDED."metadata",
  "updatedAt"=now()
RETURNING "id", (xmax = 0) AS inserted`,
			id, c.Name, legacyType, c.FeedURL, nullStr(c.WebsiteURL), nullStr(c.Country),
			nullStr(c.Region), lang, c.Languages, nullStr(c.GeographicScope), nullStr(c.Category),
			nullStr(c.Industry), nullStr(c.Subcategory), nullStr(c.Publisher), nullStr(c.OrgType),
			nullStr(c.SourceType), c.OfficialFeed, c.Priority, c.Credibility, 900, parser,
			c.Tags, r.HealthScore, r.ResponseMs, metaJSON,
		).Scan(&sourceID, &inserted)
		if err != nil {
			return sum, err
		}
		if inserted {
			sum.Inserted++
		} else {
			sum.Updated++
		}

		// Validation-log row.
		if _, err := d.Pool.Exec(ctx, `
INSERT INTO "SourceValidationLog" ("id","sourceId","checkedAt","ok","httpStatus","responseMs","itemCount","newestItemAt","redirectedTo","error")
VALUES ($1,$2,now(),true,$3,$4,$5,$6,$7,NULL)`,
			cuid.New(), sourceID, r.HTTPStatus, r.ResponseMs, r.ItemCount, newest, nullStr(r.RedirectedTo)); err != nil {
			return sum, err
		}
		sum.Logs++
	}
	return sum, nil
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
