package db

import "context"

// authSchema creates the auth/RBAC tables the Go backend owns (the original
// content tables come from the Prisma schema in backend/schema/schema.prisma).
const authSchema = `
CREATE TABLE IF NOT EXISTS "User" (
  "id"           text PRIMARY KEY,
  "email"        text NOT NULL UNIQUE,
  "name"         text NOT NULL DEFAULT '',
  "passwordHash" text NOT NULL,
  "role"         text NOT NULL DEFAULT 'VIEWER',
  "status"       text NOT NULL DEFAULT 'ACTIVE',
  "createdAt"    timestamptz NOT NULL DEFAULT now(),
  "updatedAt"    timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS "Session" (
  "id"        text PRIMARY KEY,
  "token"     text NOT NULL UNIQUE,
  "userId"    text NOT NULL REFERENCES "User"("id") ON DELETE CASCADE,
  "expiresAt" timestamptz NOT NULL,
  "createdAt" timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS "Session_token_idx" ON "Session"("token");
CREATE TABLE IF NOT EXISTS "Team" (
  "id"        text PRIMARY KEY,
  "name"      text NOT NULL,
  "createdAt" timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS "TeamMember" (
  "teamId"  text NOT NULL REFERENCES "Team"("id") ON DELETE CASCADE,
  "userId"  text NOT NULL REFERENCES "User"("id") ON DELETE CASCADE,
  "role"    text NOT NULL DEFAULT 'MEMBER',
  "addedAt" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("teamId", "userId")
);`

// MigrateAuth ensures the auth/RBAC tables exist.
func (d *DB) MigrateAuth(ctx context.Context) error {
	_, err := d.Pool.Exec(ctx, authSchema)
	return err
}

