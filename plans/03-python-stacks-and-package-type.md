# Phase 3: Python breadth + `package` type

**Depends on:** Phase 2 (MVP) complete and its exit criteria met.
**Goal:** prove the "add one package, zero other edits" claim twice (pip, poetry), add the `package` project type, fill capability gaps, and implement PM switching within a language.

## Pilot-first order
Pilot **pip** (largest capability gap, so it exercises gap-filling hardest). Lock the filling pattern in shared code, then do **poetry** (the cheaper, mostly-native case), then the `package` type, then `build/test/lint/audit` commands.

## Deliverables

### Capability gaps: shared fill helpers (no per-PM copies)
- Place reusable fills in `internal/stacks/python/shared` (python-only, imported only by python PMs) for things only Python PMs share: script table lookup for `run`, venv creation, pytest/ruff/build invocation.
- Language-neutral fills (e.g. "run a named script from `[tool.upc.scripts]`") go in `internal/components/scripts` with an interface the PM supplies a reader for.
- Fill matrix (verify each against `pip --help`, `poetry --help` in the pilot):

| Capability | pip | poetry |
|---|---|---|
| init | Filled: create `.venv` (`python -m venv`) + minimal `requirements.txt`/`pyproject.toml` | Native `poetry init`/`new` |
| install | Native `pip install -r requirements.txt` | Native `poetry install` |
| add / remove | Filled: `pip install X` then update `requirements.txt` (pin via `pip freeze` of just that package) / `pip uninstall` + edit file. The pip manifest is `requirements.txt`; keep it minimal and in PM-compatible format. | Native |
| list | Native `pip list --format json` | Native `poetry show --top-level` |
| update | Filled: `pip install -U` + rewrite pins | Native `poetry update` |
| outdated | Native `pip list --outdated --format json` | Native `poetry show --outdated` |
| run | Filled: venv python + script table | Native `poetry run` |
| build | Filled: `python -m build` (offer install of `build`) | Native `poetry build` |
| test, lint | Filled: pytest / ruff if declared in config or detected; else a clear `Unsupported` with how to enable | same |
| audit | Filled: `pip-audit` if present (ask to install) | Filled: `pip-audit` / `poetry audit` plugin if present |
| publish | Filled: `twine upload` (ask to install) | Native `poetry publish` |

`test` and `lint` are also filled for **uv** in this phase with the same shared helpers (updating the Phase 2 table from Unsupported).

### Modules
- `internal/stacks/python/pip`, `internal/stacks/python/poetry`: new packages, each embedding `core.BasePackageManager`; `Tool()` info and `Detect` (`poetry.lock`/`[tool.poetry]`, `requirements.txt`+`.venv`).
- `python.go`: add `pip`, `poetry` to PM list; priority for auto-detect when several are present: explicit config > lockfile (`uv.lock` > `poetry.lock`) > `requirements.txt`; ambiguity asks (interactive) or errors with a list (non-interactive). Log the decision.
- `internal/projects/package` (new project type; language-agnostic): scaffolds a library/package with `lib`-style layout via the PM natively where possible (`uv init --package|--lib`, `poetry new`), templates for pip. Config, capabilities and templates in its own package. Includes `src/` layout, tests dir, README, license placeholder.
- `register`: three new lines (pip, poetry, package). This is the isolation proof: git diff outside these packages is limited to registration + docs + generic CLI table rows.

### CLI
- New generic commands via the Phase 2 `capabilityCommand` factory: `build`, `test`, `lint`, `audit`.
- `config set pm <id>`: switch within a language (uv <-> pip <-> poetry): validates, warns that `upc` does not convert manifests/lockfiles ("native PM owns them"), offers `doctor`. Cross-language switch refused with `ErrIncompatible` (exit 1, clear message).
- `doctor`: add drift checks for multiple PM markers in one project (e.g. both `uv.lock` and `poetry.lock`), and report fills vs native per PM.
- `list` supports multiple PMs in one project when configured (`--pm` filter); config allows `package_managers = [...]` for "multiple PMs at once".

## Tests
- Per PM: `FakeRunner` argv tests for every capability (native route and each fill), plus the contract suite via `RunPackageManagerSuite`.
- `pip` fill behavior on a real temp dir with a fake runner: `requirements.txt` edits are exact and idempotent; failure mid-add leaves the file unchanged.
- `package` type scaffold under each of uv/pip/poetry (table-driven): expected file tree golden.
- Arch test continues to pass with zero allowlist edits (that is the isolation check).
- Config: PM switch matrix incl. cross-language refusal (still no npm module, so use a fake node-like module registered only in tests).
- E2E (opt-in): per PM, `new package` -> `add` -> `build` -> `test` in temp dir (needs pip, poetry 2.4.3 installed; uv installed).

## Documentation & Testing
- `ARCHITECTURE.md`: updated capability table (all three PMs), fill-helper locations. `docs/README.md` examples. `CHANGELOG.md`.
- ADR-0008: pip manifest handling and which fills are native vs filled; ADR for auto-detect priority if it changes the Phase 1 ADR-0004 root discovery.

## Integration validation
`make check`; `UPC_E2E=1 go test ./test/...`; manual: create a package with each PM and switch pip to uv in a scratch project (`upc config set pm uv`, `upc doctor`) and confirm the project still installs with the new PM's own commands.

## Exit criteria
- uv, pip, poetry all pass the parity contract; no capability is silently missing.
- `package` type works for each.
- Git diff review shows additions limited to the new packages, `register`, shared fill helpers, docs, and table rows.
