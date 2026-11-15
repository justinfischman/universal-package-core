# Changelog

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Update with every change.

## [Unreleased]

### Added
- Pre-development documentation: `docs/README.md`, `docs/ARCHITECTURE.md`, ADRs 0001-0005.
- Phase plans in `plans/` (01 foundation through 05 environments/workspace/release).
- Project spec (`project.devlang`), launch task, Go module and `cmd/upc` placeholder committed as the baseline.

### Added (Phase 6.3: up, down, publish)
- `upc up` / `upc down` for uv, pip, poetry, npm and yarn: start the `up`, `dev` or `start` script in the background and stop it; `-w/--member` accepted. `CapUp`/`CapDown` are Filled for every manager (the Unsupported declarations are removed).
- `upc publish`: build, confirm (`--yes` required non-interactively), publish; global `--dry-run` prints the plan.
- `internal/components/runstate` (per-app `up.pid`, `up.log` and `provider` under `.universal-package-core/run/`; stale records cleaned) replacing `internal/core/runstate.go`; `execx.Controller` (`Start`, `Alive`, `Stop`, unix and windows implementations) and `execx.Ambient`.
- `core.RunSource` and `core.UpProvider`; `internal/stacks/just` (optional justfile recipes as the last `run` source) and `internal/stacks/compose` (`docker compose` fallback for `up`/`down`), wired in `internal/cli/runwire.go`.
- Tests: arch check confining `just`/`docker`/`compose` literals to their stack packages (`internal/arch/toolliterals.go`), `-w` honored by every capability command (`internal/cli/member_test.go`), lifecycle parity (`internal/register/lifecycle_parity_test.go`).
- ADR-0016 is Accepted.

### Added (Phase 6.4: workspace, apps and contracts)
- `[workspace]` and `[apps.<name>]` config (`path`, `port`, `url`, `provides`, `consumes`, `contract`, `env`) with `${apps.<name>.<field>}` interpolation; `upc workspace list`; `upc up|down --all|--app <name>` in dependency order with rollback; run state keyed per app.
- Contract checkers for OpenAPI and JSON Schema (`internal/contracts/`), with a JSON and YAML-subset reader that rejects anchors, aliases, tags and multi-document files.
- `upc workspace check [--update]`: compares each app's contract with its committed lock `.universal-package-core/contracts/<app>.lock`; fails on a missing lock or a changed contract; writes locks only with `--update`.
- `doctor` cross-app findings: `port-collision`, `consumer-without-provider`, `unresolved-variable`, `contract-missing`, `contract-changed`.
- ADR-0014 is Accepted. Known gap: config `members` are not compared with the manager's native member list.

### Known gaps (Phase 6.3 follow-ups, see ADR-0016)
- Native `publish --dry-run` per manager; offer to create an up script; run state under `-w` keyed off the member; config-defined scripts/`up` command not wired; Windows `Stop` leaves child processes and pid reuse is unguarded; `down` keeps `up.log` until the next `up`.
- Generated-justfile export is not built (deferred).

### Added (Phase 6, steps 0 and 6.0)
- Draft ADRs 0011-0016: config layering and secrets, neutral constraint mapper, data-driven stacks and tool-name confinement, config-only workspaces/apps/contracts, environments and dependency groups, run lookup with optional justfile and `up`/`down`. ADR-0006, 0007 and 0008 are not edited.
- CI: `e2e` job (`UPC_E2E=1` with uv, poetry, npm and yarn installed) and a Windows job running build, `go vet`, golangci-lint and tests (was build only).
- `docs/README.md`: supported platforms line.

### Added (Phase 5, slice 1: environments)
- ADR-0006 (environment model) and ADR-0007 (monorepos and workspaces; decision only).
- `upc env list|use|show` (`internal/cli/env.go`) and `internal/components/envs` (discovery of declared and `.env.<name>` environments, name and path validation; never reads env files).
- Config: `[environments.<name>] file`, `default_environment`, `active_environment` (validated); wrapped commands get `UPC_ENV=<name>` via `execx.WithEnv`; `doctor` checks the active environment exists.

### Changed (Phase 5, slice 1)
- `config.Config.Environments` is now `map[string]config.Environment` (was `map[string]string`); it had no consumers.

