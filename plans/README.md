# Implementation plans

Source of truth for requirements: `../project.devlang`. Design: `../docs/ARCHITECTURE.md` and `../docs/adrs/`.

Plans are **read-only during execution** (track progress elsewhere). Finish a phase's exit criteria before starting the next. Every phase has a *Documentation & Testing* step and an *Integration validation* step, and begins with a pilot on one representative target before broad application.

## Phases

| # | File | Delivers | Depends on |
|---|---|---|---|
| 1 (done) | `01-foundation.md` | Buildable CLI skeleton, core interfaces, registry, components, config/storage, logging, test and arch harness | none |
| 2 (done) | `02-mvp-python-uv-script.md` | **MVP**: Python + `uv` + `script` type; `new/init/add/remove/list/install/run/update/outdated/doctor` | 1 |
| 3 (done) | `03-python-stacks-and-package-type.md` | `pip` and `poetry`, gap-filling, PM switching, `package` type, `build/test/lint/audit` | 2 |
| 4 (done) | `04-node-stack.md` | Node with `npm` then `yarn`, same capability set, cross-language switch refused | 3 |
| 5 (partial: only `env list\|use\|show` built; the rest moved to 06) | `05-environments-workspace-app-release.md` | `env`, `up/down`, `workspace`, `app` type, `publish`, Homebrew release | 4 |
| 6 (6.0-6.5, 6.7 done; 6.6 code complete, awaiting owner release actions; 6.8 docs audit done, gates and owner release actions pending) | `06-agnostic-core-environments-multiapp-release.md` | Data-driven commands, constraint mapper, environments and groups, config layering, `up/down`, `publish`, workspace/multi-app, `app` type, release, hardening. Absorbs the unbuilt parts of 05 | 5 (slice 1) |

Plan 03 says a cross-language switch exits 1; the code exits 2. Plans stay read-only, so this is recorded in `docs/CHANGELOG.md` (Known gaps).

## MVP definition

The simplest version that meets the mvp-required requirements: **Phase 1 + Phase 2**.

A user can, on a machine with `uv`, run `upc new script`, `upc init` in an existing Python project, `upc add|remove|list|install|update|outdated|run`, and `upc doctor`, with structured logging, `--dry-run`, `--json`, `--yes`, `--project`, project/global config, and missing-tool prompting, all routed to `uv`.

Why this cut: the spec's `decisions` put Python/`uv` and `script` first. `uv init --script` and `uv add --script` give a native script flow, so the MVP proves the whole architecture (core, registry, capability model, templates, config, routing) on one real stack with the lowest scaffolding cost. Everything later only adds modules (the architecture's central claim), which Phase 3 and 4 then validate.

## Acceptance mapping (from `project.devlang`)

| Acceptance | Delivered by |
|---|---|
| Spin up any project type with any stack | script: P2, package: P3, app: P6; stacks: uv P2, pip/poetry P3, npm/yarn P4 |
| Install and manage packages how I need | P2 (uv), P3, P4 |
| Spin a project up/down locally in different environments | P6 (`env` slice built in P5) |
| Environment never out of line between PM and upc | P2 `doctor` drift check (basic), P6 drift for all PMs and env groups |

## Cross-phase conventions

- Go 1.27.3 (installed), `gofmt`, `go vet`, `golangci-lint` 2.15.0, `goreleaser` 2.19.0, node 26.8.2, yarn 1.22.22, uv 0.12.24, poetry 2.4.3 (all installed locally).
- No symlinks in git. No committed scratch notes or `AGENTS.md`.
- Every PR-sized step: code + tests + docs (`ARCHITECTURE.md`, `CHANGELOG.md`, ADR when a decision is made).
- Tests never weakened to pass; fix the code.
- Interfaces live in `core`; concrete code only in its own package; `register` wires.
- Logging: every routed command, decision, fallback and failure uses `slog` with attributes.
- Post-change audit after each phase: list stale docs/tests and report.
