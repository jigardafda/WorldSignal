# API Reference

Two surfaces: a **GraphQL** endpoint (primary) and a small **REST** surface.

- GraphQL: `POST /graphql` (also `GET /graphql` for queries). Body: `{query, variables, operationName}`.
- Auth: `Authorization: Bearer <token>` from the `login` mutation. The token is an
  opaque session token (stored in `Session`), not a JWT.
- Errors: returned in the GraphQL `errors` array. Resolver errors carry semantic
  prefixes — `unauthenticated`, `forbidden`, `validation`, `invalid credentials`.

## Authorization

Every resolver enforces a permission. Roles → permissions:

| Permission | VIEWER | EDITOR | ADMIN |
|---|:--:|:--:|:--:|
| `signals:read`, `sources:read`, `subscriptions:read`, `deliveries:read`, `jobs:read`, `analytics:read` | ✓ | ✓ | ✓ |
| `sources:write`, `subscriptions:write`, `deliveries:retry`, `jobs:manage` | | ✓ | ✓ |
| `users:manage`, `teams:manage`, `settings:manage` | | | ✓ |

## GraphQL operations

### Auth & account
- `login(email, password) { token, user }` — public; records `LOGIN`/`LOGIN_FAILED` audit.
- `logout` · `me { …permissions }` · `permissions` · `changePassword(oldPassword, newPassword)`

### Signals & content (`signals:read`)
- `signals(filter, limit, offset)` · `signal(id)` · `signalCount(filter)`
  - `filter.search` runs **ranked full-text search** (Postgres `tsvector` +
    `websearch_to_tsquery`, weighted title > summary > briefing) with a
    substring fallback; results are ordered by relevance. `filter.entity`
    restricts to signals mentioning a named entity.
- `entities(search, type, limit) { name, type, signalCount }` — distinct
  extracted entities (people/orgs/places) with mention counts, searchable by
  name and filterable by `entityType`.
- `articles(limit, offset, sourceId, status, search) { items, total }` — `search`
  is full-text + ranked. · `article(id)`
- `rawItems(…) { items, total }` · `rawItem(id)`
- `taxonomy` · `taxonomyStats { code, count }` · `analytics`

### Sources (`sources:read` / `sources:write`)
- `sources(search, country, region, language, scope, industry, orgType, sourceType, validationStatus, tag, enabled, limit, offset)`
- `sourceCount(<same filters>)` · `sourceCoverage { byRegion, byScope, byOrgType, byValidation, byIndustry, byCountry, bySourceType, byLanguage }`
- `source(id) { … validationLogs }`
- Mutations: `createSource`, `updateSource`, `deleteSource`, `setSourceEnabled`,
  `triggerFetch`, `revalidateSource`

### Subscriptions / deliveries
- `subscriptions` · `subscription(id)` · `subscribers` · `deliveries(…) { items, total }` · `delivery(id)`
- Mutations: `createSubscription`, `updateSubscription`, `deleteSubscription`,
  `createSubscriber`, `deleteSubscriber`, `retryDelivery` (`deliveries:retry`)
- Channels: `WEBHOOK` (HMAC-signed POST), `POLLING` (pull via `deliveries`), and
  `EMAIL`. Email subscriptions carry `config { to, connectorId?, mode, interval }`
  and can send instantly or as an hourly/daily **digest** — see [EMAIL.md](EMAIL.md).

### Admin — email connectors (`settings:manage`)
- `emailConnectors` (secrets masked to last 4) · `emailProviders` — SMTP presets
  (Gmail, Outlook, Zoho, SendGrid, Custom) with host/port/security + setup hints
- Mutations: `createEmailConnector(input)`, `updateEmailConnector(id, input)`,
  `setActiveEmailConnector(id)`, `testEmailConnector(id)`, `sendTestEmail(id, to)`,
  `deleteEmailConnector(id)`

