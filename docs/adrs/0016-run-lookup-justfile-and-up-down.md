# ADR-0016: Run lookup with optional justfile, and `up`/`down`

Status: Accepted (implemented in Phase 6.3; follow-ups below)

## Context
ADR-0008 (accepted, not edited) fixes the `run` lookup order. Phase 6 adds an optional `just` source and fills `up`/`down` for every manager. Per ADR-0013, tool names must stay out of `core` and shared components.

## Decision
1. **Neutral lookup order, per stack:** file in project, config script, manifest script, `[project.scripts]`, then an optional `justfile` recipe run through `execx` (`just <recipe>`). The manager resolves its own sources; only when it reports the script as not declared does `internal/cli/runwire.go` try the registered `RunSource`s. If there is no justfile or `just` is absent the source reports no recipes and the project behaves as if it did not exist; `just` is never required. (The "config script" step is not wired yet, follow-up d.)
2. **Justfile is run-only.** Generated-justfile export is not built and is listed as deferred in `CHANGELOG.md`.
3. **`up` resolution:** configured `up` command, then an `up`, `dev` or `start` script (`scripts.UpNames`) started through the manager, then a registered `UpProvider` (compose: `docker-compose.yml`/`compose.yaml`); offering to create a script through the manager (never by editing manifests directly) when none exist is not built (follow-ups b, d).
4. **Run state.** `internal/components/runstate` records `up.pid` and `up.log` under `<app dir>/.universal-package-core/run/` (per app in a workspace, ADR-0014), plus a `provider` file when an `UpProvider` started the app. `down` stops through the recorded provider (`docker compose down` for compose), otherwise stops the recorded process through `execx.Controller`, and cleans stale records after a crash. A failed `up` leaves no orphan state.
5. **Tool-name confinement.** Core defines `RunSource` and `UpProvider`. `just` and `compose` each live in their own stack-style package (for example `internal/stacks/just`, `internal/stacks/compose`) registered in `register`; neither lives in `internal/components` or `internal/tools`.
6. **`publish`:** build, confirm (`--yes` required non-interactively), publish. Global `--dry-run` prints the plan; native per-manager dry-run is follow-up a. Credentials are never stored.

## Consequences
- `CapUp`/`CapDown` are Filled for all five managers; the Unsupported declarations are removed.
- The extended arch test (`internal/arch/toolliterals.go`) enforces confinement.

## Follow-ups
Not blocking acceptance; carry to 6.7 or later (plans/06, Phase 6.3 step 6).
- a. Native `publish --dry-run` per manager (today the global `--dry-run` prints the plan only).
- b. Offer to create an up script when none exists.
- c. Run state under `-w` keys off the request directory, not the member.
- d. `internal/config` has no script/up settings, so the "config script" and "configured up" lookup steps are not wired.
- e. Windows `Stop` does not kill child processes and pid reuse is unguarded.
- f. `down` keeps `up.log` until the next `up`.
