# Architecture

Status: **Phases 1 to 4 and 6.0 to 6.7 built** (config layering, environments and groups, `up`/`down`/`publish`, workspaces, `app`, drift, hardening; the `v0.1.0` release awaits owner authorization and 6.8 is the final audit) (core, components, config, CLI, and the MVP stack: Python + uv + `script`). This document describes the design from `project.devlang`. Sections marked *(planned)* are not built yet; see `plans/` for the phase that builds each one. Update this file with every change.

## 1. Purpose and boundaries

`upc` is a CLI that creates, installs and manages packages and apps across languages. It **routes** to native package managers (PMs) and **sets up** projects. It never replaces a PM.

- The native PM owns manifests, lockfiles, versions, sync and upgrades.
- `upc` reads the PM's live state on demand. It never keeps a cached copy of package state.
- `upc` stores only its own choices (which language, PM, project type, environment mapping).
- Env vars and secrets are out of scope: no handling.
- Distribution: Homebrew only, for now.

## 2. Layers and dependency direction

```
cmd/upc            main only: build registry, run CLI
  └─ internal/register    composition root: the ONLY package importing concrete stacks/projects
       ├─ internal/stacks/<language>/...   concrete languages + PMs (one package tree per language)
       └─ internal/projects/<type>/...     concrete project types (one package per type)
  └─ internal/cli         cobra wiring; language-agnostic; talks to core interfaces only
internal/core        interfaces, base types, registry, errors. Imports nothing concrete.
internal/components  shared infrastructure (exec, log, prompt, fs, render, output)
internal/config      layered config (global < workspace root < project; ADR-0011), schema + compatibility checks
```

Rules (enforced by an architecture test, see ADR-0002):

1. `core` imports only the standard library and `components` interfaces.
2. Outside `internal/stacks/python`, the string/identifier "python" appears nowhere (except `register` and docs). Same for each language and PM. A PM id (for example `uv`) is also allowed anywhere inside the tree of the language that owns it (`internal/stacks/<lang>/<pm>` exists), so the language module can declare and configure its own manager. Project type ids are ordinary words and are covered by the import rules instead.
3. Concrete packages never import each other across languages. A project type may not import a language, and a language may not import a project type: they meet only through `core` interfaces and templates selected at runtime.
4. Adding a language, PM or project type = adding one package + one line in `register`. Zero other edits.

Go has no abstract classes. Shared behavior lives in **embedded base structs** in `core` (e.g. `core.BasePackageManager`); variation lives behind **interfaces**.

## 3. Core model *(built, Phase 1)*

| Type | Role |
|---|---|
| `Module` | Base identity: `ID() string`, `Kind() Kind` (`language`, `pm`, `projecttype`). |
| `Language` | Detects a language in a directory; lists its PM ids and default PM. |
| `PackageManager` | Belongs to one language. Declares a `CapabilitySet`; exposes `Tool()` (binary, version probe, install hint) and `Detect()`. |
| `ProjectType` | `script`, `package`, `app` (no CLI or library sub-types). Declares which (language, PM) pairs it scaffolds and renders templates. |
| `PackageFile` | Abstraction over a PM manifest that `upc` must read (e.g. `pyproject.toml`, `package.json`) to detect, never to own. |
| `Registry` | Generic registries keyed by id; populated only by `register`. |
| `Capability` | Enumerated operation: `init, install, add, remove, list, update, outdated, audit, run, build, test, lint, up, down, publish`. The set is data in `core`. |
| `Support` | `Native` (routed to PM), `Filled` (implemented by `upc`), `Unsupported(reason)`. |

**Capability parity.** Every PM must declare every capability in `core.AllCapabilities`. A contract test iterates all registered PMs and fails if any capability is undeclared. Adding a capability to the core set therefore breaks the build until each PM declares it, and `Unsupported` fails loudly at runtime with the PM and capability named (never a silent no-op). See ADR-0003.

