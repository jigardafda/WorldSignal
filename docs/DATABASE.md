# Database

PostgreSQL accessed via `pgx/v5`. Identifiers are quoted camelCase (Prisma-style),
so all SQL quotes them (`"createdAt"`).

## Migrations

The original content tables (`Source`, `RawItem`, `Article`, `Signal`, …) originate
from the Prisma schema in `backend/schema/schema.prisma`. The Go backend owns two
idempotent migrations that run on every boot (`cmd/server/main.go`) and in tests
(`internal/dbtest`):

- **`MigrateAuth`** — `User`, `Session`, `Team`, `TeamMember`.
- **`MigrateContent`** — extends `Source` with rich metadata, and creates
  `SourceValidationLog`, `LLMKey`, `AuditLog`, `EmailConnector`, `DigestQueue`
  (plus `Subscription.lastDigestAt`), `ApiKey`/`ApiKeyUsage`, the generated
  `searchVector` full-text columns on `Signal`/`Article`, plus performance indexes.
- **`MigrateSearch`** — best-effort `pg_trgm` extension + trigram indexes for
  fuzzy/substring search. Logged-and-skipped if the role can't `CREATE EXTENSION`
  (full-text search is unaffected).

Both use `CREATE TABLE IF NOT EXISTS` / `ADD COLUMN IF NOT EXISTS`, so they apply
cleanly to existing databases and are safe to re-run (backward compatible). The
Prisma schema file is kept in sync for documentation/ORM parity.

## Core tables

| Table | Purpose |
|---|---|
| `Source` | Feeds + rich metadata (languages[], geographicScope, industry, orgType, sourceType, tags[], healthScore, validationStatus, …). |
| `SourceValidationLog` | One row per validation attempt (httpStatus, responseMs, itemCount, newestItemAt, error). FK→Source ON DELETE CASCADE. |
| `RawItem` | Immutable fetched feed items. FK→Source. |
| `Article` | Normalized articles (dedup by `contentHash`). FK→Source/RawItem. |
| `Signal` | Canonical events. |
| `SignalArticle`, `SignalTag` | Signal↔article and signal↔taxonomy joins. |
| `Subscription`, `Subscriber`, `DeliveryEvent` | Delivery routing + history. |
| `EmailConnector` | Admin-managed SMTP connectors for the email channel (secret ciphertext + last4). |
| `ApiKey` | Public-REST-API credentials — SHA-256 key hash + display prefix, scopes[], per-minute rate limit, usage counters. |
| `ApiKeyUsage` | Fixed-window (per-minute) request counters for API-key rate limiting. FK→ApiKey ON DELETE CASCADE. |
| `DigestQueue` | Signals queued for a digest-mode email subscription, drained into one rollup delivery per interval. FK→Subscription/Signal ON DELETE CASCADE. |
| `TaxonomyTag` | Closed classification vocabulary. |
| `User`, `Session`, `Team`, `TeamMember` | Auth/RBAC. |
| `LLMKey` | Admin-managed provider keys (ciphertext + last4). |
| `AuditLog` | Security-relevant action history. |
| `ws_jobs` | Postgres-backed job queue. |

## Indexes

Indexes back every filter/sort/join column:

- **Source**: `country`, `enabled+priority`, `type`, `language`, `region`,
  `industry`, `geographicScope`, `validationStatus`, GIN on `tags`.
- **Signal**: `status`, `severity`, `confidence`, `country`, `lastSeenAt`, GIN on `searchVector` (full-text), best-effort trigram GIN on `title`/`summary`.
- **Article**: GIN on `searchVector` (full-text) + best-effort trigram GIN on `title` (in addition to the columns below).
- **SignalAttribute**: partial index on `(valueText, valueCode) WHERE key='entity'` for entity search/filter.
- **Article**: `sourceId`, `canonicalUrl`, `contentHash`, `publishedAt`, `fetchedAt`.
- **RawItem**: `status`, `contentHash`, `publishedAt`, `fetchedAt`, unique `(sourceId, sourceGuid)`.
- **DeliveryEvent**: `status`, `createdAt`, unique `(subscriptionId, signalId)`.
- **Subscription**: `createdAt`.
- **SourceValidationLog**: `(sourceId, checkedAt DESC)`.
- **LLMKey**: `(provider, isActive)`.
- **EmailConnector**: `isActive`. **DigestQueue**: `(subscriptionId, queuedAt)`.
- **AuditLog**: `createdAt DESC`, `actorId`, `action`.

## Referential integrity

Child rows cascade on parent delete (`SourceValidationLog`/`RawItem`/`Article`→
`Source`; `Session`/`TeamMember`→`User`/`Team`; `AuditLog` is independent/retained).
Deletes are exercised in tests to confirm cascades.

## Conventions / gotchas

- Unquoted identifiers are lowercased by Postgres — **always quote** camelCase columns.
- `jsonb` values gain whitespace on storage; tests compare structurally, not byte-wise.
- DB tests serialize (`go test -p 1`) because they share a database.
