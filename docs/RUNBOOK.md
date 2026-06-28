# Operational Runbook

Common operational tasks and incident playbooks.

## Health checks

- `GET /health` → `200 {"status":"ok"}`.
- Quick data check (authenticated): GraphQL `{ stats }`.
- Boot logs print the role and LLM mode: `starting WorldSignal (role=…, llm=openai|heuristic-fallback)`.

## A source keeps failing

Symptoms: a source shows a high **Fails** count / no recent **Last success**.

1. Open **Sources → the source → Validation history**, or query `source(id){validationLogs}`.
2. Click **Revalidate** (or `mutation { revalidateSource(id) }`) to re-check live and
   capture the exact error (HTTP status / parse error / staleness).
3. If the feed URL is dead/redirected, edit the source URL to a working feed and
   revalidate. Bulk re-discovery/validation: `cmd/sourcetool` (`validate` then `seed`).

## LLM enrichment is disabled or failing

1. **Settings** shows the effective status (source ENV/DB/none, model, enabled).
2. **Add OpenAI key** (encrypted at rest, validated on save) and **Set active** — it
   overrides the env key at runtime (≤30s cache). Use **Test** to re-validate.
3. With no valid key, enrichment automatically falls back to the heuristic — the
   pipeline keeps running; signals are produced with lower-fidelity classification.
4. The model dropdown is populated live from the provider; an empty list means no
   working key is configured.

## Jobs are backing up

1. **Jobs** page (or `{ jobCounts }`) shows counts by state; failed jobs show
   `lastError`. Retry individually via **Retry** / `mutation { retryJob(id) }`.
2. Ensure at least one `ROLE=worker` (or `all`) instance is running and connected to
   the same `DATABASE_URL`.
3. Scale the worker tier if the active/queued backlog grows persistently.

## Rotating the signing secret / LLM key

- `WEBHOOK_SIGNING_SECRET` also derives the LLM-key encryption key. Rotating it makes
  stored admin LLM keys undecryptable — after rotation, delete and re-add LLM keys.
- To rotate a leaked OpenAI key: revoke it at the provider, then update `.env.local`/
  env (system key) or replace the admin key in **Settings**.

## Auditing

- **Audit Log** (admin) records logins (incl. failures), user/team/source/LLM-key
  changes. Filter by action/actor/target or search; paginated server-side.

## Backup & restore

- Standard Postgres `pg_dump`/`pg_restore`. Schema migrations are idempotent, so a
  restored DB upgrades automatically on next boot.

## First-run / fresh install

1. Boot with `DATABASE_URL` set → migrations run, default admin is seeded
   (`ADMIN_EMAIL`/`ADMIN_PASSWORD`).
2. Log in, **change the admin password**, add users/teams.
3. (Optional) add an LLM key in **Settings**; seed sources via `cmd/sourcetool`.