**Signatures (as built).** `PackageManager` exposes `Language()`, `Tool()`, `Capabilities()`, `Detect(fs.FS)` and `Execute(ctx, Capability, Request)`. Concrete managers embed `core.BasePackageManager` (built with `NewBasePackageManager(id, language, tool, markers...)`) and call `Declare(capability, support, handler)` for every capability; `Declare` rejects unknown capabilities, an unsupported entry without a reason, and a native or filled entry without a handler. `Execute` on an unsupported or undeclared capability returns `*UnsupportedError` (matches `ErrUnsupported`, exit code 3). `ProjectType.Scaffold` takes a `ScaffoldRequest` that embeds `Request` and adds `Name`, `Language` and `PackageManager`. `Registry[T]` rejects empty ids, kind mismatches and duplicates. The contract suites in `internal/core/contract` (`RunAll`) are what later phases call with `register.Default()`.

**Requests.** Handlers receive a single `core.Request` struct (project dir, packages, group, script, passthrough args, dry-run) and return a `core.Result`. This keeps the registry uniform; see ADR-0003 for the trade-off.

## 4. Shared components *(built, Phase 1)*

| Package | Responsibility |
|---|---|
| `components/execx` | `Runner` interface over `os/exec`: context, working dir, env, dry-run, structured logging of every routed command. A fake runner is used in tests. |
| `components/runstate` | Tool-neutral run state per app (Phase 6.3): `State` for `<app dir>/.universal-package-core/run/` with `up.pid`, `up.log` and a `provider` record (`provider.go`); `Start`, `Stop` (outcomes stopped, stale, nothing running) and the provider record. |
| `components/logx` | `log/slog` setup: text or JSON handler to stderr, level from `--verbose`. |
| `components/prompt` | `Prompter` interface: confirm / choose. Non-interactive (`--yes`, no TTY, CI) never blocks: auto-answers or fails with a clear message. |
| `components/fsx` | Filesystem access via an interface so tests use temp dirs/`fstest`. Atomic writes. |
| `components/render` | `text/template` rendering from `embed.FS`; used by project types. |
| `components/output` | Human vs `--json` result printing. Data on stdout, logs on stderr. |

## 5. Storage and config *(built, Phase 1)*

- Global: `~/.universal-package-core/`. Project: `.universal-package-core/` in the project root. Directories are created lazily.
- Layers, lowest to highest (ADR-0011, `internal/config/layers.go` `LoadMerged`): global, workspace-root config (found by walking up), the gitignored root `local.toml`, project or app config, then the gitignored app-level `local.toml`. Scalars override, tables merge, lists replace. `config show --origin` (ADR-0011 decisions 2 and 6) reports the layer each key came from.
- Format: TOML (`config.toml`). See ADR-0004.
- Config holds only `upc`'s own choices (language, PM, project type, env mapping, workspace members). It never mirrors dependency lists or versions.
- An environment is a name, an optional env file and dependency `groups` (ADR-0015); there is no venv per environment. Group install is built for uv, poetry and pip; npm and yarn accept only dev and optional (arbitrary groups, ADR-0015 option B, are deferred; `internal/stacks/node/npm/deps.go`).
- `config set` validates against registry compatibility: PM switching is allowed within a language and refused across languages.
- `config show --origin` prints each key's effective value and its layer (global, root config, root local, app config, app local, flag; ADR-0011 order). Values from a `local.toml` layer are masked as `****`. No CLI flag sets a config key yet, so the flag layer is never populated by the command (it is tested at the config level). An empty value counts as unset, matching the merge path. Without `--origin` it is a usage error (use `config get`). `--json` data is a list of `{key, value, layer, masked}`.

## 6. Cross-cutting behavior

- **Logging:** structured `slog`; every routed command, decision, fallback and failure is logged with attributes (`pm`, `capability`, `cmd`, `dir`, `dry_run`).
- **Failure handling:** operations that write multiple files stage in a temp dir and move into place, or roll back, so no half-done state remains where avoidable.
- **Missing tools:** `ToolMissingError` is caught by the CLI, which asks (`Prompter`) to install via the platform's installer, then continues; with `--yes` it installs, non-interactive without `--yes` it fails with instructions.
- **Exit codes:** 0 ok, 1 runtime failure, 2 usage, 3 unsupported capability, 4 tool missing and declined.

