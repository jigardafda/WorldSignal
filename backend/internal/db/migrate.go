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
