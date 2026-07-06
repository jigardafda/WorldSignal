# Categorization Accuracy Overhaul — Design

**Date:** 2026-07-06
**Goal:** Drive signal categorization accuracy toward ~99% and stop the large share of
signals defaulting to `GENERAL.OTHER` / uncategorized.

## Problem

Signals are enriched with a topic **category** (a taxonomy code like `DISASTER.EARTHQUAKE`)
via two paths in `backend/internal/llm`:

- **LLM path** (`gateway.go::runLLM`) when an API key is configured.
- **Heuristic path** (`enrich.go::heuristic`) — deterministic keyword matcher, the fallback
  when no key is set or the LLM call fails.

The first tag becomes the signal's `eventType`; its domain (prefix before `.`) becomes the
map layer. A large fraction of signals land in `GENERAL.OTHER`.

### Root causes (by impact)

1. **Sparse taxonomy** — 10 domains / 27 leaves. Entire common news topics have no bucket
   (crime, environment/climate, science/space, education, culture/entertainment/religion,
   immigration & social issues, protests, energy, transport/accidents, weather, scandal).
   → forced into `GENERAL.OTHER`.
2. **No domain-level fallback** — a story clearly in a domain but not matching a narrow leaf
   (Politics-but-not-elections) has nowhere to go but `GENERAL.OTHER`. No `POLITICS.OTHER`.
3. **Weak heuristic** — `strings.Contains` substring match, ~5 keywords/leaf, no word
   boundaries (`war` ⊂ `warm`), no aliases/stemming. Most real headlines score 0 → GENERAL.
4. **LLM prompt invites GENERAL** — flat leaf list, no domain descriptions/examples, explicit
   "If nothing fits, use GENERAL.OTHER" escape hatch, no "prefer closest domain" instruction.

## Design

### 1. Expand + layer the taxonomy (`taxonomy.go`)

~17 topical domains + `GENERAL`, each domain gaining a `<DOMAIN>.OTHER` leaf so any story
recognized at the domain level lands in that domain — never GENERAL. GENERAL.OTHER becomes a
true last resort. Keyword/alias lists per leaf are substantially expanded (they feed both the
heuristic and give the LLM anchors).

Domains: POLITICS, ECONOMY, BUSINESS, TECHNOLOGY, SCIENCE, ENVIRONMENT, DISASTER,
PUBLIC_HEALTH, LEGAL, CRIME, CONFLICT, SOCIETY, CULTURE, SPORTS, EDUCATION, ENERGY,
TRANSPORT, GENERAL.

Serialization shape and `GENERAL.OTHER` staying last are preserved (byte-parity test holds).
`categoryValues` in `attributes.go` mirrors the taxonomy automatically.

### 2. Rewrite the heuristic classifier (`enrich.go`)

- Token / word-boundary matching (kill false substrings), case-insensitive aliases.
- **Two-tier fallback:** best leaf → else `<DOMAIN>.OTHER` when domain-level keywords hit →
  else `GENERAL.OTHER`.
- Domain-level keyword nets so broad stories still resolve to a domain.

### 3. Rewrite the LLM prompt (`gateway.go`)

- Taxonomy grouped by domain with a one-line description per domain + a few worked examples.
- Instruct: choose the most specific leaf; if the leaf is unclear pick the domain's `.OTHER`;
  use `GENERAL.OTHER` ONLY when no domain applies (target < 1%).

### 4. Measurement harness (new `*_test.go`)

Labeled corpus of ~150 realistic headlines spanning every domain with an expected domain.
Deterministic test on the **heuristic path** asserting **≥99% land outside `GENERAL.OTHER`**
plus a domain-accuracy floor. CI cannot call the real LLM, so this hardens the deterministic
floor and prevents regressions; the LLM path performs at least as well in production.

### 5. Backfill

Re-enqueue enrichment (`QEnrichSignal`) for existing signals currently in `GENERAL.OTHER` so
live numbers move, not just new ingests.

### 6. Frontend sync (`frontend/src/lib/categories.ts`)

Extend the domain → color/label map for the new domains so map layers / UI render them. The
taxonomy tree itself is already served dynamically from the backend.

## Success criteria — achieved

- Heuristic corpus test (`TestClassificationCorpusFloor`, 138 realistic headlines
  spanning all domains, incl. adversarial cases): **100% categorized outside
  `GENERAL.OTHER`**, **99.3% exact-domain accuracy** (1 genuine politics/legal
  ambiguity). Floors enforced at 99% non-GENERAL and 90% domain.
- Taxonomy grew from 10 domains / 27 leaves → **18 domains / ~90 leaves**, every
  topical domain carrying a `<DOMAIN>.OTHER` catch-all.
- `go test ./...` (serialized) and all 312 frontend tests green; typecheck + gofmt clean.
- Backfill: `sourcetool reenrich [-limit N]` re-enqueues enrichment for signals in
  the GENERAL domain so existing data is reclassified, not just new ingests.
