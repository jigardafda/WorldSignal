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
  `SourceValidationLog`, `LLMKey`, `AuditLog`, plus performance indexes.

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
| `TaxonomyTag` | Closed classification vocabulary. |
| `User`, `Session`, `Team`, `TeamMember` | Auth/RBAC. |
| `LLMKey` | Admin-managed provider keys (ciphertext + last4). |
| `AuditLog` | Security-relevant action history. |
| `ws_jobs` | Postgres-backed job queue. |

## Indexes

Indexes back every filter/sort/join column:

- **Source**: `country`, `enabled+priority`, `type`, `language`, `region`,
  `industry`, `geographicScope`, `validationStatus`, GIN on `tags`.
- **Signal**: `status`, `severity`, `confidence`, `country`, `lastSeenAt`.
- **Article**: `sourceId`, `canonicalUrl`, `contentHash`, `publishedAt`, `fetchedAt`.
- **RawItem**: `status`, `contentHash`, `publishedAt`, `fetchedAt`, unique `(sourceId, sourceGuid)`.
- **DeliveryEvent**: `status`, `createdAt`, unique `(subscriptionId, signalId)`.
- **Subscription**: `createdAt`.
- **SourceValidationLog**: `(sourceId, checkedAt DESC)`.
- **LLMKey**: `(provider, isActive)`.
- **AuditLog**: `createdAt DESC`, `actorId`, `action`.

## Referential integrity

Child rows cascade on parent delete (`SourceValidationLog`/`RawItem`/`Article`→
`Source`; `Session`/`TeamMember`→`User`/`Team`; `AuditLog` is independent/retained).
Deletes are exercised in tests to confirm cascades.

## Conventions / gotchas

- Unquoted identifiers are lowercased by Postgres — **always quote** camelCase columns.
- `jsonb` values gain whitespace on storage; tests compare structurally, not byte-wise.
- DB tests serialize (`go test -p 1`) because they share a database.
