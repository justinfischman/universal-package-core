# universal-package-core (`upc`)

One place to create, install and manage packages and apps across languages. `upc` routes to the native package managers (uv, poetry, pip, npm, yarn, ...). It never replaces them.

**Status: Phase 1 (foundation) complete.** Working today: `version`, `completion`, `config get|set|validate`, and the global flags. No language, package manager or project type is registered yet, so nothing routes to a package manager until Phase 2 (the MVP). The rest of this page describes intended behavior; see `plans/` for the build order.

## Goals
- Create easily installable apps and packages (installable by `upc`).
- Manage packages in other projects with one tool.

## How it works
- The native PM owns manifests, lockfiles, versions, sync and upgrades. `upc` reads live PM state and does not cache a competing copy.
- If a PM or runtime is missing, `upc` asks to install it, then continues.
- Language, PM and project-type code each live in their own package; adding a new one touches one package plus registration. See `docs/ARCHITECTURE.md`.

## Planned commands
`init`, `new {script|package|app}`, `add|remove|list|install`, `update|outdated|audit`, `run`, `build|test|lint`, `up|down`, `env`, `config`, `workspace`, `doctor`, `publish`, `completion`, `version`.

Global flags: `--dry-run`, `--json`, `--yes`, `--verbose`, `--project {path}`, `--global`. Non-interactive mode for CI.

First supported stack: Python with `uv`. Then Node with `npm`, then other PMs. First project type: `script`, then `package`, then `app`.

## Storage
Global `~/.universal-package-core/`, per-project `.universal-package-core/` (project wins). Filesystem only.

## Development
Requires Go 1.27+ and `golangci-lint` v2.
- `make build` builds `bin/upc` (version info via ldflags).
- `make check` runs gofmt, `go vet`, `golangci-lint` and all tests, including the architecture test and the testscript E2E in `test/`.
- Golden files: `go test ./internal/components/output -update` rewrites them; review the diff before committing.
- `UPC_HOME` overrides the global config directory, which keeps tests and experiments away from `~/.universal-package-core/`.

## Install
Homebrew only, for now *(planned, Phase 5)*.

## Documentation map
- `docs/ARCHITECTURE.md`: design and layering
- `docs/adrs/`: decisions
- `docs/CHANGELOG.md`: changes
- `plans/`: ordered phase plans
- `project.devlang`: source of truth for requirements
