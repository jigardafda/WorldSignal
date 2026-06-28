# WorldSignal Admin Console — Full Build Plan

Goal: a production-ready admin frontend exposing **all** backend capabilities, backed by
real auth/RBAC, with ≥95% test coverage (unit + integration + e2e) and complete
validation / loading / empty / error states.

**UI library:** [Mantine](https://mantine.dev). **Routing:** react-router-dom.
**Auth:** opaque bearer tokens + DB sessions; roles ADMIN/EDITOR/VIEWER + teams.

3-strike rule: if an item fails its gates 3 times, mark `[!]` and stop for review.

---

## Phase A — Backend: auth & RBAC

- [x] A1 DB migrations (`MigrateAuth`): User, Session, Team, TeamMember; `SeedDefaultAdmin`.
- [x] A2 `internal/auth`: bcrypt, token gen, role/permission matrix (96% cov).
- [x] A3 DB layer: user/session/team CRUD (`users.go`, `teams.go`).
- [x] A4 GraphQL: login, logout, me, permissions, users, user, createUser, updateUser,
      deleteUser, changePassword, teams, team, createTeam, deleteTeam, add/removeTeamMember.
- [x] A5 Identity middleware (Bearer → ctx) + per-resolver `authz()` everywhere.
- [x] A6 Tests: auth flow, user/team mgmt, authz-forbidden, validation, closed-DB +
      hidden-table errors. (Backend ~94.6%; ≥95% once Phase B fills the entity stub.)

## Phase B — Backend: expose every entity + analytics

- [x] B1 Sources: detail, update, delete, enable/disable, fetch, raw-item counts.
- [x] B2 Articles: list (filters/paging) + detail (signal links + source).
- [x] B3 RawItems: list (by source/status) + detail (raw payload).
- [x] B4 Signals: list paging/total, related articles, timeline.
- [x] B5 Subscriptions + Subscribers: full CRUD, delivery history.
- [x] B6 Deliveries: list (filters) + detail (payload, attempts) + retry.
- [x] B7 Taxonomy: tree + per-tag signal counts.
- [x] B8 Jobs/queue: list by queue/state, counts, retry/cancel.
- [x] B9 Analytics: counts over time, by severity/status/eventType/country, top sources,
      delivery success rates, ingestion throughput.
- [x] B10 Tests ≥95% for all new queries/mutations.

## Phase C — Frontend foundation (Mantine + router + auth)

- [x] C1 Mantine + react-router; AppShell, theme, nav, notifications.
- [x] C2 Typed GraphQL client w/ auth header + error normalization; auth context + storage.
- [x] C3 Login page; protected routes; user menu; role-gated nav. (Account page in D10.)
- [x] C4 Shared UI: DataTable, DetailCard, StatCard, charts, Loading/Empty/Error, dialogs.

## Phase D — Frontend pages (list / detail / analytics / management)

- [ ] D1 Dashboard  - [ ] D2 Signals  - [ ] D3 Sources  - [ ] D4 Articles
- [ ] D5 Raw items  - [ ] D6 Subscriptions + Subscribers  - [ ] D7 Deliveries
- [ ] D8 Taxonomy  - [ ] D9 Jobs/queue  - [ ] D10 Admin (Users/Teams/Roles/account)
- [ ] D11 Analytics

## Phase E — Quality gates

- [ ] E1 Frontend coverage ≥95% (Vitest) + typecheck clean.
- [ ] E2 Backend coverage ≥95% (Go) + build/vet/lint clean.
- [ ] E3 Playwright e2e: auth + every major workflow, green vs Go backend.
- [ ] E4 Validation / loading / empty / error states verified on every page.
