# Security Policy

We take the security of WorldSignal seriously. Thank you for helping keep the
project and its users safe.

## Supported versions

WorldSignal is pre-1.0 and under active development. Security fixes are applied
to the latest release and the `main` branch.

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |
| < 0.1   | :x:                |

## Reporting a vulnerability

**Please do not report security vulnerabilities through public GitHub issues,
discussions, or pull requests.**

Report privately using either of the following channels:

1. **GitHub Security Advisories (preferred).** Open a private report via
   [Report a vulnerability](https://github.com/jigardafda/WorldSignal/security/advisories/new).
2. **Email.** Send details to **security@worldsignal.dev**.

Please include as much of the following as you can:

- A description of the vulnerability and its impact.
- Steps to reproduce, or a proof of concept.
- Affected version, commit, or deployment configuration.
- Any suggested remediation.

## Response process and SLA

- **Acknowledgement:** within **3 business days**.
- **Initial assessment:** within **7 business days**.
- **Fix / mitigation timeline:** communicated after triage and prioritized by
  severity.

We will keep you informed of progress and coordinate a disclosure timeline with
you. With your permission, we are happy to credit you once a fix is released.

## Scope

In scope:

- The Go backend (`backend/`), including the GraphQL executor, REST surface,
  authentication and RBAC, the Postgres-backed job queue, scheduler, and
  workers.
- The React/Mantine admin console (`frontend/`).
- Source tooling (`backend/cmd/sourcetool`).

Out of scope:

- Vulnerabilities in third-party dependencies that are not exploitable through
  WorldSignal (please report those upstream).
- Findings that require a compromised host, privileged local access, or
  physical access.
- Reports generated solely by automated scanners without a demonstrated,
  exploitable impact.

### A note on SSRF

WorldSignal is a feed aggregator. Fetching content from operator-configured
sources and delivering signals to operator-configured webhooks means the
service makes outbound HTTP requests to URLs it is told to contact. This
server-side request behavior is **an intrinsic and accepted capability** of the
system, not a vulnerability in itself. Reports in this area should focus on
concrete privilege escalation, bypass of intended network/destination
restrictions, or leakage of internal data beyond the configured scope.

## Secrets and configuration

- **Never commit secrets** (API keys, tokens, credentials) to the repository.
- Local secrets and configuration belong in `.env.local`, which is gitignored.
- See [docs/CONFIGURATION.md](docs/CONFIGURATION.md) for how configuration and
  LLM keys are managed.

If you discover an exposed secret in the repository history, please report it
privately using the channels above so it can be rotated.
