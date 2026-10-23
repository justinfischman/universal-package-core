# ADR-0001: Go, cobra, and stdlib slog

Status: Accepted (pre-development)

## Context
`project.devlang` requires a CLI-only tool in Go with structured logging, shell completion, global flags and non-interactive CI mode.

## Decision
- Go (module `github.com/justinfischman/universal-package-core`, binary `upc`).
- `github.com/spf13/cobra` for commands, flags and completion. Use `RunE` everywhere. Set `EnableTraverseRunHooks = true` on the root so persistent pre-run hooks of the root and the subcommand both run (cobra otherwise runs only the nearest one).
- `log/slog` from the standard library for logging. No third-party logger.
- Commands are thin: parse flags into a `core.Request`, call a use-case, print the result.

## Consequences
Two direct dependencies at start (cobra, a TOML library, see ADR-0004). Everything else is stdlib. Commands are testable via `SetArgs`/`SetOut`.
