<!--
Thanks for contributing to WorldSignal!
Please fill out this template so reviewers can understand and verify your change.
-->

## Summary

<!-- What does this PR do, and why? Link any related issues, e.g. "Closes #123". -->

## Type of change

<!-- Mark all that apply with an "x". -->

- [ ] Bug fix (non-breaking change that fixes an issue)
- [ ] New feature (non-breaking change that adds functionality)
- [ ] Breaking change (fix or feature that changes existing behavior)
- [ ] Documentation
- [ ] Refactor / performance / tooling (no functional change)

## How was this tested?

<!-- Describe the tests you added/ran and any manual verification. -->

## Checklist

- [ ] My commits follow [Conventional Commits](https://www.conventionalcommits.org/).
- [ ] Backend: `gofmt`, `go vet`, and `golangci-lint` pass.
- [ ] Backend tests pass: `go test ./... -p 1` (against a Postgres test DB).
- [ ] Frontend: `npm run lint`, `npm run typecheck`, and `npm test` pass.
- [ ] New and changed code is covered by tests; coverage stays **≥95%**.
- [ ] Documentation (`docs/`, README, etc.) updated where relevant.
- [ ] No secrets are committed (`.env.local` is gitignored).
- [ ] I have read the [Contributing guide](../CONTRIBUTING.md).
