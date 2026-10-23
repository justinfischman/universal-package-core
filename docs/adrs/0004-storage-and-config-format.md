# ADR-0004: Filesystem storage and TOML config

Status: Accepted (pre-development)

## Context
Storage is filesystem only, two scopes, project wins, directories created when needed. `upc` must not cache PM state.

## Decision
- Global dir `~/.universal-package-core/`, project dir `.universal-package-core/` (hidden, project root). Resolve the home dir via `os.UserHomeDir`; allow `UPC_HOME` override for tests.
- File: `config.toml` in each scope, parsed with `github.com/pelletier/go-toml/v2` (comments, human-editable, strict unknown-field detection via `DisallowUnknownFields`).
- Merge: global first, project overlays per key. Resolution order is logged at debug level.
- Content: language, package manager, project type, environment mapping, workspace members. **Never** dependency lists, versions or lockfile data.
- Writes are atomic (temp file + rename). Directories are created on first write only.
- Project root discovery: walk up from `--project` or cwd to find `.universal-package-core/`, falling back to PM marker files (`pyproject.toml`, `package.json`, ...) via PM `Detect`.

## Consequences
Zero-config for existing projects (detection), no disk bloat, no stale copy of PM state.