// contentSchema extends the Prisma-owned content tables with the rich source
// metadata + validation model the global-source expansion needs. Every statement
// is idempotent (ADD COLUMN IF NOT EXISTS / CREATE TABLE IF NOT EXISTS) so it can
// run on existing dev/test/e2e databases without a Prisma migration.
const contentSchema = `
ALTER TABLE "Source" ADD COLUMN IF NOT EXISTS "websiteUrl"          text;
ALTER TABLE "Source" ADD COLUMN IF NOT EXISTS "languages"           text[] NOT NULL DEFAULT '{}';
ALTER TABLE "Source" ADD COLUMN IF NOT EXISTS "geographicScope"     text;
ALTER TABLE "Source" ADD COLUMN IF NOT EXISTS "industry"            text;
ALTER TABLE "Source" ADD COLUMN IF NOT EXISTS "subcategory"         text;
ALTER TABLE "Source" ADD COLUMN IF NOT EXISTS "publisher"           text;
ALTER TABLE "Source" ADD COLUMN IF NOT EXISTS "orgType"             text;
ALTER TABLE "Source" ADD COLUMN IF NOT EXISTS "sourceType"          text;
ALTER TABLE "Source" ADD COLUMN IF NOT EXISTS "officialFeed"        boolean NOT NULL DEFAULT false;
ALTER TABLE "Source" ADD COLUMN IF NOT EXISTS "contentType"         text;
ALTER TABLE "Source" ADD COLUMN IF NOT EXISTS "updateFrequency"     text;
ALTER TABLE "Source" ADD COLUMN IF NOT EXISTS "tags"                text[] NOT NULL DEFAULT '{}';
ALTER TABLE "Source" ADD COLUMN IF NOT EXISTS "biasRating"          text;
ALTER TABLE "Source" ADD COLUMN IF NOT EXISTS "healthScore"         integer;
ALTER TABLE "Source" ADD COLUMN IF NOT EXISTS "validationStatus"    text NOT NULL DEFAULT 'PENDING';
ALTER TABLE "Source" ADD COLUMN IF NOT EXISTS "lastValidatedAt"     timestamptz;
ALTER TABLE "Source" ADD COLUMN IF NOT EXISTS "lastValidationError" text;
ALTER TABLE "Source" ADD COLUMN IF NOT EXISTS "avgResponseMs"       integer;
ALTER TABLE "Source" ADD COLUMN IF NOT EXISTS "metadata"            jsonb;
ALTER TABLE "Source" ADD COLUMN IF NOT EXISTS "cooldownUntil"       timestamptz;

CREATE INDEX IF NOT EXISTS "Source_language_idx"   ON "Source"("language");
CREATE INDEX IF NOT EXISTS "Source_region_idx"     ON "Source"("region");
CREATE INDEX IF NOT EXISTS "Source_industry_idx"   ON "Source"("industry");
CREATE INDEX IF NOT EXISTS "Source_scope_idx"      ON "Source"("geographicScope");
CREATE INDEX IF NOT EXISTS "Source_validation_idx" ON "Source"("validationStatus");
CREATE INDEX IF NOT EXISTS "Source_tags_idx"       ON "Source" USING gin ("tags");
-- Scheduler scans enabled sources by priority; this index backs due-source selection.
CREATE INDEX IF NOT EXISTS "Source_enabled_cooldown_idx" ON "Source"("enabled","cooldownUntil","priority");

CREATE TABLE IF NOT EXISTS "SourceValidationLog" (
  "id"           text PRIMARY KEY,
  "sourceId"     text NOT NULL REFERENCES "Source"("id") ON DELETE CASCADE,
  "checkedAt"    timestamptz NOT NULL DEFAULT now(),
  "ok"           boolean NOT NULL,
  "httpStatus"   integer,
  "responseMs"   integer,
  "itemCount"    integer,
  "newestItemAt" timestamptz,
  "redirectedTo" text,
  "error"        text
);
CREATE INDEX IF NOT EXISTS "SourceValidationLog_source_idx" ON "SourceValidationLog"("sourceId","checkedAt" DESC);

CREATE TABLE IF NOT EXISTS "LLMKey" (
  "id"            text PRIMARY KEY,
  "provider"      text NOT NULL DEFAULT 'OPENAI',
  "label"         text NOT NULL,
  "keyCiphertext" text NOT NULL,
  "keyLast4"      text NOT NULL,
  "model"         text,
  "isActive"      boolean NOT NULL DEFAULT false,
  "status"        text NOT NULL DEFAULT 'UNTESTED',
  "lastTestedAt"  timestamptz,
  "lastError"     text,
  "createdBy"     text,
  "createdAt"     timestamptz NOT NULL DEFAULT now(),
  "updatedAt"     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS "LLMKey_provider_active_idx" ON "LLMKey"("provider","isActive");

CREATE TABLE IF NOT EXISTS "AuditLog" (
  "id"         text PRIMARY KEY,
  "actorId"    text,
  "actorEmail" text,
  "actorRole"  text,
  "action"     text NOT NULL,
  "targetType" text,
  "targetId"   text,
  "metadata"   jsonb,
  "createdAt"  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS "AuditLog_createdAt_idx" ON "AuditLog"("createdAt" DESC);
CREATE INDEX IF NOT EXISTS "AuditLog_actor_idx" ON "AuditLog"("actorId");
CREATE INDEX IF NOT EXISTS "AuditLog_action_idx" ON "AuditLog"("action");

-- Backfill indexes for list sort columns the Prisma schema didn't cover.
CREATE INDEX IF NOT EXISTS "DeliveryEvent_createdAt_idx" ON "DeliveryEvent"("createdAt" DESC);
CREATE INDEX IF NOT EXISTS "Subscription_createdAt_idx"  ON "Subscription"("createdAt" DESC);
CREATE INDEX IF NOT EXISTS "Article_fetchedAt_idx"       ON "Article"("fetchedAt" DESC);
CREATE INDEX IF NOT EXISTS "RawItem_fetchedAt_idx"       ON "RawItem"("fetchedAt" DESC);

-- Deep-enrichment attributes: hot scalar/geo dimensions promoted to Signal
-- columns for fast filtering; everything else lives in SignalAttribute. Values
-- are written only after normalization through the internal/attributes dictionary.
ALTER TABLE "Signal" ADD COLUMN IF NOT EXISTS "city"           text;
ALTER TABLE "Signal" ADD COLUMN IF NOT EXISTS "locality"       text;
ALTER TABLE "Signal" ADD COLUMN IF NOT EXISTS "geoScope"       text;
ALTER TABLE "Signal" ADD COLUMN IF NOT EXISTS "sentiment"      text;
ALTER TABLE "Signal" ADD COLUMN IF NOT EXISTS "sentimentScore" double precision;
ALTER TABLE "Signal" ADD COLUMN IF NOT EXISTS "influence"      text;
ALTER TABLE "Signal" ADD COLUMN IF NOT EXISTS "relevance"      double precision;
ALTER TABLE "Signal" ADD COLUMN IF NOT EXISTS "language"       text;
ALTER TABLE "Signal" ADD COLUMN IF NOT EXISTS "originalTitle"   text;
ALTER TABLE "Signal" ADD COLUMN IF NOT EXISTS "originalSummary" text;
CREATE INDEX IF NOT EXISTS "Signal_region_idx"    ON "Signal"("region");
CREATE INDEX IF NOT EXISTS "Signal_geoScope_idx"  ON "Signal"("geoScope");
CREATE INDEX IF NOT EXISTS "Signal_sentiment_idx" ON "Signal"("sentiment");
CREATE INDEX IF NOT EXISTS "Signal_influence_idx" ON "Signal"("influence");

-- SignalAttribute is the normalized, extensible store for multi-valued/closed
-- dictionary attributes (industry, category, entity, …). valueCode holds the
-- canonical vocabulary code; valueText holds free-text values (e.g. entity name);
-- valueNum holds scalars. Every (key,valueCode) pair maps to the dictionary.
CREATE TABLE IF NOT EXISTS "SignalAttribute" (
  "signalId"   text NOT NULL REFERENCES "Signal"("id") ON DELETE CASCADE,
  "key"        text NOT NULL,
  "valueCode"  text NOT NULL DEFAULT '',
  "valueText"  text NOT NULL DEFAULT '',
  "valueNum"   double precision,
  "confidence" double precision NOT NULL DEFAULT 1,
  PRIMARY KEY ("signalId","key","valueCode","valueText")
);
CREATE INDEX IF NOT EXISTS "SignalAttribute_key_value_idx" ON "SignalAttribute"("key","valueCode");
CREATE INDEX IF NOT EXISTS "SignalAttribute_signal_idx"    ON "SignalAttribute"("signalId");
-- Entity lookups: exact-name filter and name search over the 'entity' attribute.
CREATE INDEX IF NOT EXISTS "SignalAttribute_entity_idx" ON "SignalAttribute"("valueText","valueCode") WHERE "key"='entity';

-- Full-text search. A generated tsvector column (weighted title > summary >
-- narrative) backed by a GIN index replaces the previous unindexed ILIKE scan;
-- queries rank with ts_rank and parse input with websearch_to_tsquery.
ALTER TABLE "Signal" ADD COLUMN IF NOT EXISTS "searchVector" tsvector
  GENERATED ALWAYS AS (
    setweight(to_tsvector('english', coalesce("title", '')), 'A') ||
    setweight(to_tsvector('english', coalesce("summary", '')), 'B') ||
    setweight(to_tsvector('english', coalesce("whatHappened", '') || ' ' || coalesce("whyItMatters", '')), 'C')
  ) STORED;
CREATE INDEX IF NOT EXISTS "Signal_search_idx" ON "Signal" USING gin ("searchVector");

ALTER TABLE "Article" ADD COLUMN IF NOT EXISTS "searchVector" tsvector
  GENERATED ALWAYS AS (
    setweight(to_tsvector('english', coalesce("title", '')), 'A') ||
    setweight(to_tsvector('english', coalesce("summary", '') || ' ' || coalesce("body", '')), 'B')
  ) STORED;
CREATE INDEX IF NOT EXISTS "Article_search_idx" ON "Article" USING gin ("searchVector");

-- EmailConnector is an admin-managed SMTP configuration ("connector") used by the
-- EMAIL delivery channel. The secret (password/API key) is stored only as
-- ciphertext (AES-GCM, same key as LLMKey); secretLast4 is safe to display.
CREATE TABLE IF NOT EXISTS "EmailConnector" (
  "id"               text PRIMARY KEY,
  "name"             text NOT NULL,
  "provider"         text NOT NULL DEFAULT 'CUSTOM',
  "host"             text NOT NULL,
  "port"             integer NOT NULL DEFAULT 587,
  "security"         text NOT NULL DEFAULT 'STARTTLS',
  "username"         text NOT NULL DEFAULT '',
  "secretCiphertext" text NOT NULL DEFAULT '',
  "secretLast4"      text NOT NULL DEFAULT '',
  "fromEmail"        text NOT NULL,
  "fromName"         text NOT NULL DEFAULT 'WorldSignal',
  "isActive"         boolean NOT NULL DEFAULT false,
  "enabled"          boolean NOT NULL DEFAULT true,
  "status"           text NOT NULL DEFAULT 'UNTESTED',
  "lastTestedAt"     timestamptz,
  "lastError"        text,
  "createdBy"        text,
  "createdAt"        timestamptz NOT NULL DEFAULT now(),
  "updatedAt"        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS "EmailConnector_active_idx" ON "EmailConnector"("isActive");

-- ApiKey secures the public REST API (/v1/*). Only a one-way SHA-256 hash of the
-- key is stored (the raw key is shown once at creation); keyPrefix is safe to
-- display. Each key carries a scope set and a per-minute rate limit.
CREATE TABLE IF NOT EXISTS "ApiKey" (
  "id"              text PRIMARY KEY,
  "name"            text NOT NULL,
  "keyHash"         text NOT NULL UNIQUE,
  "keyPrefix"       text NOT NULL,
  "scopes"          text[] NOT NULL DEFAULT '{}',
  "rateLimitPerMin" integer NOT NULL DEFAULT 120,
  "enabled"         boolean NOT NULL DEFAULT true,
  "expiresAt"       timestamptz,
  "lastUsedAt"      timestamptz,
  "requestCount"    bigint NOT NULL DEFAULT 0,
  "createdBy"       text,
  "createdAt"       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS "ApiKey_hash_idx" ON "ApiKey"("keyHash");

-- Fixed-window (per-minute) request counters for API-key rate limiting. One live
-- row per active key; stale windows are pruned on write.
CREATE TABLE IF NOT EXISTS "ApiKeyUsage" (
  "keyId"       text NOT NULL REFERENCES "ApiKey"("id") ON DELETE CASCADE,
  "windowStart" timestamptz NOT NULL,
  "count"       integer NOT NULL DEFAULT 0,
  PRIMARY KEY ("keyId","windowStart")
);

-- Digest support: batched email subscriptions accumulate matched signals here
-- instead of sending immediately; the digest scheduler drains this into one
-- rollup delivery per interval. lastDigestAt tracks when a subscription last fired.
ALTER TABLE "Subscription" ADD COLUMN IF NOT EXISTS "lastDigestAt" timestamptz;
CREATE TABLE IF NOT EXISTS "DigestQueue" (
  "subscriptionId" text NOT NULL REFERENCES "Subscription"("id") ON DELETE CASCADE,
  "signalId"       text NOT NULL REFERENCES "Signal"("id") ON DELETE CASCADE,
  "queuedAt"       timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("subscriptionId","signalId")
);
CREATE INDEX IF NOT EXISTS "DigestQueue_sub_idx" ON "DigestQueue"("subscriptionId","queuedAt");

-- Streaming subscriptions: SSE + WebSocket are pull-family channels. Delivery
-- rows are the durable log clients tail; "seq" is the monotonic, resumable
-- stream cursor. Adding it with a volatile default backfills existing rows.
CREATE SEQUENCE IF NOT EXISTS "DeliveryEvent_seq_seq";
ALTER TABLE "DeliveryEvent" ADD COLUMN IF NOT EXISTS "seq" bigint NOT NULL DEFAULT nextval('"DeliveryEvent_seq_seq"');
CREATE INDEX IF NOT EXISTS "DeliveryEvent_sub_seq_idx" ON "DeliveryEvent"("subscriptionId","seq");

-- Smart-signals relevance engine: a subscription carries a weighted interest
-- graph (dimension:value -> weight) used to rank its personalized "For You" feed;
-- feedback (open/up/down) is logged per subscription+signal for later learning.
ALTER TABLE "Subscription" ADD COLUMN IF NOT EXISTS "interests" jsonb NOT NULL DEFAULT '{}';
CREATE TABLE IF NOT EXISTS "SignalFeedback" (
  "subscriptionId" text NOT NULL REFERENCES "Subscription"("id") ON DELETE CASCADE,
  "signalId"       text NOT NULL REFERENCES "Signal"("id") ON DELETE CASCADE,
  "action"         text NOT NULL,
  "createdAt"      timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("subscriptionId","signalId","action")
);
CREATE INDEX IF NOT EXISTS "SignalFeedback_sub_idx" ON "SignalFeedback"("subscriptionId","createdAt" DESC);

-- Multi-tenancy spine. Account is a SaaS tenant that owns API keys, subscriptions
-- and (later) billing; the global signal corpus stays shared. Pre-multitenancy
-- rows are backfilled to a seeded default account so account-scoped foreign keys
-- always resolve. A platform-staff user has accountId = NULL.
CREATE TABLE IF NOT EXISTS "Account" (
  "id"        text PRIMARY KEY,
  "name"      text NOT NULL,
  "slug"      text NOT NULL UNIQUE,
  "status"    text NOT NULL DEFAULT 'ACTIVE',
  "plan"      text NOT NULL DEFAULT 'FREE',
  "createdAt" timestamptz NOT NULL DEFAULT now(),
  "updatedAt" timestamptz NOT NULL DEFAULT now()
);
INSERT INTO "Account" ("id","name","slug") VALUES ('acct_default','Default Account','default')
  ON CONFLICT ("id") DO NOTHING;

ALTER TABLE "ApiKey" ADD COLUMN IF NOT EXISTS "accountId" text NOT NULL DEFAULT 'acct_default';
CREATE INDEX IF NOT EXISTS "ApiKey_account_idx" ON "ApiKey"("accountId");
ALTER TABLE "User" ADD COLUMN IF NOT EXISTS "accountId" text;
CREATE INDEX IF NOT EXISTS "User_account_idx" ON "User"("accountId");

DO $migrate$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ApiKey_accountId_fkey') THEN
    ALTER TABLE "ApiKey" ADD CONSTRAINT "ApiKey_accountId_fkey"
      FOREIGN KEY ("accountId") REFERENCES "Account"("id") ON DELETE CASCADE;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'User_accountId_fkey') THEN
    ALTER TABLE "User" ADD CONSTRAINT "User_accountId_fkey"
      FOREIGN KEY ("accountId") REFERENCES "Account"("id") ON DELETE SET NULL;
  END IF;
END $migrate$;

-- Subscriptions are a customer-owned entity: each belongs to an Account (the
-- tenant). Pre-multitenancy rows backfill to the default account; the legacy
-- Subscriber entity is then removed entirely (below).
ALTER TABLE "Subscription" ADD COLUMN IF NOT EXISTS "accountId" text NOT NULL DEFAULT 'acct_default';
CREATE INDEX IF NOT EXISTS "Subscription_account_idx" ON "Subscription"("accountId");

DO $migrate2$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'Subscription_accountId_fkey') THEN
    ALTER TABLE "Subscription" ADD CONSTRAINT "Subscription_accountId_fkey"
      FOREIGN KEY ("accountId") REFERENCES "Account"("id") ON DELETE CASCADE;
  END IF;
END $migrate2$;

-- Remove the legacy Subscriber entity: Account is the sole owner of a
-- subscription now. Drop the FK + column, then the table and its enum.
ALTER TABLE "Subscription" DROP CONSTRAINT IF EXISTS "Subscription_subscriberId_fkey";
ALTER TABLE "Subscription" DROP COLUMN IF EXISTS "subscriberId";
DROP TABLE IF EXISTS "Subscriber" CASCADE;
DROP TYPE IF EXISTS "SubscriberStatus";`

