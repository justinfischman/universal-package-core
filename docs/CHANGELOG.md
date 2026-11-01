# Changelog

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Update with every change.

## [Unreleased]

### Added
- Pre-development documentation: `docs/README.md`, `docs/ARCHITECTURE.md`, ADRs 0001-0005.
- Phase plans in `plans/` (01 foundation through 05 environments/workspace/release).
- Project spec (`project.devlang`), launch task, Go module and `cmd/upc` placeholder committed as the baseline.

### Added (Phase 1: foundation)
- `internal/core`: `Module`, `Capability`/`Support`/`CapabilitySet`, `Request`/`Result`, error types with `ExitCode`, `PackageManager` with embedded `BasePackageManager`, `Language`, `ProjectType`, `PackageFile`, generic `Registry`, `coretest` fakes, and `contract` suites.
- `internal/components`: `execx` (runner, dry-run, fake), `logx`, `prompt`, `fsx` (atomic writes), `render`, `output`.
- `internal/config`: layered TOML store (`UPC_HOME`), validation (cross-language manager switches refused), project root discovery.
- `internal/cli`: root command with global flags, session wiring, `version`, `config get|set|validate`; `completion` is cobra's built-in.
- `internal/register` composition root (empty until Phase 2), `internal/arch` architecture test (ADR-0002).
- Makefile, `scripts/check.sh`, `.golangci.yml`, GitHub Actions CI, testscript E2E in `test/`.

### Fixed
- `cmd/upc/main.go` now declares `package main` and wires registry, CLI and signal handling.
- `.gitignore` now ignores `/bin`, `/dist`, `*.test` and `coverage.out`. `/target` is kept because the editor writes `target/flycheck*` there.

### Deviations from `plans/01-foundation.md` (also folded into the plan after the fact, at the user's request)
- The architecture test lives in `internal/arch`, not `internal/core`: it must import `register`, which core is forbidden to do.
- `render` renders everything in memory, checks conflicts, then writes atomically per file and removes created files on failure, instead of staging in a temp directory.
- The forbidden-word scan covers language and package manager ids only. Project type ids (`script`, `package`, `app`) are ordinary words in core's vocabulary; the import rules cover project types.
- The `version` testscript runs by default (it needs no external tool). Tests that need real package managers will be opt-in via `UPC_E2E=1`.
