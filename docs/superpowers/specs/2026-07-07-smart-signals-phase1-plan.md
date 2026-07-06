# Smart Signals — Phase 1 Implementation Plan (Relevance Engine)

**Branch:** `feat/smart-signals-relevance-engine` (do NOT merge to main until all phases land + verified)

Turns the generic signal stream into a **ranked, personalized "For You" feed**: each
subscriber profile carries *weighted interests*; every signal is scored for that
profile; the feed returns signals ranked by relevance with a plain-language "why",
and records feedback (open / 👍 / 👎) for later learning. No external data sources —
builds entirely on the existing enrichment + subscriptions.

## Components

### 1. `internal/relevance` (pure, unit-tested core)
- `Profile{ Interests map[string]float64; Keywords []string }` — keys are
  `dimension:value` (`tag:DISASTER`, `entity:nike`, `country:IN`, `region:...`,
  `sentiment:NEGATIVE`). Weight sets importance.
- `Signal{...}` — scorable projection (eventType, tags, country, region, entities,
  sentiment, influence, severity, relevance, confidence, ageHours, title, summary).
- `Score(p, s) Scored` and `Rank(p, sigs) []Scored`.
- Formula: `score = (2·match + quality) · recency` where `match` = Σ matched
  interest weights + keyword hits, `quality` = influence/severity/relevance/
  confidence blend (0..1), `recency` = decay in [0.3, 1]. `Reasons` lists what
  matched. Deterministic → fully unit-testable.

### 2. DB
- `Subscription.interests jsonb` column (ALTER ADD COLUMN IF NOT EXISTS).
- `SignalFeedback` table (subscriptionId, signalId, action, createdAt).
- `CandidateSignals(ctx, since, limit)` → recent signals + attributes projected to
  `relevance.Signal`.
- `RecordFeedback`, and expose stored interests on the subscription.

### 3. API (REST, mirrors existing `/v1/*`)
- `GET /v1/subscriptions/{id}/feed?limit=` → ranked signals (score + reasons + why).
- `POST /v1/feedback` → record open/up/down.
- `PATCH /v1/subscriptions/{id}/interests` → set weighted interests.

### 4. Frontend
- **"For You"** page: pick a profile → ranked feed with a relevance badge, the
  `whyItMatters` + matched reasons, and open/👍/👎 controls.
- **Interest weights** editor (extends the filter builder): add weighted interests.

### 5. Tests + verify
- Unit: scoring (match boost, domain-tag match, entity, keyword, recency, ranking,
  empty profile). DB: feed query + feedback. API: feed/feedback/interests. Frontend:
  For-You render + feedback click.
- Verify: drive the For-You feed against the local DB (~90k enriched signals);
  confirm ranking reflects weights and feedback records.

## Later phases (need external keys/accounts — scaffold interfaces now)
- **Phase 2:** social velocity, Google Trends, first-party ingestion adapters.
- **Phase 3:** Slack app, Shopify app, Signals API keys + metered billing.