// MigrateContent ensures the extended source-metadata columns and the
// SourceValidationLog table exist. Safe to run repeatedly.
func (d *DB) MigrateContent(ctx context.Context) error {
	if _, err := d.Pool.Exec(ctx, contentSchema); err != nil {
		return err
	}
	// New enum values must be added one statement at a time (not inside a
	// multi-command batch). Idempotent for DBs that already have them.
	for _, v := range []string{"SSE", "WEBSOCKET"} {
		if _, err := d.Pool.Exec(ctx, `ALTER TYPE "DeliveryChannel" ADD VALUE IF NOT EXISTS '`+v+`'`); err != nil {
			return err
		}
	}
	return nil
}

// trgmSchema enables trigram indexes for fuzzy/substring search. It is applied
// best-effort (see MigrateSearch) because CREATE EXTENSION needs a privilege that
// some managed Postgres roles lack; full-text search works without it.
const trgmSchema = `
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX IF NOT EXISTS "Signal_title_trgm_idx"  ON "Signal"  USING gin ("title" gin_trgm_ops);
CREATE INDEX IF NOT EXISTS "Signal_summary_trgm_idx" ON "Signal" USING gin ("summary" gin_trgm_ops);
CREATE INDEX IF NOT EXISTS "Article_title_trgm_idx" ON "Article" USING gin ("title" gin_trgm_ops);
CREATE INDEX IF NOT EXISTS "SignalAttribute_entity_trgm_idx" ON "SignalAttribute" USING gin ("valueText" gin_trgm_ops);`

// MigrateSearch enables pg_trgm and its trigram indexes. It is best-effort: the
// caller should log a failure (e.g. insufficient privilege to CREATE EXTENSION)
// but continue, since substring/entity search still works via a sequential scan
// and full-text search is unaffected. Returns the error for logging.
func (d *DB) MigrateSearch(ctx context.Context) error {
	_, err := d.Pool.Exec(ctx, trgmSchema)
	return err
}
