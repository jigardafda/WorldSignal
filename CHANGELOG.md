# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Public REST API authentication.** Every `/v1/*` endpoint now requires a
  scoped **API key** (only `/health` stays open) — closing a gap where the REST
  surface was unauthenticated behind permissive CORS. Keys are stored **hashed**
  (SHA-256; the raw secret is shown once at creation), carry a **scope** set
  (`signals:read`, `sources:read/write`, `subscriptions:read/write`,
  `deliveries:read`, `stats:read`), and enforce a **per-minute rate limit**
  (Postgres fixed-window) returning `401`/`403`/`429` with `X-RateLimit-*` and
  `Retry-After` headers. Managed in a new **API Keys** console page and via
  GraphQL (`apiKeys`, `apiScopes`, `createApiKey`, `setApiKeyEnabled`,
  `deleteApiKey`; `settings:manage`). See [docs/API.md](docs/API.md#authentication-api-keys).

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
