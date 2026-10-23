# Phase 5: Environments, up/down, workspace, `app`, publish, release

**Depends on:** Phase 4 complete.
**Goal:** finish the remaining spec features and ship via Homebrew. The two open decisions (environment model, monorepo strategy) are made here, first, as ADRs.

## Step 0: decisions (ADRs before code)

### ADR-0006 Environment model (the spec leaves this to us)
Evaluate and choose, writing the trade-offs:
- **A. `.env.<name>` files** (`.env.local`, `.env.dev`, `.env.test`, `.env.prod`) discovered by convention.
- **B. Mapping in the hidden project config**: `[environments.dev] file = ".env.dev"`, plus `default = "local"`.
- Recommendation to validate in the ADR: **B with convention fallback**: names/files declared in `.universal-package-core/config.toml`; if absent, discover `.env.*` by convention. Honors "many ways to manage a project, none cause issues".
- Boundaries from the spec: `upc` does not handle env var values or secrets. It only records *which environment is active* and passes that name/file path to the wrapped command (e.g. sets `UPC_ENV=<name>` and, only where the underlying tool takes a file argument natively, forwards the path). It never reads or prints values.
- "Environment never out of line between PM and upc" = the **virtual environment** and active-env selection stay consistent with the PM: `doctor` checks that the PM's venv (`.venv`, `node_modules`) exists, matches the lockfile (`uv sync --check`/`npm ci --dry-run` or equivalent), and that the active upc env exists in config. Active env stored per project in config (selected state, not PM state).

### ADR-0007 Monorepos / multiple apps in one folder
- Detect PM-native workspaces first and defer to them (`uv workspace`, npm/yarn `workspaces`).
- `upc workspace` adds a thin layer: `list` members (read from PM live state), `--member/-w <name>` targeting on every capability command, and `.universal-package-core` config per member overriding the root.
- "Multiple apps in one folder easy": allow multiple project roots under one parent, discovered by marker files; commands accept `--project` or `-w`; `upc workspace add <path>` records members in the root config only for non-PM-native layouts.

## Deliverables (pilot order: env -> up/down -> workspace -> app -> publish -> release)

1. **`env`** (`internal/cli` generic + `internal/components/envs`): `env list|use|show`. Reads config; `use` validates the environment exists and writes the active selection. Pilot on Python/uv, then Node by recipe.
2. **`up` / `down`**: new capabilities already in `AllCapabilities` since Phase 1 (declared Unsupported in earlier phases). Now fill:
   - Resolution order: explicit `[scripts]`/`[commands] up` in config > detected `dev`/`start` script in the PM manifest (read via PackageFile) > `docker-compose.yml`/`compose.yaml` (`docker compose up`) > if none exist, offer to **create** a script (per spec: "creates them if none exist") through the PM natively (`npm pkg set scripts.dev=...`, `[tool.upc.scripts]`/ `uv` project script).
   - `up`: starts in the background with a pid/log record under the project hidden dir (`.universal-package-core/run/`), `down` stops it (docker: `compose down`). Logged and cleaned up on failure; no orphan state.
   - Active environment is applied (section ADR-0006).
   - Compose is a capability of a **tool module** (`internal/stacks`-style sibling `internal/tools/docker`? decide in the pilot, ADR): it must follow the isolation rule: docker knowledge in exactly one package.
3. **`workspace`**: per ADR-0007: `list`, `add`, `remove`, and `-w` targeting plumbed through the generic command factory (one change in one place).
4. **`app` project type** (`internal/projects/app`): scaffolds a runnable application (Python: `uv init --app`; Node: TS app with `dev`/`start`/`build` scripts) plus optional Dockerfile/compose templates supplied by the language package. Sub-types (CLI, library) map onto script/package/app per spec; implemented by flags, not new top-level types.
5. **`publish`**: `build` then hand off to the PM publish (`uv publish`, `poetry publish`, `npm publish`; twine fill for pip). Requires explicit confirmation (outward-facing); `--dry-run` supported natively where the PM offers it; never stores credentials.
6. **Release & distribution (Homebrew only)**:
   - `.goreleaser.yaml`: darwin/linux (amd64/arm64) builds, ldflags for `version` command, checksums, and a Homebrew tap formula (`brews`/ homebrew_casks per the current goreleaser docs; goreleaser is installed locally; run `goreleaser check` against its schema when adding the file).
   - Tap repo `homebrew-tap` and a `release.yml` GitHub Action with a tag trigger. Tap token via CI secret (never committed).
   - Windows binary build is produced for portability but not distributed via Homebrew (spec: Homebrew only for now); note in README.
   - `upc completion` ships in the formula (`generate_completions_from_executable`).
   - Disk footprint check: single static binary, `-trimpath -ldflags "-s -w"`, no runtime assets outside `~/.universal-package-core/`.

## Tests
- env: config parse/validate, `use` persistence, missing env error, convention discovery fixtures.
- up/down: `FakeRunner` with scripted background process; resolution order table; created-script path; `down` after crash; no leftover run files on failure.
- workspace: uv and npm workspace fixtures; targeting; multi-root discovery.
- app scaffold goldens (python, node); publish confirmation + dry-run (never actually publishes in tests; fake PM).
- Parity suite updated: `up`/`down` now Native/Filled for every PM, or explicit Unsupported with a reason (no PM may silently lack it).
- Release: `goreleaser check`, `goreleaser release --snapshot --clean` in CI; formula `brew audit --strict --new` and `brew install --build-from-source` smoke test on macOS.
- Acceptance E2E (opt-in) mapping each `project.devlang` acceptance bullet to a testscript.

## Documentation & Testing
`ARCHITECTURE.md` (final command/phase table, env/workspace design), ADR-0006, ADR-0007 (written first), `docs/README.md` (Homebrew install, env usage, up/down), `CHANGELOG.md` release entry `0.1.0`, release process doc `docs/RELEASING.md`.

## Integration validation
Full `make check`; all opt-in E2E; fresh-machine style run: `brew install <tap>/upc` from the snapshot formula, then run the four acceptance scenarios. Verify `doctor` reports no drift after each.

## Exit criteria
- All four acceptance statements in `project.devlang` demonstrated by automated or documented manual runs.
- `0.1.0` installable via Homebrew.
- All docs match code; final post-change audit and a list of deferred items (pnpm, bun, other languages) recorded in `CHANGELOG.md`.