### Added (Phase 1: foundation)
- `internal/core`: `Module`, `Capability`/`Support`/`CapabilitySet`, `Request`/`Result`, error types with `ExitCode`, `PackageManager` with embedded `BasePackageManager`, `Language`, `ProjectType`, `PackageFile`, generic `Registry`, `coretest` fakes, and `contract` suites.
- `internal/components`: `execx` (runner, dry-run, fake), `logx`, `prompt`, `fsx` (atomic writes), `render`, `output`.
- `internal/config`: layered TOML store (`UPC_HOME`), validation (cross-language manager switches refused), project root discovery.
- `internal/cli`: root command with global flags, session wiring, `version`, `config get|set|validate`; `completion` is cobra's built-in.
- `internal/register` composition root (empty until Phase 2), `internal/arch` architecture test (ADR-0002).
- Makefile, `scripts/check.sh`, `.golangci.yml`, GitHub Actions CI, testscript E2E in `test/`.

### Added (Phase 2: MVP, Python + uv + script)
- `internal/stacks/python` (language, `pyproject.toml` PackageFile, PEP 723 detection) and `internal/stacks/python/uv` (package manager, capability table, Diagnoser).
- `internal/projects/script` project type with staged scaffold and rollback on failure.
- CLI: `new`, `init`, `add`, `remove`, `list`, `install`, `update`, `outdated`, `run` (one table-driven factory), `doctor`; missing-tool install prompt and re-run.
- `internal/register` registers `python`, `uv`, `script`.
- ADR-0008: script `run` lookup order.

### Added (Phase 3 slice C: package type, generic commands, manager switching)
- `internal/projects/packagetype` (`package` project type): the chosen manager creates `Dir/Name` natively (`uv init --package`, `poetry new --src`, pip fills a setuptools src layout); a neutral `LICENSE` placeholder and the language's `package` templates (`.gitignore`, `tests/`) are added; `core.Request.Name` carries the project name.
- CLI: `build`, `test`, `lint`, `audit` (passthrough arguments) through the capability factory.
- Config: `pm` alias for `package_manager`; `package_managers = [...]` list key. `config set pm` is limited to the project's language (`ErrIncompatible`, exit 2) and notes that manifests and lockfiles are not converted.
- `doctor`: drift findings when several managers' files match equally or the files point at a manager other than the configured one(s).
- Opt-in real-uv E2E testscripts in `test/testdata/e2e` (`UPC_E2E=1`): script lifecycle and `init` byte-identity.

### Changed (Phase 2)
- Architecture test: a package manager id is allowed anywhere in the tree of the language that owns it.
- Test fixtures and test names use neutral identifiers so the word scan stays strict outside the stack trees.

### Fixed
- `cmd/upc/main.go` now declares `package main` and wires registry, CLI and signal handling.
- `.gitignore` now ignores `/bin`, `/dist`, `*.test` and `coverage.out`. `/target` is kept because the editor writes `target/flycheck*` there.

### Deviations from `plans/01-foundation.md` (also folded into the plan after the fact, at the user's request)
- The architecture test lives in `internal/arch`, not `internal/core`: it must import `register`, which core is forbidden to do.
- `render` renders everything in memory, checks conflicts, then writes atomically per file and removes created files on failure, instead of staging in a temp directory.
- The forbidden-word scan covers language and package manager ids only. Project type ids (`script`, `package`, `app`) are ordinary words in core's vocabulary; the import rules cover project types.
- The `version` testscript runs by default (it needs no external tool). Tests that need real package managers will be opt-in via `UPC_E2E=1`.

### Added (Phase 3, slice A: fill helpers and pip)
- `internal/components/scripts`: language-neutral named-script lookup (ADR-0008 order) and command splitting; uv's `run` now uses it.
- `internal/stacks/python/shared`: command invocation, venv lookup, pytest/ruff fill plans, module checks with install offer, pip-JSON parsing, name validation; uv reuses it.
- `internal/stacks/python/pip`: package manager with a full capability table (native install/list/outdated, filled init/add/remove/update/run/build/test/lint/audit/publish); exact, idempotent, atomic `requirements.txt` edits that leave the file unchanged on failure.
- `core.DetectRanker`: managers rank overlapping markers; auto-detect keeps the strongest, logs the decision, asks or errors listing choices on a tie.
- `internal/register` registers `pip`; the Python language lists `uv` and `pip`.

### Changed (Phase 3, slice A)
- uv `test` and `lint` are Filled (pytest / ruff or the `test`/`lint` script) instead of Unsupported.
- A directory with only `requirements.txt` now resolves to pip instead of the language default.
- `python.Project` exposes `ToolTables` and `Dependencies`; `RequirementName`/`NormalizeName` added.
- Prompt test data uses neutral option names so the word scan stays strict.

