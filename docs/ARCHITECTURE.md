# Architecture

Status: **Phase 1 built** (core, components, config, CLI skeleton). This document describes the design from `project.devlang`. Sections marked *(planned)* are not built yet; see `plans/` for the phase that builds each one. Update this file with every change.

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
internal/config      layered config (global < project), schema + compatibility checks
```

Rules (enforced by an architecture test, see ADR-0002):

1. `core` imports only the standard library and `components` interfaces.
2. Outside `internal/stacks/python`, the string/identifier "python" appears nowhere (except `register` and docs). Same for each language, PM and project type.
3. Concrete packages never import each other across languages. A project type may not import a language, and a language may not import a project type: they meet only through `core` interfaces and templates selected at runtime.
4. Adding a language, PM or project type = adding one package + one line in `register`. Zero other edits.

Go has no abstract classes. Shared behavior lives in **embedded base structs** in `core` (e.g. `core.BasePackageManager`); variation lives behind **interfaces**.

## 3. Core model *(built, Phase 1)*

| Type | Role |
|---|---|
| `Module` | Base identity: `ID() string`, `Kind() Kind` (`language`, `pm`, `projecttype`). |
| `Language` | Detects a language in a directory; lists its PM ids and default PM. |
| `PackageManager` | Belongs to one language. Declares a `CapabilitySet`; exposes `Tool()` (binary, version probe, install hint) and `Detect()`. |
| `ProjectType` | `script`, `package`, `app`. Declares which (language, PM) pairs it scaffolds and renders templates. |
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
| `components/logx` | `log/slog` setup: text or JSON handler to stderr, level from `--verbose`. |
| `components/prompt` | `Prompter` interface: confirm / choose. Non-interactive (`--yes`, no TTY, CI) never blocks: auto-answers or fails with a clear message. |
| `components/fsx` | Filesystem access via an interface so tests use temp dirs/`fstest`. Atomic writes. |
| `components/render` | `text/template` rendering from `embed.FS`; used by project types. |
| `components/output` | Human vs `--json` result printing. Data on stdout, logs on stderr. |

## 5. Storage and config *(built, Phase 1)*

- Global: `~/.universal-package-core/`. Project: `.universal-package-core/` in the project root. Directories are created lazily.
- Project overrides global, field by field.
- Format: TOML (`config.toml`). See ADR-0004.
- Config holds only `upc`'s own choices (language, PM, project type, env mapping, workspace members). It never mirrors dependency lists or versions.
- `config set` validates against registry compatibility: PM switching is allowed within a language and refused across languages.

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
| 2 (**MVP**) | `new script`, `init`, `add`, `remove`, `list`, `install`, `run`, `update`, `outdated`, `doctor` (uv) |
| 3 | `build`, `test`, `lint`, `audit`, `new package`, PM switch, pip/poetry |
| 4 | Node (`npm`, `yarn`) for all of the above |
| 5 | `env`, `up`/`down`, `workspace`, `new app`, `publish`, Homebrew release |

## 8. Testing strategy

- Unit tests per package; table-driven.
- Contract tests in `core` run against every registered PM/language/project type (capability parity, id uniqueness, isolation).
- CLI tests execute cobra with `SetArgs`/`SetOut` against a fake `Runner`.
- End-to-end scripts (`test/`, `testscript`) drive the real binary against temp project dirs. Real-PM tests are opt-in via `UPC_E2E=1`.
- Architecture test (`internal/arch`): `go list -json` import data checked against section 2, plus a whole-word scan that language and package manager ids appear only in their own tree, `register`, and docs. Project type ids are not word-scanned (see `docs/CHANGELOG.md`). The rules are pure functions with their own tests on synthetic violations, so they cannot pass vacuously while no concrete module exists. `internal/arch` is the one non-`cmd` package allowed to import `register`.

## 9. ADR index

See `docs/adrs/`. Decisions still open: environment model (decided in Phase 5, ADR-0006), monorepo/workspace strategy (Phase 5, ADR-0007), template location/format refinements (Phase 2).
