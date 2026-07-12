# Multi-Tenant SaaS — Architecture & Migration Design

**Status:** Draft for review · **Date:** 2026-07-12
**Scope:** Turn WorldSignal (currently a single-tenant admin console over a global
signal pipeline) into a multi-tenant SaaS served from **one instance**, without
forking the shared signal corpus.

## Decisions locked

| Decision | Choice | Rationale |
|---|---|---|
| Signal corpus | **Fully shared global pool** | Crawling + LLM enrichment is expensive and identical for everyone; tenants differ only in the *lens* (filter + ranking + delivery). Matches the existing schema. |
| Identity | **One `User` table, `accountId`-scoped** | Staff = user with `accountId = NULL` + platform role; customer = user tied to an account. Reuses the existing `ADMIN/EDITOR/VIEWER` matrix. |
| Onboarding | **Admin-provisioned first, self-serve later** | Ship isolation + billing safely before opening public signup. |
| Deliverable order | **Design (this doc) → Account layer → isolation → UI split → billing/self-serve** | Get tenant isolation right at the data layer before anything customer-facing exists. |

## Core principle

> **Signals are central. The _lens_ onto signals is per-tenant.**

The ingest → parse → enrich → cluster pipeline runs **once, globally**. A tenant
never owns a copy of a `Signal`. A tenant owns a **view**: which signals it may
see (`Subscription.filter`), how they rank (`Subscription.interests`), how they
are delivered (channel/connector), and the feedback/usage that personalizes it.

Aggregate tenant interest **may feed back into crawl prioritization** (crawl
sources many tenants care about more often) — but that is an optimization input
to the central scheduler; it never forks the corpus.

## Today's layers (what already exists)

The schema already separates cleanly into three layers:

| Layer | Tables | Ownership |
|---|---|---|
| **Signal factory** (ingest→enrich) | `Source`, `RawItem`, `Article`, `Signal`, `SignalArticle`, `SignalAttribute`, `SignalTag`, `TaxonomyTag` | **Central** — stays global, unchanged |
| **Platform ops / infra** | `User`, `Session`, `Team`, `LLMKey`, `EmailConnector` (system default), `AuditLog`, `ws_jobs` | **Central** — operator-owned |
| **Consumption / delivery** | `Subscriber`, `Subscription` (`filter`, `interests`), `DeliveryEvent`, `DigestQueue`, `SignalFeedback`, `ApiKey` | **Per-tenant** — needs `accountId` |

`Subscriber` today is a proto-account. We **absorb `Subscriber` into `Account`**
rather than keep two overlapping "customer" concepts.

## Target entity model

### New: `Account` (the tenant spine)

```
Account
  id            text pk
  name          text
  slug          text unique          -- subdomain / URL segment
  status        text  ACTIVE|SUSPENDED|DELETED
  plan          text  FREE|PRO|ENTERPRISE
  billingRef    text  null            -- Stripe customer id, later
  createdAt     timestamptz
```

`Subscriber` is migrated into `Account` (same id space) so existing
`Subscription.subscriberId` becomes `Subscription.accountId` with no data loss.

### Add `accountId` to per-tenant tables

| Table | Change |
|---|---|
| `User` | `+ accountId text NULL` (NULL = platform staff), `+ platformRole` stays as `role` |
| `Subscription` | `subscriberId` → `accountId` (rename FK target to `Account`) |
| `ApiKey` | `+ accountId text NOT NULL` (currently global — **must** be scoped) |
| `DeliveryEvent` | inherits scope via `Subscription`; add denormalized `accountId` for RLS + fast filtering |
| `SignalFeedback`, `DigestQueue` | inherit via `Subscription` |
| `EmailConnector` | `+ accountId text NULL` (NULL = system default, non-null = tenant-owned SMTP) |

`AuditLog` gains `+ accountId text NULL` so tenant actions are attributable while
platform actions stay global.

### New: per-account quota / usage

```
AccountQuota   (accountId pk, maxSubscriptions, maxSeats, signalsPerMonth, apiReqPerMin)
AccountUsage   (accountId, windowStart, apiRequests, signalsServed, ...)  -- metering
```

`ApiKey.rateLimitPerMin` stays as a per-key cap; the **account** quota is the
outer bound enforced across all of a tenant's keys.

## Tenant isolation (the part that must be bulletproof)

**Model: shared database + row scoping.** No schema-per-tenant or DB-per-tenant —
those fight the shared-corpus design and add ops cost for no isolation benefit
here (the corpus is deliberately shared; only the thin consumption layer is
private).