### Added (Phase 3, slice D: poetry, marker rule, ADR-0009)
- `internal/stacks/python/poetry`: package manager (native init/install/add/remove/list/outdated/update/run/build/publish; filled test/lint/audit through `poetry run`), `[tool.poetry]` / `poetry.lock` detection with ranks 200 / 300. `internal/register` registers `poetry`; the Python language lists `uv`, `pip` and `poetry`.
- Architecture test: marker-file scan. Managers declare marker files (`uv.lock`, `poetry.lock`, `requirements.txt`); `core.Registries.MarkerFiles()` and `arch.CheckMarkers` allow them to be named only under the owning language's tree.
- ADR-0009: package-manager switching and detection ranking.
- `docs/ARCHITECTURE.md` section 12: capability table for uv, pip and poetry, project types, ranking, marker rule.

### Known gaps (Phase 3)
- `list --pm` does not exist. (`publish` CLI added in Phase 6.3.)
- A new pip `package` has no `.venv` until `upc install`; a new poetry `package` needs `upc install` before `upc test`.
- `plans/03-python-stacks-and-package-type.md` says a cross-language `config set pm` exits 1; the implementation exits 2 (`ErrIncompatible` is a usage error, ADR-0009). The plan is read-only and was not changed.

### Added (Phase 4, slice A: Node language and npm)
- `internal/stacks/node` (language `node`; detects `package.json`, `tsconfig.json`, `package-lock.json`, `yarn.lock`; TypeScript templates for `script` and `package`; read-only `package.json` `PackageFile`, `ScriptReader`) and `internal/stacks/node/shared`.
- `internal/stacks/node/npm`: package manager declaring every capability (native init/install/add/remove/list/update/outdated/run/audit/publish/test; filled build and lint; up/down unsupported at the time, filled in Phase 6.3), JSON parsing for `npm ls` and `npm outdated` (exit 1 with output is a result), `DetectRanker` (`package-lock.json` 300, `package.json` 100), `Diagnoser`. `internal/register` registers `node` and `npm`.
- ADR-0010: Node/TypeScript template and runtime choices (type stripping for scripts, `tsc` for packages, `node:test`, path for pnpm and bun).
- Tests: contract suite, argv per capability, recorded `npm ls`/`npm outdated` fixtures (empty, multi-location, error), detection ranking, scaffold goldens (`internal/register/testdata`), cross-language refusal testscript (`test/testdata/pm_cross_language.txtar`), opt-in real-npm E2E (`node_package_lifecycle`, `node_script_lifecycle`).
- Design defects in core, cli, components, config or the project types: none found. The `script` and `package` types scaffold Node unchanged.

### Changed (Phase 4, slice A)
- `test/testdata/pm_switch.txtar`: the "unknown manager" case uses a name that is not registered, since `npm` now is.
- A Python test fixture that named a Node marker file (`package.json`) uses a neutral file name, as the marker scan now covers it.

### Known gaps (Phase 4, slice A)
- `node/shared` duplicates the invocation and package-list helpers of the other language's shared package; promoting them to `internal/components` needs a coordinated change in both trees.
- A new Node package runs `npm install` during `upc new`, so it needs network access.

### Added (Phase 4, slice B: yarn)
- `internal/stacks/node/yarn`: yarn classic (1.22) manager with the npm capability set; `yarn --version` picks berry argv for update/audit/publish only (one FakeRunner case, no berry fixtures); list/outdated are Unsupported on berry. `DetectRanker`: `yarn.lock` 300, `packageManager` `yarn@...` 200. Registered in `internal/register`; `node` lists `yarn`.
- Runtime constants and script helpers (`FileArgs`, `StartCommand`, `ScriptSource`, engines) moved from `node/npm` to `node/shared`.
- ADR-0010 addendum (no new ADR). Tests: contract suite, argv, recorded `yarn list`/`yarn outdated` NDJSON fixtures (`ls_deps.ndjson` has one hand-added scoped entry), detection tie-break (`internal/register/node_test.go`), opt-in `yarn_package_lifecycle` E2E (skips when yarn is absent). Missing-tool prompt checked by hand with yarn off PATH (exit 4, install hint).

### Known gaps (Phase 4, slice B)
- yarn berry is argv-only for update/audit/publish; init, list, outdated and the E2E target classic.
- `yarn init` has no `pkg set`, so upc seeds the starting `package.json` itself (name, version, type, engines, scripts) and yarn keeps it; a new yarn script also gets yarn's `main` and `license` fields.
- Design defects outside the allowed paths: none found; no edits were made outside `internal/stacks/node`, `internal/register`, docs and `test/testdata`.