## 7. CLI surface

See `project.devlang` `cli-commands`. Phase mapping:

| Phase | Commands |
|---|---|
| 1 (**built**) | `version`, `completion`, global flags, `config get/set/validate` |
| 2 (**built, MVP**) | `new script`, `init`, `add`, `remove`, `list`, `install`, `run`, `update`, `outdated`, `doctor` (uv) |
| 3 (**built**) | `build`, `test`, `lint`, `audit`, `new package`, `config set pm` switch, pip/poetry. `publish` CLI added in Phase 6.3 |
| 4 | Node (`npm`, `yarn`) for all of the above |
| 5 (**built via Phase 6**) | `env list|use|show|add|remove` and `--env` (**built**, ADR-0006; `internal/components/envs`, active name passed as `UPC_ENV` through the runner modules receive); `workspace check`, `new app` (**built**, Phase 6.4/6.5); the Homebrew release is configured but unpublished (`docs/RELEASING.md`); `up`/`down`/`publish` are built (Phase 6.3, ADR-0016) |

## 8. Testing strategy

- Unit tests per package; table-driven.
- Contract tests in `core` run against every registered PM/language/project type (capability parity, id uniqueness, isolation).
- CLI tests execute cobra with `SetArgs`/`SetOut` against a fake `Runner`.
- End-to-end scripts (`test/`, `testscript`) drive the real binary against temp project dirs. Real-PM tests are opt-in via `UPC_E2E=1`.
- Architecture test (`internal/arch`): `go list -json` import data checked against section 2, plus a whole-word scan that language and package manager ids appear only in their own tree, `register`, and docs, plus a marker-file scan (`arch.CheckMarkers`, see section 12 and ADR-0009). Project type ids are not word-scanned (see `docs/CHANGELOG.md`). The rules are pure functions with their own tests on synthetic violations, so they cannot pass vacuously while no concrete module exists. `internal/arch` is the one non-`cmd` package allowed to import `register`.

## 9. ADR index

See `docs/adrs/`. ADR-0008 (script `run` lookup order) was added in Phase 2 and ADR-0009 (package-manager switching and detection ranking) in Phase 3 and ADR-0010 (Node and TypeScript template and runtime choices) in Phase 4. ADR-0006 (environment model) and ADR-0007 (monorepos and workspaces; decision only, workspace code is not built) were added in Phase 5. ADR-0011 through ADR-0016 (config layering, constraint mapper, data-driven stacks, workspaces/apps, environments and groups, run lookup and `up`/`down`) belong to Phase 6; ADRs 0011 to 0016 are Accepted and implemented.

## 10. Phase 2 stack: Python, uv, `script` *(built)*

Commands: `new`, `init`, `add`, `remove`, `list`, `install`, `update`, `outdated`, `run`, `doctor`. All but `new`, `init` and `doctor` come from one table-driven `capabilityCommand` factory in `internal/cli`; dispatch (`internal/cli/dispatch.go`) logs every routed command, and when a tool is missing it asks to install it (`--yes` installs, non-interactive otherwise fails with exit code 4) and re-runs the original command. `init` never edits manifests; it writes `.universal-package-core/config.toml` and runs `install`. `doctor` reports tool presence and version, the capability matrix, config-versus-manager drift and uv's own findings, and offers fixes.

`script` (`internal/projects/script`) asks the chosen manager to `init` in script mode, adds neutral extras from its templates, and removes partial output on failure. It finds compatible stacks by querying the registry.

`app` (`internal/projects/app`, Phase 6.5) has the same shape: the manager runs `init` in a staging directory, the language's `templates/app` are rendered into it (`APP.md`, a minimal server, `Dockerfile`, `compose.yaml`; Node adds `tsconfig.json` and `src/`), and the result is moved into place only when complete. Python apps also get the upc-owned `[tool.upc.scripts]` table (`dev`, `start`) in the new `pyproject.toml`; Node apps get `dev`, `build` and `start` in `package.json`. Existing manifests are never edited. Supported for uv, pip, poetry, npm and yarn.

