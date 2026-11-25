# Contributing

## Build and test

Requires Go (version in `go.mod`) and `golangci-lint`.

```sh
make build            # builds bin/upc
scripts/check.sh      # gofmt, go vet, golangci-lint, go test ./... (the full local gate)
```

`make check` runs the same checks. `UPC_E2E=1 go test ./test/...` also runs the real-manager scripts (needs `uv` and network). Set `UPC_HOME` to keep experiments away from `~/.universal-package-core/`. More in [docs/README.md](docs/README.md) (Development) and [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Branches and commits

- Branch names: `feature/`, `fix/`, `hotfix/`, `chore/`, `refactor/` or `docs/` plus a short description.
- Commits use Conventional Commits: `<type>(<scope>): <summary>`, imperative, at most 72 characters, explaining why.
- Never force-push `main` or shared branches. Never commit secrets, `.env` or `.DS_Store`.

## Pull requests

- Keep the change in scope, add or update tests, and run `scripts/check.sh` before opening the PR.
- Update `docs/CHANGELOG.md` with every change, and the docs the change makes stale.
- Record architectural decisions as an ADR in `docs/adrs/`.

## Plans

Phase plans live in `plans/`. A plan is read-only while it is being executed; only its progress ledger may be updated. If the code and a plan disagree, record the difference in `docs/CHANGELOG.md` rather than editing the plan.

By contributing you agree your work is licensed under the [MIT License](LICENSE).
