# ADR-0016: Run lookup with optional justfile, and `up`/`down`

Status: Draft (Phase 6; accepted when 6.3 lands)

## Context
ADR-0008 (accepted, not edited) fixes the `run` lookup order. Phase 6 adds an optional `just` source and fills `up`/`down` for every manager. Per ADR-0013, tool names must stay out of `core` and shared components.

## Decision
1. **Neutral lookup order, per stack:** file in project, config script, manifest script, `[project.scripts]`, then an optional `justfile` recipe run through `execx` (`just <recipe>`). If `just` is absent the source is skipped with a debug log; `just` is never required.
2. **Justfile is run-only.** Generated-justfile export is not built and is listed as deferred in `CHANGELOG.md`.
3. **`up` resolution:** configured `up` command, then `dev`/`start` script, then `docker-compose.yml`/`compose.yaml`; offer to create a script through the manager (never by editing manifests directly) if none exist.
4. **Run state.** `up` records pid and log under `.universal-package-core/run/` (per app in a workspace, ADR-0014). `down` stops the process (`compose down` for compose) and cleans stale pid files after a crash. A failed `up` leaves no orphan state.
5. **Tool-name confinement.** Core defines `RunSource` and `UpProvider`. `just` and `compose` each live in their own stack-style package (for example `internal/stacks/just`, `internal/stacks/compose`) registered in `register`; neither lives in `internal/components` or `internal/tools`.
6. **`publish`:** build, confirm (`--yes` required non-interactively), publish, native `--dry-run` where offered. Credentials are never stored.

## Consequences
- `CapUp`/`CapDown` become Filled for all five managers; the Unsupported declarations are removed.
- The extended arch test enforces confinement.
