# Phase 2: MVP - Python + uv + `script`

**Depends on:** Phase 1 complete. **Blocks:** Phases 3-5.
**Goal:** the MVP. All of it routes to `uv` (verified installed: uv 0.12.24 with `init --script|--app|--lib|--package`, `add|remove --script`, `sync`, `lock`, `tree`, `run`, `audit`, `build`, `publish`, `pip list --outdated`).

## Pilot-first order

1. **Pilot target:** `upc add` for Python/uv. It exercises detection, config, registry lookup, capability dispatch, runner routing, dry-run, logging, JSON output and tests. Lock the pattern.
2. Apply the locked pattern to the remaining commands.
3. Then `script` project type and `new`/`init`.

## Deliverables

### `internal/stacks/python` (language module; no other package mentions Python)
- `python.go`: `Language` impl. `Detect(dir)`: markers `pyproject.toml`, `requirements.txt`, `setup.py`, `*.py` with PEP 723 block. `PackageManagers()`: `["uv"]` now; default `uv`.
- `uv/uv.go`: `PackageManager` embedding `core.BasePackageManager`. `Tool()`: binary `uv`, probe `uv --version`, install hint `brew install uv` (platform-selected).
  - `Detect`: `uv.lock` present, or `[tool.uv]` in `pyproject.toml`, or user config.
  - Capability table (**Native** routes; the exact flags verified in the pilot against `uv help <cmd>`):

| Capability | Support | uv command |
|---|---|---|
| init | Native | `uv init` / `uv init --script <file>` |
| install | Native | `uv sync` (script: `uv sync --script`? verify, else `uv run --script` resolves on demand) |
| add / remove | Native | `uv add|remove [--dev|--group g] [--script f] pkgs...` |
| list | Native | `uv tree --depth 1` for projects; `uv pip list --format json` for flat list |
| update | Native | `uv lock --upgrade[-package p]` then `uv sync` |
| outdated | Native | `uv pip list --outdated --format json` (verify against `uv tree --outdated`) |
| run | Native | `uv run <script-name or file> -- args` ; `[project.scripts]`/`[tool.upc.scripts]` lookup filled by `upc` when name is not a file |
| audit | Native | `uv audit` |
| build, publish | Native (declared now, command surface lands in Phase 3/5) | `uv build`, `uv publish` |
| test, lint | Unsupported(reason: no native runner) in Phase 2; Phase 3 fills | |
| up, down | Unsupported in Phase 2; Phase 5 fills | |

  - `PackageFile` impl for `pyproject.toml` (read-only: detect, project name, requires-python, script table). `upc` never writes it; uv does.
  - Missing-tool behavior: `uv` not found -> `ToolMissingError` -> CLI prompts install (`brew install uv`, or the official installer on Linux/Windows) -> re-run original command.
- `templates/script/`: files uv's `--script` flow does not create (e.g. README snippet). Prefer native `uv init --script` (ADR-0005).

### `internal/projects/script` (project type; language-agnostic)
- `Scaffold(ctx, plan)`: asks the chosen language/PM to `init` in script mode, then renders neutral extras. Staged and atomic: a failure removes the partial output.
- Declares compatible stacks via registry query (no names hardcoded).

### `internal/cli` additions
- `new <type>` (flags `--language --pm --name`; guided prompts if missing and interactive; `--yes` picks defaults: python/uv).
- `init`: detect language+PM from the cwd/`--project` via registry `Detect`; write `.universal-package-core/config.toml` only if absent or confirmed; if nothing detected -> guided walkthrough; leave the project ready (runs `install`). Non-disruptive: it never edits manifests.
- `add|remove|list|install|update|outdated|run`: a single generic `capabilityCommand` factory builds each from a table (name, capability, args validation), so there is no per-command duplication.
- `doctor`: report per detected stack: tool present/version; config vs PM drift (config says uv but no `uv.lock`/`[tool.uv]`, or lockfile missing); capability matrix; offer fixes via Prompter (`--yes` applies safe fixes).
- Config rule check: an incompatible PM in config is rejected (Phase 1 `Validate`).

### `internal/register`
- Registers `python`, `uv`, `script`. One line each. Contract suites now run on real modules.

## Tests (everything tested)
- uv PM unit tests with `FakeRunner`: exact argv for each capability, dry-run, `--script`, groups, errors (nonzero exit surfaced with stderr), missing tool prompt/accept/decline/non-interactive.
- python language: detection table over temp-dir fixtures (pyproject only, requirements only, PEP 723 script, empty dir).
- script type: scaffold success and rollback on injected failure.
- CLI tests per command through cobra with fakes; `--json` golden files.
- Contract suites: PM parity, language, project type.
- Arch test: confirm no "python"/"uv" outside their trees.
- **E2E (opt-in `UPC_E2E=1`)** `test/*.txtar`: real `uv`; `new script` -> `add requests` -> `list` -> `run` -> `remove` in a temp dir; `init` on a fixture uv project leaves `pyproject.toml`/`uv.lock` byte-identical (non-disruption check); `init` on a bare `requirements.txt` project.

## Documentation & Testing
- Update `ARCHITECTURE.md` (commands built, capability table for uv), `docs/README.md` (real usage examples), `CHANGELOG.md`.
- Add an ADR for any new decision, e.g. script `run` lookup order (file path > `[tool.upc.scripts]` > `[project.scripts]`). Numbers 0006 and 0007 are reserved for Phase 5 (environments, workspaces); use 0008+ for decisions made earlier.

## Integration validation
`make check`; `UPC_E2E=1 go test ./test/...`; manual run in a scratch dir: `upc new script hello --yes && cd hello && upc add rich && upc run hello.py && upc outdated --json && upc doctor`; `upc add rich --dry-run` prints the routed command and changes nothing; delete `uv` from PATH in a subshell to see the install prompt and the non-interactive failure message.

## Exit criteria (MVP)
- All listed commands work for Python/uv + script, verified by the E2E run.
- Logs show every routed command and decision (`--verbose`).
- No edits outside `internal/stacks/python`, `internal/projects/script`, `internal/register`, and generic CLI commands were needed to add the stack (verified by git diff review: this is the first proof of the isolation rule).
- Docs match code; post-change audit reported.