uv capability table (`internal/stacks/python/uv`):

| Capability | Support | Routed command |
|---|---|---|
| init | Native | `uv init`, `uv init --script <file>.py` |
| install | Native | `uv sync [--group g]`, `uv sync --script f`; `requirements.txt` projects: `uv venv` then `uv pip install -r requirements.txt` |
| add / remove | Native | `uv add|remove [--dev|--group g] [--script f] pkgs...` |
| list | Native | `uv tree --depth 1`; script `uv tree --script f --depth 0`; requirements `uv pip list --format json` |
| update | Native | `uv lock --upgrade[-package p]` then `uv sync`; requirements `uv pip install --upgrade -r requirements.txt` |
| outdated | Native | `uv tree --depth 1 --outdated`; requirements `uv pip list --format json --outdated` |
| run | Native | file, then `[tool.upc.scripts]`, then `[project.scripts]` (ADR-0008) |
| audit, build, publish | Native | `uv audit`, `uv build`, `uv publish` (`audit` and `build` have CLI commands since Phase 3; `publish` since Phase 6.3) |
| test, lint | Filled (Phase 3) | `uv run -- pytest [args]`, `uv run -- ruff check [args]`, or the `test`/`lint` entry of `[tool.upc.scripts]`; needs a pyproject.toml. Not declared: Unsupported (exit 3) with how to enable |
| up, down | Filled (Phase 6.3) | `up`: first of `up`/`dev`/`start` script, started through `uv run` in the background; `down`: stops it (see section Run state) |

uv `Detect` is true for a `uv.lock`, a `[tool.uv]` table or a configured choice. `python.Pyproject` implements `core.PackageFile` (read-only).

## 11. Phase 3 (slice A): fill helpers and pip *(built)*

Fill helpers live in two places, never per package manager:

- `internal/components/scripts` (language-neutral): `Resolve(fsys, name, Reader)` implements the ADR-0008 lookup (file, then command table, then entry point) over a `Table` that a manager's `Reader` supplies, plus `Split` for quoted command lines. `python.ScriptReader` reads `[tool.upc.scripts]` and `[project.scripts]`.
- `internal/stacks/python/shared` (Python only): `Invocation` (command recording, dry-run result), virtual environment lookup (`.venv/bin` or `.venv/Scripts`), `Tool` plans for pytest and ruff (`Plan` returns a command only when the project declares the tool: a `test`/`lint` script, a dependency, a `[tool.*]` table or a config file; otherwise `ErrNotConfigured`, which managers report as Unsupported with the enabling step), `RequireModule` (turns a missing Python module into a `ToolMissingError` so dispatch offers to install it), pip-JSON parsing and script-name validation. uv now uses the same helpers.

pip (`internal/stacks/python/pip`, `pip.New(fsx.FS)`): `requirements.txt` is the manifest and `.venv` the environment; `python -m pip` runs from the venv.

| Capability | Support | Routed command |
|---|---|---|
| init | Filled | `python3 -m venv .venv` and an empty `requirements.txt` (kept if present); script type writes `<name>.py` with a PEP 723 block |
| install | Native | `pip install -r requirements.txt` (`requirements-<group>.txt` for a group), creating `.venv` first if missing |
| add | Filled | `pip install pkgs...`, `pip freeze`, then one atomic write that pins each package (`name[extras]==ver; marker`) in `requirements.txt` (`--dev` is `requirements-dev.txt`, `--group g` is `requirements-g.txt`) |
| remove | Filled | `pip uninstall --yes pkgs...`, then the lines are dropped from the file |
| update | Filled | `pip install --upgrade pkgs...` (or `-r requirements.txt`), then `==` pins are moved to the installed versions; ranges and options are untouched |
| list / outdated | Native | `pip list --format json [--outdated]` (all installed packages in `.venv`) |
| run | Filled | shared lookup; files run under the venv python, commands and entry points from the venv `bin` |
| build, audit, publish | Filled | `python -m build`, `python -m pip_audit`, `python -m twine upload dist/*`; the module is checked first and its install is offered |
| test, lint | Filled | shared pytest / ruff plans run from the venv; undeclared: Unsupported with the enabling step |
| up, down | Filled (Phase 6.3) | `up`: first of `up`/`dev`/`start` script started in the background via the manager's `run`; `down` stops it via run state |

