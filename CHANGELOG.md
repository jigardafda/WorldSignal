# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Live map upgrade.** The **Live** dashboard gains a richer, real-time picture:
  markers are now **sized by severity** and **faded by recency**, with a louder
  "breaking" ripple plus an aggregated toast for new HIGH/CRITICAL signals. A
  **Pins · Clusters · Heat** view toggle (clustering via `leaflet.markercluster`,
  a severity-weighted density surface via `leaflet.heat`) is persisted in the
  URL. A new collapsible **Live pulse** side-rail shows a **newest-first ticker**
  (click a row to open the signal and fly the map to its marker), an
  **events/min velocity** counter, and **top movers** — the categories and
  country hotspots surging in the recent half of the window. The map now
  **loads instantly** from an IndexedDB cache (cleared on logout) and is
  **offline-ready** via a service worker (`vite-plugin-pwa`) that precaches the
  app shell and runtime-caches OpenStreetMap tiles; an offline indicator shows
  when connectivity drops. The live-feed query cap was raised from 200 to 5000.
- **Per-route permission guards.** Directly navigating to a route the current
  role can't access (e.g. `/users`, `/settings`) now renders an "Access denied"
  page instead of the component, via a `RequirePerm` wrapper in `App.tsx` — the
  gated page never mounts or fetches. This is defence-in-depth alongside the nav
  gating and the authoritative server-side `authz` checks.
- **Full-text search.** Signal and article search now use ranked Postgres
  full-text search — a generated, weighted `tsvector` column (title > summary >
  briefing) backed by a GIN index, parsed with `websearch_to_tsquery` and ordered
  by `ts_rank` — replacing the previous unindexed `ILIKE` scan. A substring
  fallback (accelerated by best-effort `pg_trgm` trigram indexes) still catches
  partial words.
- **Queryable entities.** Entities extracted per Signal (people, organizations,
  places, …) are now first-class: a new **Entities** console page and
  `entities(search, type, limit)` GraphQL query / `GET /v1/entities` REST
  endpoint list distinct entities with mention counts, searchable by name and
  filterable by type. Signals can be filtered by entity (`filter.entity`), and
  the Entities page drills into the matching Signals.
- **Email delivery channel.** The `EMAIL` delivery channel is now fully
  implemented — matched Signals are rendered as HTML+text emails and sent over
  SMTP. See [docs/EMAIL.md](docs/EMAIL.md).
- **Email connectors.** Admin-managed, encrypted SMTP configurations with
  built-in presets for **Gmail, Outlook/Microsoft 365, Zoho, SendGrid** and a
  custom option, configured interactively in a new **Connectors** console page.
  Secrets are encrypted at rest (AES-256-GCM); connections are verified on save,
  with **Test** and **Send test email** actions. GraphQL:
  `emailConnectors`, `emailProviders`, `createEmailConnector`,
  `updateEmailConnector`, `setActiveEmailConnector`, `testEmailConnector`,
  `sendTestEmail`, `deleteEmailConnector` (all `settings:manage`).
- **Digests.** Email subscriptions can batch matched Signals into a single
  **hourly** or **daily** rollup instead of one message per Signal, driven by a
  new digest scheduler. Digests reuse the existing delivery queue, so they get the
  same retry/dead-letter handling.
- **Structured email subscription config** in the console (recipients, connector,
  delivery mode) instead of raw JSON.
- Config: `APP_BASE_URL` (link emails back to the console) and
  `DIGEST_TICK_SECONDS`.

## [0.1.0] - 2026-06-28

Initial public release.

### Added

- **Go backend** built on `net/http` with Postgres via pgx, providing the
  WorldSignal pipeline that converts public news and RSS/Atom feeds into
  deduplicated, enriched, classified Signals.
- **APIs:** a custom schemaless GraphQL executor plus a small REST surface.
- **Authentication and RBAC:** bearer session tokens, bcrypt password hashing,
  the `ADMIN`, `EDITOR`, and `VIEWER` roles, and the `settings:manage`
  permission.
- **Source catalog:** 1000+ validated global sources, with discovery,
  validation, and seeding via the `sourcetool` command
  (`backend/cmd/sourcetool`).
- **Automated ingestion:** a scheduler with concurrent workers and per-source
  cooldown, backed by a Postgres job queue (no Redis).
- **Enrichment:** OpenAI-based LLM enrichment with a deterministic heuristic
  fallback, plus LLM key management.
- **Audit log:** records of significant administrative and security-relevant
  actions.
- **Admin console:** a React + Vite + Mantine frontend for operating the
  system.
- **Operations:** a single `server` binary (`backend/cmd/server`) selectable as
  API and/or worker via `ROLE=all|api|worker`.

[Unreleased]: https://github.com/jigardafda/WorldSignal/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/jigardafda/WorldSignal/releases/tag/v0.1.0
