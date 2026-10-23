# Changelog

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Update with every change.

## [Unreleased]

### Added
- Pre-development documentation: `docs/README.md`, `docs/ARCHITECTURE.md`, ADRs 0001-0005.
- Phase plans in `plans/` (01 foundation through 05 environments/workspace/release).
- Project spec (`project.devlang`), launch task, Go module and `cmd/upc` placeholder committed as the baseline.

### Known repo issues to fix in Phase 1
- `cmd/upc/main.go` declares `package upc`; must be `package main`.
- `.gitignore` contains `/target` (Rust convention); replace with `/bin` and `/dist`.
