# Governance

WorldSignal uses a lightweight, maintainer-led governance model. The goal is to
keep decision-making fast and transparent while the project is young, and to
provide a clear path for the community to take on more responsibility over time.

## Roles

### Users

Anyone who uses WorldSignal. Users contribute by filing issues, joining
discussions, and helping others.

### Contributors

Anyone who contributes code, documentation, source catalog improvements, issue
triage, or reviews. Contributing does not require any formal status — open a
pull request and you are a contributor.

### Maintainers

Maintainers have write access and are responsible for reviewing and merging
contributions, triaging issues, shaping the roadmap, and upholding the
[Code of Conduct](CODE_OF_CONDUCT.md). Maintainers are listed in
[.github/CODEOWNERS](.github/CODEOWNERS) and are automatically requested for
review on relevant pull requests.

### Lead maintainer (BDFL-delegated)

The project currently follows a "benevolent dictator, delegated" model. The
lead maintainer, [@jigardafda](https://github.com/jigardafda), holds final
decision-making authority and delegates day-to-day authority to maintainers.
This concentration of authority is intended to be temporary; as the maintainer
group grows, decision-making will move toward consensus among maintainers.

## How decisions are made

- **Routine changes** (bug fixes, docs, well-scoped features) are decided
  through normal pull request review. At least one maintainer approval and green
  CI are required to merge.
- **Significant changes** (architecture, public API or schema changes, new
  dependencies, breaking changes) should start as a GitHub
  [Discussion](https://github.com/jigardafda/WorldSignal/discussions) or an
  issue so the approach can be agreed before implementation.
- **Consensus first.** Maintainers seek lazy consensus. If consensus cannot be
  reached, the lead maintainer makes the final call.

All technical discussion happens in public (issues, discussions, and pull
requests) so decisions and their rationale are discoverable.

## Becoming a maintainer

Maintainers are invited based on a sustained track record of quality
contributions and good judgment. Typical signals include:

- A history of merged, well-tested pull requests.
- Thoughtful, constructive code review and issue triage.
- Reliability and alignment with the project's direction and Code of Conduct.

Existing maintainers nominate candidates; the lead maintainer confirms the
invitation. Maintainers who become inactive for an extended period may be moved
to emeritus status to keep the active set accurate. This is not a judgment of
past contributions and does not preclude returning.

## Code ownership

Review responsibilities are encoded in
[.github/CODEOWNERS](.github/CODEOWNERS). Changes to paths with an assigned
owner require that owner's review.

## Changing this document

Amendments to this governance model are proposed via pull request and approved
by the lead maintainer.
