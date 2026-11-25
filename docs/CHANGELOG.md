# Changelog

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Update with every change.

## [0.1.0] - 2026-11-17
Tagged `v0.1.0`; installable with `brew install justinfischman/upc/upc` (`docs/RELEASING.md`). The Homebrew tap is this repository (`Formula/`).

### Changed
- Process control for `up`/`down` uses gopsutil (liveness, start time, whole-tree terminate then kill) instead of `ps`, process-group signals and `taskkill`; only `detach` stays per OS. The CI `check` job runs on Linux only (the macOS runner queue was too slow); Windows and the e2e job were unchanged in 0.1.0 (the Windows job was enabled in 0.2.0, see Unreleased).

### Added
- 6.7 hardening: `upc init` adds `.universal-package-core/run/` to `.gitignore` (created if missing, never duplicated); `scripts.<name>` and `up` argv lists in config (`internal/config`) outrank manager sources for `run` and `up` (ADR-0016 amendment); the run state pid file records the process start time (`execx.Controller.StartedAt`) so a reused pid is never treated as the app or signalled; Windows `Stop` kills the process tree (`taskkill /T /F`); `down` removes only the pid file and keeps `up.log` until the next `up`; `cmd/upc` smoke test (version, help, unknown command exit codes).
- 6.2 gap: `upc env add|remove`, `--env <name>` on `install`, `run`, `test` and `up` (one command, selection not saved), environment `groups` installed by `install --env` for uv, poetry and pip (`core.Request.Groups`). npm/yarn group install (ADR-0015 option B), `doctor` group checks and `local.toml` / `config show --origin` are still open.
- `app` project type (6.5), see below.
- Environment drift in `doctor` for every manager (uv, pip, poetry, npm, yarn), file-based.
- `upc completion` for bash, zsh, fish and powershell, with tests (`docs/COMPLETION.md`).
- Release pipeline: `.goreleaser.yaml`, `.github/workflows/release.yml`, snapshot build in CI, `docs/RELEASING.md`.
- Four acceptance scripts `test/testdata/e2e/acceptance_*.txtar` (run with `UPC_E2E=1`).

### Changed
- Exit-code discrepancy with plan 03: `plans/03-python-stacks-and-package-type.md` says `config set pm` to a manager of another language exits 1; the implementation exits 2 (`core.ErrIncompatible` maps to `ExitUsage`, ADR-0009). Code is authoritative; the read-only plan was not changed (verified by `TestConfigSetRefusesCrossLanguageSwitch`).

### Deferred
- Windows support (backlog in 0.1.0; resolved in 0.2.0, see Unreleased and ADR-0017): the CI job and release build are disabled. The Windows test suite has about 30 failing tests (fake-fs paths, `0755` mode expectations, `upc.exe` lookup in testscript, temp-dir cleanup locks, doctor golden paths); `execx` already has `detach_windows.go`.
- pnpm, bun and generated-justfile export (done in 0.2.0, see Unreleased); other languages (ADR-0019 design and an unregistered Go spike only).
- From 6.2: all done in 0.2.0 (npm/yarn groups, `doctor` group checks for every manager, root-level `local.toml`, `config show --origin`); see Unreleased.
- From 6.7: native `publish --dry-run`, offer to create an up script, workspace-member comparison and app sub-types (CLI, library) are done in 0.2.0 (see Unreleased). Still deferred: `--app X` with dependencies, `${...}` escape syntax.
- Release: none; `v0.1.0` was tagged and the `brew install` check passed once the repository was public.

## [Unreleased] - 0.2.0 (Plan 07)

