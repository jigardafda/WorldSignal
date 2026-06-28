# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

_Nothing yet._

## [0.1.0] - 2026-06-28

Initial public release.

### Added

- **Go backend** built on `net/http` with Postgres via pgx, providing the
  WorldSignal pipeline that converts public news and RSS/Atom feeds into
  deduplicated, enriched, classified Signals.
- **APIs:** a custom schemaless GraphQL executor plus a small REST surface.
- **Authentication and RBAC:** bearer session tokens, bcrypt password hashing,
  the `ADMIN`, `EDITOR`, and `VIEWER` roles, and the `settings:manage`
  permission.
- **Source catalog:** 1000+ validated global sources, with discovery,
  validation, and seeding via the `sourcetool` command
  (`backend/cmd/sourcetool`).
- **Automated ingestion:** a scheduler with concurrent workers and per-source
  cooldown, backed by a Postgres job queue (no Redis).
- **Enrichment:** OpenAI-based LLM enrichment with a deterministic heuristic
  fallback, plus LLM key management.
- **Audit log:** records of significant administrative and security-relevant
  actions.
- **Admin console:** a React + Vite + Mantine frontend for operating the
  system.
- **Operations:** a single `server` binary (`backend/cmd/server`) selectable as
  API and/or worker via `ROLE=all|api|worker`.

[Unreleased]: https://github.com/jigardafda/WorldSignal/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/jigardafda/WorldSignal/releases/tag/v0.1.0
