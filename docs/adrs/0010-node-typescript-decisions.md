# ADR-0010: Node and TypeScript template and runtime choices

Status: Accepted (Phase 4, slice A: `node` language, `npm`)

## Context
Phase 4 adds the second language. TypeScript is how most new Node code is written, but it is not a separate ecosystem: the same manifest (`package.json`), registry and package managers serve both. The plan left three choices open: how a `script` runs TypeScript, what test runner a `package` ships with, and how later managers join. Installed locally: Node 26.8.2, npm 11.19.1. The plan refers to this ADR as 0009; 0009 is taken (switching and ranking), so it is 0010 (0006 and 0007 stay reserved for Phase 5).

## Decision
1. **One language, `node`.** TypeScript is a template and tooling variant. The `package` templates are TypeScript; a project counts as TypeScript when it has a `tsconfig.json` (`node.IsTypeScript`), which is what the `build` fill checks. Plain JavaScript projects are the same language without that file.
2. **Script runtime: Node's built-in type stripping.** A script is a directory with a private `package.json` (so `add`, `install` and `list` work with npm) and one `<name>.ts` file. `upc run <file>.ts` runs `node --experimental-strip-types <file>`; `.js` and `.mjs` run as `node <file>`. The generated `start` script uses the same command. The flag is accepted by every release that can strip types (22.6 and later, including 26.x, where it is a no-op without warnings) so scripts keep working across the supported range; `engines.node` is `>=22.6.0`. `tsx` was rejected: it is an extra download on every run (`npx`) or a dependency per script, and it hides type errors the same way stripping does. Type checking is explicit: `tsconfig.json` is present for editors and `tsc --noEmit`, with `erasableSyntaxOnly` so everything written type-checks and also runs stripped.
3. **Package build: `tsc`.** A `package` compiles `src/` to `dist/` with `tsc` (devDependencies `typescript` and `@types/node`, installed by `new` so the project builds immediately). `rewriteRelativeImportExtensions` lets source import `./x.ts`, which the stripped test run needs, while `dist` gets `.js` imports.
4. **Test runner: Node's built-in `node:test`.** `scripts.test` is `node --test`; it discovers `test/*.test.ts` by default from Node 22.18, so `engines.node` for a package is `>=22.18.0`. No test dependency is added. vitest and jest stay a user choice (`upc add --dev vitest` and a `test` script).
5. **`package.json` is owned by npm.** `upc` reads it (name, version, scripts, engines, `packageManager`) and changes it only through `npm` (`npm init`, `npm pkg set`, `npm install`). The one exception is a seed file holding only `{"name": ...}` that a new script writes before `npm init`, because `npm init` would otherwise name the package after the staging directory.
6. **Capabilities.** npm routes init, install, add, remove, list, update, outdated, run, audit, publish and test natively. `build` is filled (the `build` script, else `tsc` when a `tsconfig.json` exists, else Unsupported with guidance) and `lint` is filled (the `lint` script, else Unsupported with guidance; npm has no linter and upc picks none). `up` and `down` are Unsupported. `npm outdated` and `npm ls` exit 1 while still printing valid JSON; a non-zero exit with parsable output is a result, and an `error` object or an empty output is a failure.
7. **Detection.** Language: `package.json`, `tsconfig.json`, `package-lock.json` or `yarn.lock`. npm: `package-lock.json` ranks 300 and `package.json` 100, so a lock file of another manager added later outranks the shared manifest.

## Adding pnpm or bun (the proof path)
A later manager is one package under `internal/stacks/node/<id>` with its own `New`, capability table, markers (`pnpm-lock.yaml`, `bun.lock`) and rank, one name in the language's `PackageManagers()`, and one import plus one registration line in `internal/register`. It reuses `node/shared` and the `package.json` reader. Core, the CLI, `script` and `package` need no change, which this phase confirms: nothing outside `internal/stacks/node`, `internal/register`, docs and test fixtures changed for npm. Yarn (classic and berry, detected by `yarn --version` and the `packageManager` field) is the next slice and follows the same recipe.

## Consequences
- Scripts need Node 22.6 or newer, packages 22.18 or newer; older runtimes fail with Node's own message.
- Stripped TypeScript is not type checked at run time. Users who want checks run `tsc --noEmit`, which `upc build` does for packages.
- A new package needs network access at creation (it installs `typescript`); a failure removes the half-built directory.
- `node/shared` duplicates the command-invocation and package-list helpers of the other language's shared package. Both are language-neutral in effect; promoting them to `internal/components` is a later cleanup that needs a coordinated change to the other tree.

## Addendum: yarn (Phase 4, slice B)
- **Target.** Yarn classic (1.22) is what is installed and tested. `yarn --version` is read only where classic and berry (2+) differ: `upgrade` vs `up`, `audit` and `publish` vs `yarn npm ...`. Berry's `list` and `outdated` are Unsupported because it has neither command. Berry init, fixtures and E2E are out of scope; one FakeRunner case covers its argv.
- **`package.json` and yarn.** Yarn has no `pkg set`, so for yarn the one seed-file exception in decision 5 grows: upc writes the starting manifest (name, version, type, engines, scripts, library fields) and `yarn init --yes` keeps it and adds the rest (`main`, `license`). After that yarn alone changes it.
- **Detection.** `yarn.lock` ranks 300 and a `packageManager` of `yarn@...` 200; npm's `package-lock.json` is 300 and bare `package.json` 100. A lock of each kind ties and the CLI asks; a yarn declaration alone beats the bare manifest but loses to `package-lock.json`.
- **`yarn list`** flattens transitive packages, so upc keeps only the names `package.json` declares.
- Shared runtime constants and script helpers moved to `node/shared`.
