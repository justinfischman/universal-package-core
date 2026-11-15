# ADR-0014: Config-only workspaces, apps and contracts

Status: Draft (Phase 6; accepted when 6.4 lands)

## Context
ADR-0007 (accepted, not edited) chose manager-native workspaces and deferred `workspace add|remove`, per-member overrides and multi-root discovery. Fullstack projects (an API plus a web app) need `upc` to start several apps and wire them together without a second package-dependency system.

Correction to ADR-0007: its text says `workspace list` and `-w` were built. They were not; no member code or `workspace` command exists in `internal/cli` or `cmd`. They are built in 6.2 (minimal) and 6.4.

## Decision
1. **Config only.** `[workspace] members = [...]` and `[apps.<name>]` with `path`, `port`, `url`, `provides`, `consumes`, `contract`, `env`. There are no package dependencies between apps.
2. **Supersedes** the deferred items of ADR-0007 (workspace add|remove, per-member overrides, multi-root discovery) and partly its option B. Manager-native workspaces are still detected first.
3. **Who wins on disagreement.** The manager's native member list defines package membership; config `members` define upc apps. A path in both must match, or `workspace check` fails.
4. **Interpolation.** `${apps.api.url}` resolves only in config values and computed child env. An unresolved variable is an error, never an empty string.
5. **Orchestration.** `up --all` starts apps through capability `up`, ordered topologically by `consumes`/`provides`; `down` runs in reverse; a cycle is an error.
6. **Contract lock is committed.** The checker stores a normalized snapshot or hash at `.universal-package-core/contracts/<app>.lock`. Committed paths: `config.toml`, `contracts/*.lock`. Gitignored paths: `local.toml`, `run/`. `doctor` reports "contract missing" when no lock is committed; `workspace check` reports "changed" with a diff summary and never updates the lock; an explicit `--update` accepts the change.
7. **Per-app run state.** Each app keeps its own run directory under `.universal-package-core/run/`, so two apps in one workspace do not clash.
8. **Cross-app `doctor`** reports each of port collision, consumer without provider, contract missing, contract changed and unresolved variable as separate finding types.

## Consequences
- `ContractChecker` is an interface in `core`; `openapi` and `jsonschema` implementations live in separate packages.
- No manager package depends on the `workspace` package.
