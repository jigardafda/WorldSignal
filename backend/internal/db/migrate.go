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
CREATE INDEX IF NOT EXISTS "SignalAttribute_signal_idx"    ON "SignalAttribute"("signalId");`

// MigrateContent ensures the extended source-metadata columns and the
// SourceValidationLog table exist. Safe to run repeatedly.
func (d *DB) MigrateContent(ctx context.Context) error {
	_, err := d.Pool.Exec(ctx, contentSchema)
	return err
}
