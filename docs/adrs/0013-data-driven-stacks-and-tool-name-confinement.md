# ADR-0013: Data-driven stacks and tool-name confinement

Status: Accepted 2026-11-15 (Phase 6.1 landed)

## Context
Command lines, install hints, binary names and the default language are Go literals spread over manager packages and `internal/register`. yarn borrows npm's audit and publish verbs (`internal/stacks/node/yarn/deps.go`). That makes `core` and shared code carry per-stack knowledge.

## Decision
1. **Command templates are data.** `core.CommandSpec` holds argv templates with placeholders `{pkg}`, `{constraint}`, `{group}`, `{dir}`. Install hints are a per-OS set (darwin, linux, windows, other), not a single `brew` line. `Tool.Binary` and the language default name are data.
2. **Remove yarn to npm coupling.** yarn declares its own audit and publish specs (classic: `yarn audit`, `yarn publish`; berry: `yarn npm audit`, `yarn npm publish`), selected by detected major version, without referencing the npm package or npm verb names.
3. **Tool-name confinement.** `core` defines `RunSource` and `UpProvider` interfaces. Manifest-script lookup (`[project.scripts]`, `dev`/`start` names) becomes per-stack data. The arch test (`internal/arch`) forbids `os/exec` outside `internal/components/execx` and forbids tool-name literals (just, docker, compose, npm, yarn, uv, pip, poetry) outside `internal/stacks`, `internal/register` and docs.
4. **Interfaces in `core`, concrete code in its own package, wiring in `register`.**

## Consequences
- Adding a manager is data plus one package; `core` and `cli` stay neutral.
- `just` and `compose` live in their own stack-style packages (ADR-0016), not in `internal/components` or `internal/tools`.
- The pilot is `uv`; roll-out then follows pip, poetry, npm, yarn.
