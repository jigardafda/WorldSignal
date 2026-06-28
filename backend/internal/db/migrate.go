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

CREATE INDEX IF NOT EXISTS "Source_language_idx"   ON "Source"("language");
CREATE INDEX IF NOT EXISTS "Source_region_idx"     ON "Source"("region");
CREATE INDEX IF NOT EXISTS "Source_industry_idx"   ON "Source"("industry");
CREATE INDEX IF NOT EXISTS "Source_scope_idx"      ON "Source"("geographicScope");
CREATE INDEX IF NOT EXISTS "Source_validation_idx" ON "Source"("validationStatus");
CREATE INDEX IF NOT EXISTS "Source_tags_idx"       ON "Source" USING gin ("tags");

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
CREATE INDEX IF NOT EXISTS "SourceValidationLog_source_idx" ON "SourceValidationLog"("sourceId","checkedAt" DESC);`

// MigrateContent ensures the extended source-metadata columns and the
// SourceValidationLog table exist. Safe to run repeatedly.
func (d *DB) MigrateContent(ctx context.Context) error {
	_, err := d.Pool.Exec(ctx, contentSchema)
	return err
}