### Jobs (`jobs:read` / `jobs:manage`)
- `jobs(queue, state, limit, offset) { items, total }` · `jobCounts` · `retryJob(id)`

### Admin — users & teams (`users:manage` / `teams:manage`)
- `users` · `user(id)` · `teams` · `team(id) { members }`
- Mutations: `createUser`, `updateUser`, `deleteUser`, `createTeam`, `deleteTeam`,
  `addTeamMember`, `removeTeamMember`

### Admin — LLM keys & audit (`settings:manage`)
- `llmKeys` (secrets masked to last 4) · `llmStatus { enabled, source, model, hasSystemKey, activeLabel }`
- `llmModels` — live chat-model list from the provider via the effective key
- `auditLogs(actor, action, targetType, search, limit, offset) { items, total }`
- Mutations: `createLLMKey(input)`, `setActiveLLMKey(id)`, `testLLMKey(id)`, `deleteLLMKey(id)`

## Pagination

High-cardinality lists (`signals`, `articles`, `rawItems`, `deliveries`, `jobs`,
`sources`, `auditLogs`) accept `limit`/`offset`; list+count pairs (e.g. `sources`
+ `sourceCount`) drive UI paging. Bounded admin lists (`users`, `teams`,
`subscriptions`, `subscribers`, `llmKeys`) return up to a server-side cap (500).

## REST surface

### Authentication (API keys)

Every `/v1/*` endpoint requires an **API key** (only `/health` is open). Admins
create keys under **API Keys** in the console; each key is stored **hashed**
(SHA-256 — the raw secret is shown once, at creation), carries a set of **scopes**,
and has a per-minute **rate limit**.

Send the key on each request as either header:

```bash
curl -H "Authorization: Bearer wsk_…" https://host/v1/signals
curl -H "X-API-Key: wsk_…"            https://host/v1/signals
```

- **401** — missing or unknown key.
- **403** — key is disabled, expired, or lacks the required scope.
- **429** — rate limit exceeded. Responses carry `X-RateLimit-Limit`,
  `X-RateLimit-Remaining`, and (on 429) `Retry-After`.

Scopes: `signals:read`, `sources:read`, `sources:write`, `subscriptions:read`,
`subscriptions:write`, `deliveries:read`, `stats:read`.

Admin management (GraphQL, `settings:manage`): `apiKeys`, `apiScopes`,
`createApiKey(input)` (returns the raw `key` once), `setApiKeyEnabled(id, enabled)`,
`deleteApiKey(id)`.

### Endpoints

| Method | Path | Scope | Notes |
|---|---|---|---|
| GET | `/health` | — (open) | Liveness probe (`{status:"ok"}`). |
| GET | `/v1/stats` | `stats:read` | Headline counts. |
| GET | `/v1/taxonomy` | `signals:read` | The closed classification vocabulary. |
| GET | `/v1/signals` | `signals:read` | `{ data: Signal[] }`. Filters incl. `search` (full-text), `entity`, `country`, `status`, `tags`, … |
| GET | `/v1/signals/{id}` | `signals:read` | A single signal aggregate. |
| GET | `/v1/entities` | `signals:read` | `{ data: [{name,type,signalCount}] }`. Params: `search`, `type`, `limit`. |
| GET | `/v1/sources` | `sources:read` | `{ data: Source[] }`. |
| POST | `/v1/sources` | `sources:write` | Create a source (also enqueues a fetch). |
| PATCH | `/v1/sources/{id}` | `sources:write` | Patch enabled/priority/crawlFrequency. |
| POST | `/v1/sources/{id}/fetch` | `sources:write` | Enqueue an immediate fetch. |
| GET | `/v1/subscriptions` | `subscriptions:read` | `{ data: Subscription[] }`. |
| POST | `/v1/subscriptions` | `subscriptions:write` | Create a subscription. |
| GET | `/v1/deliveries` | `deliveries:read` | `{ data: DeliveryEvent[] }`. |
