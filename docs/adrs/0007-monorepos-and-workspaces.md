# ADR-0007: Monorepos and workspaces

Status: Accepted (Phase 5, partly deferred)

## Context
The spec wants several apps in one folder to be easy. uv and npm/yarn already have workspaces that own member discovery, shared lockfiles and per-member commands. A second member registry in `upc` would drift from them.

## Options
- **A. `upc` owns a member registry** in its config (`workspace add|remove`), independent of the manager. Works for any layout, but duplicates the manager's list and goes stale.
- **B. Defer to the manager's workspaces.** Read members from the manager's live state; `upc` adds only listing and targeting.

## Decision
**B.** Package-manager-native workspaces (uv, npm, yarn) are the source of truth.
- Built in the workspace slice: `upc workspace list` (members read from the manager's own files through the language module) and `-w/--member <name>` targeting on capability commands, plumbed once through the shared command factory.
- Deferred: `workspace add|remove`, per-member `.universal-package-core` overrides of the root config, and multi-root discovery for layouts without a manager workspace. The existing `workspace` key in config stays reserved and unused.

## Consequences
- No member list to keep in sync; a layout the manager does not know is not a workspace for `upc` yet.
- Language and manager names stay inside their own trees, as member discovery is a language capability (ADR-0002).
- This slice only records the decision; no workspace code ships with it.