Requirements edits happen only after every command succeeded and only when the content changes, so a failure mid-add leaves the file byte-identical; adding a package already listed replaces its line in place (idempotent). Comments, options and ordering are preserved.

Auto-detect priority (Python): configured manager, then the strongest marker: `core.DetectRanker` ranks a match (uv: `uv.lock` 300, `[tool.uv]` 200, inline-metadata script 150; pip: `requirements.txt` 100). The CLI keeps the highest rank, logs the decision, and on a tie asks (interactive) or fails listing the candidates (non-interactive or `--yes`). The Python language now lists `uv` and `pip`; `uv` stays the default.

## 12. Phase 3 (slices B-D): poetry, `package`, switching, markers *(built)*

**Project types.** `script` (`internal/projects/script`) and `package` (`internal/projects/packagetype`) are built; `app` is not. `package` is language-agnostic: the chosen manager creates `Dir/Name` natively (`uv init --package`, `poetry new --src`; pip fills a setuptools `src` layout), then `package` adds a neutral `LICENSE` placeholder and the language's `package` templates (`.gitignore`, `tests/`). `Supports` asks the registry whether the manager belongs to the language and can `init`.

poetry (`internal/stacks/python/poetry`, `poetry.New()`): `pyproject.toml` `[tool.poetry]` is the manifest; poetry owns `poetry.lock` and the environment.

| Capability | Support | Routed command |
|---|---|---|
| init | Native | `poetry new --src <name>` (package type) or `poetry init --no-interaction`; other project types (script) are refused with an invalid-request error |
| install | Native | `poetry install [--with g]` |
| add / remove | Native | `poetry add|remove [--group g] pkgs...` |
| list / outdated | Native | `poetry show --top-level [--outdated]` |
| update | Native | `poetry update [pkgs...]` |
| run | Native | file: `poetry run python <file>`; `[tool.upc.scripts]` command: `poetry run <words>`; `[project.scripts]` entry: `poetry run <name>` |
| build, publish | Native | `poetry build`, `poetry publish` |
| test, lint, audit | Filled | `poetry run` of pytest, ruff, pip-audit (or the `test`/`lint`/`audit` script); undeclared: Unsupported with the enabling step |
| up, down | Filled (Phase 6.3) | `up`: first of `up`/`dev`/`start` script started in the background via the manager's `run`; `down` stops it via run state |

