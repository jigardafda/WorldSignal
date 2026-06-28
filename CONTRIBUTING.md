# Contributing to WorldSignal

Thanks for your interest in contributing! WorldSignal turns public news and
RSS/Atom feeds into deduplicated, enriched, classified **Signals** — the
durable asset is the Signal, not the article. This guide covers how to set up
your environment, the conventions we follow, and how to get a change merged.

By participating in this project you agree to abide by our
[Code of Conduct](CODE_OF_CONDUCT.md).

## Table of contents

- [Ways to contribute](#ways-to-contribute)
- [Project layout](#project-layout)
- [Development setup](#development-setup)
- [Running the test and check suite](#running-the-test-and-check-suite)
- [Branch, commit, and PR conventions](#branch-commit-and-pr-conventions)
- [Coding standards](#coding-standards)
- [Test and coverage expectations](#test-and-coverage-expectations)
- [Developer Certificate of Origin (optional sign-off)](#developer-certificate-of-origin-optional-sign-off)
- [Reporting bugs and requesting features](#reporting-bugs-and-requesting-features)
- [Security issues](#security-issues)

## Ways to contribute

- Fix bugs or improve performance.
- Improve documentation in `docs/`.
- Add or refine news/RSS sources in the source catalog.
- Triage issues and review pull requests.

New to the project? Look for issues labelled
[`good first issue`](https://github.com/jigardafda/WorldSignal/labels/good%20first%20issue)
— they are scoped to be approachable without deep familiarity with the
codebase.

## Project layout

- `backend/` — Go service (Go module `github.com/worldsignal/backend`).
  - `backend/cmd/server` — the API and workers. Behaviour is selected with
    `ROLE=all|api|worker`.
  - `backend/cmd/sourcetool` — source catalog discovery, validation, and
    seeding.
- `frontend/` — React + Vite + Mantine admin console.
- `docs/` — deeper reference docs (see below).

For architecture and design, start with the docs:

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- [docs/API.md](docs/API.md)
- [docs/DATABASE.md](docs/DATABASE.md)
- [docs/CONFIGURATION.md](docs/CONFIGURATION.md)
- [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md)
- [docs/RUNBOOK.md](docs/RUNBOOK.md)

## Development setup

### Prerequisites

- Go (version pinned in `backend/go.mod`)
- Node.js LTS and npm
- Docker and Docker Compose (for Postgres)

### 1. Start Postgres

```bash
docker compose up -d postgres
```

WorldSignal uses a Postgres-backed job queue (no Redis) and Postgres via pgx,
so a running database is required for most backend work and tests.

### 2. Run the stack

```bash
./dev.sh
```

`dev.sh` brings up the backend (API + workers) and the frontend dev server.
See [docs/CONFIGURATION.md](docs/CONFIGURATION.md) for environment variables.

### Secrets

Never commit secrets. Local configuration belongs in `.env.local`, which is
gitignored. See [SECURITY.md](SECURITY.md) for details.

## Running the test and check suite

Run all of these before opening a pull request.

### Backend (Go)

```bash
cd backend
gofmt -l .            # must print nothing
go vet ./...
golangci-lint run
go test ./... -p 1    # DB tests serialize; a Postgres test DB is required
```

The `-p 1` flag is required: database-backed tests serialize and must not run
in parallel against the same database.

### Frontend (React)

```bash
cd frontend
npm run lint
npm run typecheck     # tsc
npm test              # Vitest
npx playwright test   # end-to-end (optional locally; runs in CI)
```

## Branch, commit, and PR conventions

### Branches

- Create a topic branch off `main`, e.g. `feat/source-health-metrics` or
  `fix/dedup-edge-case`.
- Keep branches focused on a single logical change.

### Commits — Conventional Commits

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<optional scope>): <description>
```

Common types: `feat`, `fix`, `docs`, `refactor`, `perf`, `test`, `build`,
`ci`, `chore`. Examples:

```
feat(enrichment): add pluggable provider interface
fix(queue): release lease on worker panic
docs(api): document signal filters
```

### Pull requests

- Fill in the [pull request template](.github/PULL_REQUEST_TEMPLATE.md).
- Keep PRs small and reviewable; link the issue they close.
- Ensure CI is green — all tests, linters, and coverage gates must pass.
- A maintainer review is required before merge (see
  [GOVERNANCE.md](GOVERNANCE.md) and [.github/CODEOWNERS](.github/CODEOWNERS)).

## Coding standards

### Go

- Code must be `gofmt`-clean and pass `go vet` and `golangci-lint`.
- Prefer the standard library; the backend deliberately uses `net/http` and a
  small dependency surface.
- Handle errors explicitly; wrap with context where it aids debugging.

### TypeScript / React

- ESLint must pass with no errors; follow the Prettier-style formatting the
  project is configured for.
- `tsc` must pass with no type errors.
- Prefer Mantine components and existing patterns in `frontend/`.

## Test and coverage expectations

- New code must be tested. Both backend and frontend coverage are gated at
  **≥95%** in CI.
- Add tests alongside the code they cover.
- For database behaviour, add tests that run under `go test ./... -p 1`
  against a Postgres test database.

## Developer Certificate of Origin (optional sign-off)

Sign-off is optional but appreciated. If you would like to certify the
[DCO](https://developercertificate.org/), add a `Signed-off-by` line with
`git commit -s`:

```
Signed-off-by: Your Name <you@example.com>
```

## Reporting bugs and requesting features

- Bugs: open a [Bug report](https://github.com/jigardafda/WorldSignal/issues/new?template=bug_report.yml).
- Features: open a [Feature request](https://github.com/jigardafda/WorldSignal/issues/new?template=feature_request.yml).
- Questions and ideas: use
  [GitHub Discussions](https://github.com/jigardafda/WorldSignal/discussions).
  See [SUPPORT.md](SUPPORT.md).

## Security issues

Do not open a public issue for security vulnerabilities. Follow the process in
[SECURITY.md](SECURITY.md).
