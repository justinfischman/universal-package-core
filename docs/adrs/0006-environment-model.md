# ADR-0006: Environment model

Status: Accepted (Phase 5)

## Context
The spec asks for named environments (`local`, `dev`, `test`, `prod`) but says `upc` does not handle environment variable values or secrets. Projects already keep `.env.<name>` files, and some keep them in other places. The package manager's virtual environment (`.venv`, `node_modules`) is a different thing and stays the manager's.

## Options
- **A. Convention only.** Every `.env.<name>` file in the project root is an environment. No config, but no way to point at `config/dev.env` and no default.
- **B. Mapping in project config.** `[environments.<name>] file = "..."` in `.universal-package-core/config.toml`, plus a default. Explicit, but empty until the user writes it.

## Decision
**B with the convention as fallback.**
- Config: `[environments.<name>] file = "<relative path>"` (empty `file` means `.env.<name>`), `default_environment = "<name>"`, and `active_environment = "<name>"`, which only `upc env use` writes. Names are letters, digits, `-` and `_`; files are relative paths that stay inside the project (`config.Validate`).
- Discovery (`internal/components/envs`): declared environments, plus `.env.<name>` files in the project root that no declaration covers. `.env.example`, `.env.sample`, `.env.template` and `.env.dist` are templates, not environments.
- `env list|show|use`. `use` writes the selection to the project config only after the environment exists and its file is present; a missing one is a usage error (exit 2). The effective environment is the active one, else the default.
- `upc` never opens an env file and never prints a value. It passes the name to wrapped commands as `UPC_ENV=<name>`, set in one place (the runner handed to modules in `internal/cli/dispatch.go`). Forwarding the file path to tools that take one natively is left to a later change.
- `doctor` warns when the effective environment is missing or its file is absent.

## Consequences
- Both styles work without setup, and an explicit mapping wins over the convention.
- The selection is `upc`'s own state in config; nothing is cached from the package manager.
- Tools read their own env file; `upc` cannot tell whether a file is well formed.
- Checking the virtual environment against the lockfile is a manager capability and is not part of this change.