Capability summary (from each manager's `declare`):

| Capability | uv | pip | poetry |
|---|---|---|---|
| init | Native | Filled | Native |
| install, list, outdated | Native | Native | Native (install/list/outdated) |
| add, remove, update, run | Native | Filled | Native |
| build, publish | Native | Filled | Native |
| audit | Native | Filled | Filled |
| test, lint | Filled | Filled | Filled |
| up, down | Filled | Filled | Filled |

**Detection ranking.** `core.DetectRanker` is optional on a manager. Ranks: uv `uv.lock` 300, `[tool.uv]` 200, inline-metadata script 150; poetry `poetry.lock` 300, `[tool.poetry]` 200; pip `requirements.txt` 100. Auto-detect keeps the highest; equal top ranks are a tie (ask when interactive, otherwise fail listing the candidates). `doctor` reports drift when matches tie or the files point at a manager other than the configured one(s).

**Switching.** `upc config set pm <id>` (alias of `package_manager`; `package_managers = [...]` is the ordered list key) is accepted only for a manager of the project's current language; another language fails with `ErrIncompatible`, exit code 2 (the Phase 3 plan text says 1; implemented and tested behavior is 2, see ADR-0009). A switch never converts manifests or lockfiles; it prints a note and points to `upc doctor`.

**Marker-file rule.** A manager may pass marker file names to `NewBasePackageManager` (`uv.lock`, `poetry.lock`, `requirements.txt`). `core.Registries.MarkerFiles()` maps each name to the owning language, and `arch.CheckMarkers` fails if any scanned `.go`, `.tmpl` or `.toml` file outside `internal/stacks/<language>/` names one (docs, `register`, `internal/arch`, testdata are exempt, as for the id scan). This keeps file-name conventions inside the language tree.

**Known gaps.**
- `list --pm` (listing another manager's view) does not exist.
- A new pip `package` has no `.venv` until `upc install` runs.
- A new poetry `package` needs `upc install` before `upc test` (pytest is not installed until then).

## 13. Phase 4: Node language, npm and yarn *(built)*

Adding the second language touched only `internal/stacks/node`, `internal/register`, docs and test fixtures; `core`, `cli`, `components`, `config` and the `script` and `package` project types are unchanged. (One Python test fixture named a Node marker file and was renamed to a neutral name so the marker scan stays strict.) Decisions: ADR-0010.

**Languages and managers**

| Language | Managers (default first) | Detection | Project types |
|---|---|---|---|
| python | uv, pip, poetry | `pyproject.toml`, `requirements.txt`, `setup.py`, PEP 723 script | script, package |
| node | npm, yarn | `package.json`, `tsconfig.json`, `package-lock.json`, `yarn.lock` | script, package (TypeScript templates) |

`internal/stacks/node` holds the `Language` (templates under `templates/{script,package}`), the read-only `package.json` reader (`PackageFile`: name, version, scripts, engines, `packageManager`; `ScriptReader` for the ADR-0008 lookup) and `shared` (command invocation, name validation, list rendering). `internal/stacks/node/npm` and `internal/stacks/node/yarn` are the managers (`npm.New(fsx.FS)`, `yarn.New(fsx.FS)`). The tables below describe npm; yarn declares the same capabilities with these differences (ADR-0010 addendum):

| yarn | Level | Command (yarn classic 1.x; berry noted) |
|---|---|---|
| init | Native | seeds `package.json` (yarn has no `pkg set`), then `yarn init --yes`; package then `yarn add --dev typescript @types/node` |
| add / remove | Native | `yarn add [-D\|-O\|-P] pkgs...`, `yarn remove pkgs...` |
| list / outdated | Native | `yarn list --depth=0 --json` (narrowed to declared names), `yarn outdated --json` (exit 1 is a result); Unsupported on berry |
| update, audit, publish | Native | `yarn upgrade`, `yarn audit`, `yarn publish`; berry: `yarn up`, `yarn npm audit`, `yarn npm publish` (chosen by `yarn --version`) |
| run, test, build, lint | Native / Filled | `yarn run <script>`, `yarn test`, `yarn run build` else `yarn tsc`, `yarn run lint` |
| up, down | Filled (Phase 6.3) | `up`: first of `up`/`dev`/`start` script started in the background as `yarn run <script>`; `down` stops it via run state |

Detection: `yarn.lock` ranks 300, a `packageManager` of `yarn@...` 200; `yarn.lock` and `package-lock.json` together tie and the CLI asks.

| Capability | Support | Routed command |
|---|---|---|
| init | Native | `npm init --yes` then `npm pkg set` (script: `<name>.ts`, private manifest, `start` script; package: library fields, then `npm install --save-dev typescript @types/node`) |
| install | Native | `npm install` |
| add / remove | Native | `npm install|uninstall [-D|-O|--save-peer] pkgs...` (groups: dev, optional, peer) |
| list | Native | `npm ls --depth=0 --json` (exit 1 with JSON is a result; an `error` object is a failure) |
| update | Native | `npm update [pkgs...]` |
| outdated | Native | `npm outdated --json` (exits 1 when packages are outdated; parsed, not failed) |
| run | Native | file: `node [--experimental-strip-types] <file>`; `package.json` script: `npm run <name> [-- args]` |
| audit, publish | Native | `npm audit`, `npm publish` |
| test | Native | `npm test [-- args]` |
| build | Filled | `npm run build`, else `npm exec --no -- tsc` when `tsconfig.json` exists, else Unsupported with guidance |
| lint | Filled | `npm run lint`, else Unsupported with guidance |
| up, down | Filled (Phase 6.3) | `up`: first of `up`/`dev`/`start` script started in the background via the manager's `run`; `down` stops it via run state |

Every npm command except `init` requires a `package.json` in the project directory (npm would otherwise create one). Detection: `package-lock.json` ranks 300 and `package.json` 100; the manager's markers are `package-lock.json` and `package.json` and may be named only under `internal/stacks/node/`. `doctor` reports a missing lock file and a `packageManager` field that names another manager. A cross-language switch (`config set pm npm` in a Python project, or a Python manager in a Node project) is refused with exit code 2 and covered by `test/testdata/pm_cross_language.txtar`.

## 14. Phase 6.3: `up`, `down`, `publish`, run sources *(built)*

Decision record: ADR-0016 (Accepted).

**Run state.** `internal/components/runstate` (`runstate.go`, `provider.go`) keeps state per app in `<app dir>/.universal-package-core/run/`: `up.pid`, `up.log`, and a `provider` file naming the up provider that started the app (absent when a script started it). `State.Start` cleans a stale record and the previous log, launches the command and records the pid; `State.Stop` returns `Stopped`, `Stale` (process already gone, record cleaned) or nothing running. The orchestrator keeps no shared state file.

**Process control.** `execx.Controller` (`internal/components/execx/process.go`: `Start(ctx, cmd, logPath)`, `Alive(pid)`, `Stop(pid)`), implemented by `execx.OS` on top of gopsutil (`proctree.go`: liveness, start time, tree terminate/kill), with only the `detach_unix.go` / `detach_windows.go` pair left per OS; `execx.ControllerFor(runner)` returns the runner as a `Controller` or an error when it is not one. `execx.Ambient` is a `Runner` that uses the runner carried by each call's context, so registered providers need no runner at construction.

**`up` resolution.** Each manager's `up` resolves the first script named `up`, `dev` or `start` (`scripts.UpNames`) and starts it in the background through the manager's own run command (`uv run`, `poetry run`, `npm run`, `yarn run`; pip via its venv). With `--dry-run` it prints the command. When no script exists (`scripts.ErrNothingToStart`), `cli/runwire.go` consults `core.Registries.UpProviders` in order (the compose provider). `down` first asks the provider recorded in run state to stop the app; otherwise the manager stops the recorded process.

**`RunSource` and `UpProvider`.** Defined in `internal/core/runsource.go`. `RunSource` (`Recipes`, `Run`) ranks below the manifest sources: when `run` finds no declared script (`scripts.ErrNotDeclared`), `runwire.go` tries each `core.Registries.RunSources`. `UpProvider` (`Name`, `Detect`, `Up`, `Down`) takes `core.UpTarget{Dir, App}`. Implementations: `internal/stacks/just` (recipes from `just --summary` through the runner; no justfile or `just` missing means no recipes, never an error) and `internal/stacks/compose` (`docker compose -p <project> -f <file> up/down`, file `docker-compose.yml` or `compose.yaml`; the project name is per app). Both are registered in `internal/register/register.go`.

**Tool-name confinement.** `just`, `docker` and `compose` as quoted Go literals may appear only under their stack package: `internal/arch/toolliterals.go` (`ToolOwners`, `CheckToolLiterals`; test files and `internal/arch` exempt, comments not matched).

**`publish`.** `internal/cli/publish.go`: runs `build`, asks for confirmation (`Prompter.Confirm`, default no; `--yes` answers yes, non-interactive without `--yes` fails naming the flag, so nothing is published), then the manager's `publish`. Global `--dry-run` runs nothing and prints the build and publish commands (the documented fallback; native per-manager dry-run is a follow-up). Credentials are never read or stored. `-w/--member` is accepted by `up`, `down` and `publish`; `internal/cli/member_test.go` checks every capability command honors it.

**Tests.** `internal/register/lifecycle_parity_test.go` fails if any manager lacks `up`, `down` or `publish`.

**Known gaps (ADR-0016 follow-ups).** Native `publish --dry-run` per manager; no offer to create an up script when none exists; run state under `-w` keys off the request directory, not the member. Closed in 6.7: config `scripts`/`up` argv lists (`internal/cli/configwire.go`), Windows process-tree kill and a pid-reuse guard (start time in `up.pid`), `down` keeping `up.log`.

## 15. Phase 6.4: workspace, apps and contracts *(built)*

Decision record: ADR-0014 (Accepted). Config only: no package graph between apps, and no package manager package imports `internal/workspace` (enforced by `internal/arch`).

**Objects.**
- `Workspace` (`internal/workspace/workspace.go`): root directory, the config `members`, and the apps by name. `Load` validates the `[apps.*]` tables first. It implements `envs.Lookup` for `${apps.<name>.<field>}` and owns `Order`, `Findings` and `ChildEnv`.
- `App` (`app.go`): name, path (relative) and dir (absolute), port, url, provides, consumes, contract, raw env. `Field` exposes `name`, `path`, `port` and `url` to interpolation.
- `ContractChecker` (`internal/core/contractchecker.go`): normalizes one contract kind to a snapshot and diffs two snapshots. Implemented by `internal/contracts/openapi` and `internal/contracts/jsonschema`; `internal/contracts/document` parses JSON and YAML (via `go.yaml.in/yaml/v3`) for both. `internal/contracts/detect.go` picks the checker.
- `ContractLocks` (`internal/workspace/contractlock.go`, `contractcheck.go`): the committed lock of each app at `.universal-package-core/contracts/<app>.lock`. `Status` compares (missing, ok, changed with a diff summary), `CheckAll` covers every app with a `contract`, and only `Update` writes.
- `EnvResolver` (`internal/components/envs/resolver.go`): resolves `${...}` references (nested, with cycle and unresolved-variable errors) and builds a child process env. It depends on the `Lookup` interface, not on the workspace.
- `Orchestrator` (`internal/workspace/orchestrator.go`): starts apps through a `Runner` in dependency order (`consumes` after `provides`) and stops them in reverse. A failed start rolls back the apps started in that call. `internal/cli/apps.go` supplies the `Runner`, which runs each app's own `up` and `down` capability.

**Commands.** `upc workspace list` (members and apps in start order); `upc workspace check [--update]` (`internal/cli/workspace_check.go`: exits non-zero when a lock is missing or a contract changed, writes locks only with `--update`); `upc up|down --all|--app <name>`; cross-app `doctor` findings `port-collision`, `consumer-without-provider`, `unresolved-variable`, `contract-missing`, `contract-changed` (`internal/workspace/findings.go`, one kind each).

**Known gap.** ADR-0014 decision 3 (config `members` must agree with the manager's native member list) is not checked: the managers expose no cheap native member listing, so `workspace check` compares contracts only.

## 16. Phase 6.6: drift in Diagnose *(built)*

`doctor` reports drift for every manager. It is file-based: each manager's `Diagnose` reads the lockfile or requirements file and the install directory (`.venv`, `node_modules`) and runs no tool.
- Python (`internal/stacks/python/shared/envdrift.go`): uv and poetry compare every package the lock requires with `.venv` (`uv/drift.go`, `poetry/drift.go`, so transitive packages included; poetry skips optional, non-main-group and marker-guarded ones); pip compares exact pins in the requirements file (`pip/pip.go`).
- Node (`internal/stacks/node/shared/envdrift.go`): npm and yarn compare the lockfile version of each dependency declared in `package.json` with `node_modules`; transitive packages are not checked. A project with no `node_modules` (not installed, or Plug'n'Play) reports nothing. pnpm is not supported.
- Manager-selection drift (several managers' files present, or config naming another manager) is separate and lives in `internal/cli/drift.go`.
