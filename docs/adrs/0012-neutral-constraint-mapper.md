# ADR-0012: Neutral version-constraint mapper

Status: Draft (Phase 6; accepted when 6.1 lands)

## Context
Each manager spells version constraints differently (`>=2.3.4` for uv and pip, `^2.3.4` and `~` for poetry and npm, `@2.3.4` for yarn). Users want one spelling: `upc add dep --version=">=2.3.4"` on every manager.

## Decision
1. **`core.Constraint`** holds an operator and a version. It supports `>=`, `>`, `<=`, `<`, `==`, `!=`, `~=`/`~`, `^` and ranges such as `>=a,<b`. `Parse(string)` returns a `Constraint` or an error; `Render` produces each manager's dialect.
2. **`Request.Version`** carries the constraint, set by a flag on `add` (and on `update` where the manager supports pinning). The 6.1 pilot decides between `--version` and `--constraint` as an alias; the root `upc --version` already exists, and a test proves there is no collision.
3. **Untranslatable constraints fail** with a clear message naming the manager and the constraint. They are never approximated or dropped.
4. **Consumers** updated with the change: dry-run and `--json` output, the doctor capability matrix, `update`, and the `internal/core/contract` parity test.

## Consequences
- One constraint grammar to document and test; per-manager rendering is data-driven (ADR-0013).
- Some constraints are valid for one manager and an error for another (for example `!=` on npm). The error text states this.
