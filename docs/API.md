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
- `articles(limit, offset, sourceId, status, search) { items, total }` · `article(id)`
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

| Method | Path | Notes |
|---|---|---|
| GET | `/health` | Liveness probe (`{status:"ok"}`). |
| GET | `/v1/stats` | Headline counts. |
| GET | `/v1/sources` | `{ data: Source[] }`. |
| POST | `/v1/sources` | Create a source (also enqueues a fetch). |
| PATCH | `/v1/sources/{id}` | Patch enabled/priority/crawlFrequency. |
| POST | `/v1/sources/{id}/fetch` | Enqueue an immediate fetch. |