Two enforcement layers, defense-in-depth:

1. **Application layer** — every per-tenant query carries `WHERE "accountId" = $ctx`.
   The account is resolved once in middleware and threaded through the request
   context (mirrors how `auth` already threads identity).
2. **Database layer — Postgres Row-Level Security (RLS)** on the per-tenant tables
   as a backstop, so a query that *forgets* the filter returns nothing instead of
   leaking. Set `SET LOCAL app.account_id = $ctx` per transaction; policies read it.

```sql
ALTER TABLE "Subscription" ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON "Subscription"
  USING ("accountId" = current_setting('app.account_id', true));
```

Central tables (`Signal`, `Article`, `Source`, …) get **no** RLS — they are
readable by all tenants by design. Writes to them remain operator-only via the
existing permission matrix.

### Tenant resolution (request → account)

- **Customer web app:** subdomain (`acme.worldsignal.io`) or path (`/app/:slug`) →
  resolve to `accountId`, cross-check against the session's `User.accountId`.
- **Public API (`/v1/*`):** the `ApiKey` *is* the account — once `ApiKey.accountId`
  exists, the key fully determines tenant context. No extra header needed.
- **Operator console:** no account context (or an explicit "acting as account X"
  for support), gated by platform role.

## Two UI entry points (one codebase)

Not two apps — **two shells over the same component library**, split by auth
context (`isPlatformStaff` vs `accountId`). Most of `src/pages` and
`src/components` are reused verbatim.

| Console | Route base | Login | Pages (mostly existing) |
|---|---|---|---|
| **Operator console** | `/admin/*` | staff (`accountId = NULL`) | Sources, Coverage, RawItems, Jobs, Connectors, LLM keys, global Signals, Users, Audit, **+ new Accounts admin** |
| **Tenant app** | `app.` / `/app/*` | customer (`accountId` set) | ForYou, Profiles, Subscriptions, filtered Signals/LiveDashboard, their ApiKeys, Deliveries, Account/billing |

Mechanics:
- Split `App.tsx`'s single route tree into `AdminRoutes` + `TenantRoutes` behind a
  context guard.
- Reuse `Layout` with two nav sets.
- Existing `RequirePerm` gating stays; tenant routes additionally require a
  resolved `accountId`.
- New **Accounts admin** page (operator-only): list/create/suspend accounts,
  invite first user, view usage.

## Migration sequence

Each step is a self-contained, idempotent migration in the style of
`internal/db/migrate.go` (`ADD COLUMN IF NOT EXISTS`, backfill, then enforce).

1. **Introduce `Account`** + migrate `Subscriber` rows into it. Backfill a single
   `default` account; point all existing `Subscription`/`ApiKey`/`DeliveryEvent`
   rows at it. Nullable → populate → `NOT NULL`. **Non-breaking.**
2. **Thread account context** through auth middleware and every per-tenant query;
   enable RLS + `SET LOCAL app.account_id`. Add regression tests that assert
   cross-tenant reads return empty.
3. **Scope `ApiKey` to accounts**; add `AccountQuota` + `AccountUsage` metering.
4. **Split the frontend** into operator vs tenant shells; add the Accounts admin
   page and tenant Account/settings page.
5. **Plans + billing + self-serve signup** (Stripe, plan gating, public register →
   auto-create account). Last, once isolation is proven.

**Do step 2 before step 4.** A cross-tenant leak is the one SaaS bug that is
fatal, and it is cheapest to close at the data layer before any customer-facing
surface exists.

## Explicitly out of scope (for now)

- Private per-tenant sources/signals (would push tenant-scoping into the central
  pipeline — revisit only if a customer requires it).
- Per-tenant LLM keys / bring-your-own-model.
- Regional data residency / DB-per-tenant.

## Open questions for review

1. **Slug/routing:** subdomain (`acme.worldsignal.io`) vs path (`/app/acme`)?
   Subdomain is cleaner for cookies/branding but needs wildcard TLS + DNS.
2. **Seats vs API-only:** do early customers log into the tenant web app, or are
   they purely API/webhook consumers? Affects how much of the tenant UI to build
   in step 4.
3. **Billing provider:** Stripe assumed — confirm before step 5.
4. **Signal visibility windows:** should plan tier gate *how far back* / *how fresh*
   a tenant can query the shared pool (e.g. FREE = 24h delay)? Cheap lever, worth
   deciding early since it touches the query layer in step 2.
