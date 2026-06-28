// Imported FIRST by every test file so config/env sees a valid environment
// before it is evaluated. Defaults target the local test Postgres.
process.env.DATABASE_URL =
  process.env.TEST_DATABASE_URL ||
  process.env.DATABASE_URL ||
  "postgresql://jigardafda@localhost:5432/worldsignal_test?schema=public";
process.env.WEBHOOK_SIGNING_SECRET = process.env.WEBHOOK_SIGNING_SECRET || "test-secret";
