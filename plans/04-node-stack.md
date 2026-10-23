# Phase 4: Node (TypeScript) stack

**Depends on:** Phase 3 complete (parity suite stable with three Python PMs).
**Goal:** second language end to end with `npm` then `yarn`; confirm the core needs no changes to add a language; confirm cross-language switching is refused.

## Pilot-first order
Pilot `npm` for `script`, `package`, then repeat for `yarn` by applying the identical recipe. Do not start `yarn` until npm is locked. Other PMs (pnpm, bun) are explicitly out of this phase: they become later single-package additions and are listed in the ADR as the proof path.

## Deliverables

### `internal/stacks/node` (the only package mentioning Node/JS/TS)
- `node.go`: `Language` impl. `Detect`: `package.json`, `tsconfig.json`, lockfiles (`package-lock.json`, `yarn.lock`). Language id `node`; TypeScript is a template/config variant (`typescript = true`) inside the language, not a separate language.
- `npm/`: native routes: `npm init`, `npm install`, `npm install|uninstall <pkg> [-D]`, `npm ls --depth=0 --json`, `npm update`, `npm outdated --json`, `npm run <script>`, `npm audit`, `npm publish`, `npm test`. Fills: `build` (run `build` script if present, else TypeScript `tsc` if configured), `lint` (run `lint` script, else Unsupported with guidance).
- `yarn/`: same capability set (`yarn add|remove|install|outdated|upgrade|run|npm audit|npm publish`). Handle yarn classic vs berry: detect via `yarn --version` and `packageManager` field; log the choice. Yarn 1.22.22 (classic) is installed locally; berry is not, so berry paths are covered by fakes and fixtures. Exercise the missing-tool prompt by removing yarn from PATH in a subshell.
- `PackageFile` for `package.json` (read-only: name, scripts, engines, `packageManager`). `upc` never writes it except through the PM (`npm pkg set` for script creation).
- Templates under `node/templates/{script,package}/`: TypeScript `tsconfig.json`, `src/index.ts`, minimal test setup (use the Node built-in test runner or vitest; decide in ADR-0009).
- Script type for Node: a single-file TS script runnable by `node --experimental-strip-types` or `npx tsx`; choose in ADR-0009 after checking the installed Node version (npm 11.19.1 present; check `node --version`).

### Shared and generic changes (should be near-zero; each one is a finding)
- `register`: add `node`, `npm`, `yarn`.
- If any `internal/cli` or `core` edit is needed beyond that, record it as a **design defect** in `CHANGELOG.md`, fix the abstraction, and note in ARCHITECTURE.md.
- `config.Validate` already refuses cross-language PM switching; add the real test now that `npm` exists: `config set pm npm` in a Python project -> refused with a clear message, config unchanged.
- Multi-language projects (e.g. Python backend + Node frontend in one repo): allowed as separate roots/workspace members (full support in Phase 5); here ensure detection returns all candidates and prompts or errors on ambiguity.

## Tests
- Parity suite on npm and yarn. `FakeRunner` argv tests for every capability. JSON-output parsing tests for `npm ls --json` and `npm outdated --json` using recorded fixtures in `testdata/` (including empty and error output).
- Language detection fixtures; yarn classic vs berry detection.
- Scaffold goldens for script and package.
- Arch test with no allowlist edit; add check that the word `python` does not appear in the node tree and vice versa (word list is registry-driven).
- Cross-language refusal integration test.
- E2E opt-in: `new package` (TS) -> `add` -> `build` -> `test` with npm; yarn E2E only if yarn available, else skipped with a clear skip reason.

## Documentation & Testing
`ARCHITECTURE.md` (languages and PM table), `docs/README.md` (Node usage), `CHANGELOG.md`, ADR-0009 (Node/TS template and runtime choices), ADR noting pnpm/bun path.

## Integration validation
`make check`; opt-in E2E; manual: `upc new package --language node --pm npm`, `upc add zod`, `upc outdated --json`, then in a Python project `upc config set pm npm` is refused.

## Exit criteria
- npm and yarn pass parity; all Phase 2/3 commands work for Node.
- Adding Node touched only `internal/stacks/node`, `register`, docs, and test fixtures (diff reviewed).
- Cross-language switch refused with test coverage.
