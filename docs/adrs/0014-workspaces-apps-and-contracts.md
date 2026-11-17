# ADR-0014: Config-only workspaces, apps and contracts

Status: Accepted 2026-11-15 (Phase 6.4 lands). Revised 2026-11-15 at the start of 6.4, after 6.2 (`-w`, config layering) and 6.3 (`up`/`down`, `runstate`) landed.

## Context
ADR-0007 (accepted, not edited) chose manager-native workspaces and deferred `workspace add|remove`, per-member overrides and multi-root discovery. Fullstack projects (an API plus a web app) need `upc` to start several apps and wire them together without a second package-dependency system.

Correction to ADR-0007: its text says `workspace list` and `-w` were built. They were not when it was written. `-w` and a minimal `workspace list` landed in 6.2; the `[workspace]`/`[apps]` behavior, `workspace check`, `up --all|--app` and contracts land in 6.4.

Starting point (6.3): `up`/`down` are capabilities for all five managers, with run state in `internal/components/runstate` keyed per project directory. 6.4 extends that key with the app name (Decision 7) and does not change the capability contract.

## Decision
1. **Config only.** `[workspace] members = [...]` and `[apps.<name>]` with `path`, `port`, `url`, `provides`, `consumes`, `contract`, `env`. There are no package dependencies between apps.
2. **Supersedes** the deferred items of ADR-0007 (workspace add|remove, per-member overrides, multi-root discovery) and partly its option B. Manager-native workspaces are still detected first.
3. **Who wins on disagreement.** The manager's native member list defines package membership; config `members` define upc apps. A path in both must match, or `workspace check` fails.
4. **Interpolation.** `${apps.api.url}` resolves only in config values and computed child env. An unresolved variable is an error, never an empty string.
5. **Orchestration.** `up --all` starts apps through capability `up`, ordered topologically by `consumes`/`provides`; `down` runs in reverse; a cycle is an error. If an app fails to start, apps already started in this run are stopped in reverse order (rollback) and the failure is reported. The orchestrator takes a runner interface so it is tested with a fake.
6. **Contract lock is committed.** The checker stores a normalized snapshot or hash at `.universal-package-core/contracts/<app>.lock`. Committed paths: `config.toml`, `contracts/*.lock`. Gitignored paths: `local.toml`, `run/`. `doctor` reports "contract missing" when no lock is committed; `workspace check` reports "changed" with a diff summary and never updates the lock; an explicit `--update` accepts the change.
7. **Per-app run state.** Each app keeps its own run directory under `.universal-package-core/run/`, so two apps in one workspace do not clash.
8. **Cross-app `doctor`** reports each of port collision, consumer without provider, contract missing, contract changed and unresolved variable as separate finding types.

## Consequences
- `ContractChecker` is an interface in `core`; `openapi` and `jsonschema` implementations live in separate packages.
- No manager package depends on the `workspace` package.
- Port collisions are checked statically from config; a runtime probe is advisory only.
- Rollout: a two-uv-app pilot (`api` provides, `web` consumes) locks the method before a Node member is added and before the checkers, `EnvResolver` and orchestrator are built in parallel.
- YAML contracts are read by `internal/contracts/document`, which parses them with `go.yaml.in/yaml/v3`, so a lock is built from the full parse. Anchors, aliases and merge keys are resolved; numbers take their canonical form (for example `1.50` becomes `1.5`); timestamps become RFC 3339 strings and binary values become strings; mapping keys must be strings. Multi-document files, empty files and duplicate keys are errors.
- Decision 3 (config `members` against the manager's native member list) is not yet enforced by `workspace check`: the managers expose no cheap native member listing. Follow-up.

## Related
- ADR-0007 (monorepos and workspaces): superseded in part by this ADR.
- ADR-0011 (config layering): the workspace root and app config layers.
- ADR-0015 (environments and groups): npm and yarn groups use workspace members under `.universal-package-core/groups/`.
- ADR-0016 (run lookup and `up`/`down`): `up` and run state are per app.
