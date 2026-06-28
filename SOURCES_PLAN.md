# Global Source Expansion — Execution Plan

Authoritative spec: [GOAL_GLOBAL_SOURCES.md](GOAL_GLOBAL_SOURCES.md).
Target: **1,000+ validated, active, richly-tagged** news/knowledge sources with global
coverage (every country, all regions, industries, 30 languages). **Every source is
validated by a live fetch before insertion** — no unverified/broken/dead/duplicate feeds.

## Strategy

A blend that maximizes both authority and coverage, every entry validated:

1. **Curated direct publisher feeds** — a substantial hand-built set of leading native
   publications, national agencies, public broadcasters, gov/security/research feeds,
   and per-industry outlets, with full metadata.
2. **Google News editions as a universal structured aggregator** (spec §6 explicitly
   permits structured/alternative sources) — covers every country × language × topic and
   industry search, all RSS, active, fresh, parseable. Tagged `sourceType=AGGREGATOR`,
   `publisher="Google News"`, `officialFeed=false` so they're distinguishable from direct
   feeds. Families:
   - top stories: `…/rss?hl={lang}&gl={CC}&ceid={CC}:{lang}`
   - topic: `…/rss/headlines/section/topic/{TOPIC}?hl=…&gl=…&ceid=…`
   - industry/topic search (multilingual): `…/rss/search?q={q}&hl=…&gl=…&ceid=…`

The validator fetches **all** candidates and inserts only those that pass. Target the
candidate pool well above 1,000 so the validated survivors clear the bar.

## Phases

- [ ] **S0 Schema & data model** — idempotent content migration adding rich metadata to
      `Source` (websiteUrl, languages[], geographicScope, industry, subcategory, publisher,
      orgType, sourceType, officialFeed, tags[], healthScore, validationStatus,
      lastValidatedAt, lastValidationError, avgResponseMs, metadata jsonb) + new
      `SourceValidationLog` table. Wire into boot + dbtest. Backfill existing rows. Update
      prisma doc.
- [ ] **S1 Validator + discovery engine** (`internal/sources` + `cmd/sourcetool`) —
      candidate model, curated dataset, Google-News matrix generator, concurrent validator
      (HTTP + gofeed parse + freshness + redirect/dup checks + healthScore), upsert of
      passing rows + validation-log writes. CLI: `validate` (report) / `seed` (insert).
- [ ] **S2 Backend exposure** — extend Source DB struct/queries + new filters
      (language/region/scope/industry/orgType/tags/validation/health); GraphQL fields,
      validation-log query, `revalidateSource` mutation, coverage aggregates.
- [ ] **S3 Frontend** — Sources list columns+filters, Source detail full metadata +
      validation history + revalidate, a Coverage analytics page (by country/region/
      language/industry/scope).
- [ ] **S4 Run at scale** — execute validator on full candidate pool; insert 1,000+
      passing; verify coverage breadth.
- [ ] **S5 Quality gates** — backend & frontend coverage ≥95%, e2e, vet/typecheck clean.

## Exit criteria
- ≥1,000 sources in DB, all `validationStatus=VALID`, each with a `lastValidatedAt` and a
  validation-log entry; zero dead/duplicate.
- Coverage spans every country, all listed regions, all industry buckets, all 30 languages.
- Rich metadata + multi-dimensional tags populated; exposed and filterable in the UI.
- Tests green at ≥95%.