### Added
- `upc config show --origin`: shows the layer each value came from.
- Workflow commands: `publish --native-dry-run` (build, then the manager's own publish check; sends nothing), `up` offers to create an up script, `workspace check` compares config `members` with the manager's native member list, `up|down --app`.
- Native dry-run and member listing for every manager except the exceptions below.
- `upc export`: writes the project's commands as a justfile of `upc` recipes (kept unless `--force`).
- pnpm and bun package managers, with dependency groups, drift and capability parity tests.
- `upc new app --kind cli|library` for Node and Python (default: a service).
- Dependency groups for npm and yarn, root-level `local.toml`, and `doctor` group checks for uv, poetry, pip, npm, yarn, pnpm and bun.
- Windows support: the CI matrix runs ubuntu, macos and windows; goreleaser builds Windows archives (amd64, arm64) as `.zip`; process handling uses gopsutil (ADR-0017).
- Supply chain: an SPDX SBOM per archive (syft), `scripts/install.sh` that verifies the archive's sha256 against `checksums.txt` before installing, all GitHub Actions pinned by commit SHA, dependabot for Actions.
- ADR-0019 (additional language stacks) and an unregistered Go stack spike.

### Known limits
- yarn 2/3 and classic: installing a custom dependency group is a full install (`workspaces focus` is built in only from yarn 4; ADR-0015).
- yarn classic refuses `publish --dry-run` (no such flag).
- pip has no native publish dry-run (twine has none) and no member lister (pip has no workspaces); poetry has no member lister (no workspaces).
- Scoop and winget manifests are not included.
- cosign signing is deferred until the repository is public.
- Windows support is unverified until the first CI run on `windows-latest`.
- Not verified here: the SBOM step end to end (no syft), `install.sh` against real GitHub URLs, shellcheck, and pnpm parsers beyond fixtures.

### Known gaps
- Exit-code mismatch with plan 03: `plans/03-python-stacks-and-package-type.md` (line 40) says a cross-language `config set pm` exits 1. The code exits 2: `core.ExitCode` maps `ErrIncompatible` and `ErrInvalidRequest` to `ExitUsage` (`internal/core/errors.go:71`). Code is authoritative and ADR-0009 records 2; `plans/README.md` and the plan are unchanged.

### Added
- Root `README.md`, `LICENSE` (MIT), `SECURITY.md`, `CONTRIBUTING.md` and `docs/INSTALL.md` (upgrade, uninstall, `go install`).
- Pre-development documentation: `docs/README.md`, `docs/ARCHITECTURE.md`, ADRs 0001-0005.
- Phase plans in `plans/` (01 foundation through 05 environments/workspace/release).
- Project spec (`project.devlang`), launch task, Go module and `cmd/upc` placeholder committed as the baseline.

### Added (Phase 6.5: app project type)
- `upc new app NAME --language python|node --pm uv|pip|poetry|npm|yarn`: scaffolds a minimal server with `Dockerfile` and `compose.yaml` from each stack's `templates/app`; Python apps get `[tool.upc.scripts]` `dev`/`start`, Node (TypeScript) apps get `dev`, `build`, `start`. `upc up [port]` / `upc down` run it. E2E: `uv_app_lifecycle`, `node_app_lifecycle`.

### Added (Phase 6.3: up, down, publish)
- `upc up` / `upc down` for uv, pip, poetry, npm and yarn: start the `up`, `dev` or `start` script in the background and stop it; `-w/--member` accepted. `CapUp`/`CapDown` are Filled for every manager (the Unsupported declarations are removed).
- `upc publish`: build, confirm (`--yes` required non-interactively), publish; global `--dry-run` prints the plan.
- `internal/components/runstate` (per-app `up.pid`, `up.log` and `provider` under `.universal-package-core/run/`; stale records cleaned) replacing `internal/core/runstate.go`; `execx.Controller` (`Start`, `Alive`, `Stop`, unix and windows implementations) and `execx.Ambient`.
- `core.RunSource` and `core.UpProvider`; `internal/stacks/just` (optional justfile recipes as the last `run` source) and `internal/stacks/compose` (`docker compose` fallback for `up`/`down`), wired in `internal/cli/runwire.go`.
- Tests: arch check confining `just`/`docker`/`compose` literals to their stack packages (`internal/arch/toolliterals.go`), `-w` honored by every capability command (`internal/cli/member_test.go`), lifecycle parity (`internal/register/lifecycle_parity_test.go`).
- ADR-0016 is Accepted.

### Added (Phase 6.4: workspace, apps and contracts)
- `[workspace]` and `[apps.<name>]` config (`path`, `port`, `url`, `provides`, `consumes`, `contract`, `env`) with `${apps.<name>.<field>}` interpolation; `upc workspace list`; `upc up|down --all|--app <name>` in dependency order with rollback; run state keyed per app.
- Contract checkers for OpenAPI and JSON Schema (`internal/contracts/`), with a JSON and YAML reader built on `go.yaml.in/yaml/v3` (anchors, aliases and merge keys are resolved; numbers take their canonical form; multi-document files, duplicate keys and non-string keys are rejected).
- `upc workspace check [--update]`: compares each app's contract with its committed lock `.universal-package-core/contracts/<app>.lock`; fails on a missing lock or a changed contract; writes locks only with `--update`.
- `doctor` cross-app findings: `port-collision`, `consumer-without-provider`, `unresolved-variable`, `contract-missing`, `contract-changed`.
- ADR-0014 is Accepted. Known gap: config `members` are not compared with the manager's native member list.

### Known gaps (Phase 6.3 follow-ups, see ADR-0016)
- Native `publish --dry-run` per manager; offer to create an up script; run state under `-w` keyed off the member. (Closed in 6.7: config-defined scripts and `up` command, Windows process-tree kill and pid-reuse guard, `down` keeping `up.log`.)
- Generated-justfile export is not built (deferred).

### Added (Phase 6, steps 0 and 6.0)
- ADRs 0011-0016 (written as drafts in step 0, all Accepted by 6.8): config layering and secrets, neutral constraint mapper, data-driven stacks and tool-name confinement, config-only workspaces/apps/contracts, environments and dependency groups, run lookup with optional justfile and `up`/`down`. ADR-0006, 0007 and 0008 are not edited.
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
