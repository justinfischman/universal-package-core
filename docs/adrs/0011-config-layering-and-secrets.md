# ADR-0011: Config layering and secrets

Status: Accepted 2026-11-15 (Phase 6.2 landed; app-level `local.toml` is loaded; root `local.toml` and `config show --origin` deferred)

## Context
Config today is one project file. Environments (ADR-0015) and multi-app workspaces (ADR-0014) need layers: a global default, a workspace root, and per-app overrides, plus a place for values that must not be committed.

## Decision
1. **Hidden config directories.** Project config lives in `.universal-package-core/config.toml`; global config in `~/.universal-package-core/config.toml`. Secrets live in the gitignored `.universal-package-core/local.toml`.
2. **Precedence, highest first:** flag, app `local.toml`, app config, root `local.toml`, root config, global config.
3. **Merge key by key.** Scalars override, tables merge, lists replace. The lowest (most specific) level wins and the walk up fills gaps.
4. **Walk limits.** The upward walk stops at the git root or a workspace-root marker (the nearest ancestor with a manager-native or config workspace marker).
5. **No user env vars.** `upc` has no required environment variables. Computed env is injected into child processes only, through `execx.WithEnv`.
6. **`upc config show --origin`** prints each key's source layer.

## Note: narrowing of the spec
`project.devlang` says environment variables and secrets are "not our concern". `local.toml` and rejection of secret keys in committed config are a deliberate narrowing of that statement. `upc` still never reads or prints environment values; it only keeps secrets out of committed config.

## Consequences
- The `Workspace []string` field in `internal/config` is migrated to a typed form (the old list is accepted as `members`).
- Discarded errors in config loading (`p.Get`, `res.Set`) must be handled when layering is built.
- A layering test covers each precedence step and each merge rule.

## Related
- ADR-0014 (workspaces and apps): defines the workspace root and app layers this ADR merges.
- ADR-0015 (environments and groups): the `Environment` table merged key by key here.
- ADR-0004 (storage and config format): the TOML format this ADR layers.
