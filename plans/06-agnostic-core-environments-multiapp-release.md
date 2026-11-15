# Phase 6: Agnostic core, environments/groups, multi-app, `app`, publish, release

**Depends on:** Phase 4 complete (code state: uv, pip, poetry, npm, yarn; `script` and `package` types).
**Supersedes:** the unfinished items of `05-environments-workspace-app-release.md` (that file stays read-only; its scope is re-cut here with the user's later decisions). Where 05 and 06 differ, 06 wins.
**Goal:** make `upc` a neutral mapper whose per-stack knowledge is data, add named environments with dependency groups, config-only multi-app workspaces, then finish up/down, publish, the `app` type, release and hardening.

## Decisions (from the user, each gets an ADR in the phase that implements it)

| # | Decision | ADR (new, next free number) |
|---|---|---|
| D1 | Hidden config `.universal-package-core/` (project) and `~/.universal-package-core/` (global); no user env vars; upc injects computed env into child processes only; secrets in gitignored `.universal-package-core/local.toml`; precedence flag > app local.toml > app config > root local.toml > root config > global, merged key by key (scalars override, tables merge, lists replace), walk stops at the git root or a workspace-root marker; `config show --origin` | 0011 |
| D2 | Neutral constraint mapper: `upc add dep --version=">=2.3.4"` on every manager; untranslatable constraints fail with a clear message | 0012 |
| D3 | Command lines, install hints, default language, binary names are data (command templates), not Go literals; remove yarn->npm coupling | 0013 |
| D4 | Fullstack via config only (`[workspace]`, `[apps.<name>]`, `${apps.api.url}`); no package dependencies between apps | 0014 |
| D5 | Environments = name + optional env file + dependency groups; no venv per environment | 0015 (extends ADR-0006, narrowing its env-file-only scope; ADR-0006 text is not edited) |
| D6 | Justfile is an optional run source in the ADR-0008 order, run via `execx`; `just` never required | 0016 (ADR-0008 not edited; 0016 restates the neutral lookup order per stack) |

**Step 0 (before any code):** write ADR-0011..0016 as drafts in one step, numbered in this order; each phase then updates its own ADR from draft to accepted. ADR-0014 supersedes the deferred items of ADR-0007 (workspace add|remove, per-member overrides, multi-root discovery) and partly its option B, and states who wins when manager-native workspace members and config `members` disagree (the manager's native list defines package membership; config `members` define upc apps; a path in both must match or `workspace check` fails).

## Verified starting state (evidence)

- `internal/config/config.go:40-52`: `Config` has `Environments map[string]Environment{File}`, `DefaultEnvironment`, `ActiveEnvironment`, and `Workspace []string` (reserved, unused). `config.go:220-228` discards errors from `p.Get(k)` and `res.Set(k, v)` (`res.Config, _ = res.Set(k, v)`).
- `internal/core/capability.go:11-25`: `CapUp`, `CapDown`, `CapPublish` exist; `internal/stacks/python/uv/uv.go:85-100` declares up/down `Unsupported` with a literal reason.
- `internal/core/request.go:9-25`: `Request` already has `Group`, `Packages`, `Script`, `Args`; it has no version-constraint field.
- Install hints and OS switches are Go literals: `internal/stacks/python/uv/uv.go:54-60`, `pip/pip.go:61-65` (`brew install python`), `poetry/poetry.go:57-61`, `node/npm/npm.go:64-68`, `node/yarn/yarn.go:66-68`. Default language is a literal at `internal/register/register.go:70`.
- yarn borrows npm semantics: `internal/stacks/node/yarn/deps.go:79,88` (`audit`, `publish` via `verb(..., "npm", ...)`). Discarded error at `internal/stacks/node/yarn/list.go:145` (`_ = json.Unmarshal`).
- `internal/register/register.go:19-72` is 72 lines of hand-built registration; construction errors are repeated per manager.
- `internal/components/execx/execx.go:79` is the only `exec.CommandContext`; `execx/withenv.go` injects env (`UPC_ENV` is set once in `internal/cli/dispatch.go`, `internal/cli/env.go:16`).
- pip calls `python -m pip` (`internal/stacks/python/pip/env.go:56`, `deps.go:177,253`), not a bare `pip`; the Phase 6 check is to confirm the interpreter name (`python3` vs `python`) is resolved, see 6.7.
- CI (`.github/workflows/ci.yml:12-23`) runs `scripts/check.sh` on macOS and Ubuntu only; there is no Windows job and no E2E job. E2E testscripts live in `test/testdata/e2e/*.txtar` (6 files).
- No `completion` command in `internal/cli` (grep found none); cobra's default may apply, which 6.6 must verify.
- Test files over 300 lines are listed in the user's brief (npm_test 585, yarn_test 551, pip_test 463, uv capabilities_test 437, poetry_test 402, cli/lifecycle_test 371); line counts to be re-measured in 6.7 before splitting.

## Cross-phase rules

- Pilot rule: every phase touching 2 or more packages starts with a pilot on `uv` alone, a review of build/tests/output, a fix of the method, then roll-out to pip, poetry, npm, yarn in that order.
- One external-process runner only: `internal/components/execx`. No new `os/exec` call anywhere (a go-arch test in `internal/arch` already guards imports; extend it to forbid `os/exec` outside `execx`).
- No `_helpers`/`_utils` packages or files; name by responsibility. Files near 300 lines.
- Never relax types or contracts; fix code. No symlinks; no credentials in repo; secrets only in gitignored `local.toml`.
- Interfaces in `core`, concrete code in its own package, wiring in `register`.
- Each phase ends with its own Documentation & testing and Integration validation (`make check` plus the phase's E2E).

---

## Phase 6.0: CI gate (E2E and Windows)

**Goal:** later phases land behind a real gate. Add to `.github/workflows/ci.yml` (currently macOS and Ubuntu only, lines 12-23): an E2E job with `UPC_E2E=1` that installs uv, yarn and poetry, and a Windows job running `go vet`, lint and tests. Fix failures in the code, not by skipping tests.
**Exit criteria:** CI green on macOS, Ubuntu and Windows plus the E2E job.
**Documentation & testing:** `docs/README.md` supported platforms. **Integration validation:** one full CI run.

---

## Phase 6.1: Data-driven commands and constraint normalizer (pilot: uv)

**Goal:** per-stack facts become data; one neutral constraint type; every manager renders it.

### Scope
- New `core.Constraint` (operator + version; supports `>=`, `>`, `<=`, `<`, `==`, `!=`, `~=`/`~`, `^`, ranges `>=a,<b`) with `Parse(string) (Constraint, error)` and `Render` per dialect.
- `Request.Version string` plus a flag on `add` (and `update` where the manager supports pinning). The root `upc --version` already exists, so the pilot decides between `--version` (as the user asked, scoped to `add`) and `--constraint` as an alias; a test proves no collision. Consumers to update: dry-run and `--json` output, doctor capability matrix, `update`, and the `internal/core/contract` parity test.
- Tool-name confinement: core defines `RunSource` and `UpProvider` interfaces; manifest-script lookup (`[project.scripts]`, `dev`/`start` names) becomes per-stack data. Extend the arch test (`internal/arch`) to forbid `os/exec` outside `execx` AND tool-name literals (just, docker, compose, npm, yarn, uv, pip, poetry) outside `internal/stacks`, `internal/register` and docs.
- Command templates: `core.CommandSpec` (argv templates with placeholders `{pkg}`, `{constraint}`, `{group}`, `{dir}`), `core.InstallHint` set (per OS: darwin, linux, windows, other; not only brew), `Tool.Binary`, language default name.
- Remove yarn->npm coupling (`yarn/deps.go:79,88`): yarn declares its own audit/publish spec (classic: `yarn audit`, `yarn publish`; berry: `yarn npm audit`, `yarn npm publish`) in data, selected by detected major version, with no reference to the npm package or npm verb names.

### Files
- Create `internal/core/constraint.go`, `internal/core/constraint_test.go` (parse + render), `internal/core/commandspec.go`, `internal/core/installhint.go`.
- Create per-stack data: `internal/stacks/python/uv/spec.go` (pilot), later `pip/spec.go`, `poetry/spec.go`, `node/npm/spec.go`, `node/yarn/spec.go`. Data is Go structs in the stack's own package (a stack's knowledge stays in its package, ADR-0002), not external files, so the isolation rule holds.
- Modify `internal/core/request.go` (add `Version`), `internal/cli/capability.go` / `deps.go` (flag), `internal/register/register.go` (build managers from specs, one loop, one error path; target under 40 lines), `internal/stacks/python/uv/uv.go:54-60,85-100`, and the four sibling managers.
- Default language: `python.ID` literal at `register.go:70` moves to a `Language.Default bool` (data) set by `python.New()`; `register` selects by flag, not by name.

### Steps
1. Update the ADR-0012 and ADR-0013 drafts (written in Step 0).
2. Pilot: implement `Constraint` and uv rendering (`uv add "dep>=2.3.4"`); table-driven test `constraint -> argv` for uv including quoting (`>`, `<`, `,`, spaces) and no shell expansion (argv elements, never a joined string).
3. Move uv's install hint and command lines into `spec.go`; keep behavior byte-identical (existing uv tests are the regression net).
4. Review pilot (build, tests, `--dry-run` output). Fix the method (for example, placeholder expansion rules, dialect enum) before roll-out.
5. Roll out: poetry (`poetry add "dep>=2.3.4"`, caret native), pip (`dep>=2.3.4`; `^` and `~` translated to PEP 440 ranges; `~` npm-style to `~=`/range), npm (`dep@">=2.3.4"`, `^`/`~` native), yarn classic/berry (same syntax as npm; berry may also need `yarn add dep@>=2.3.4`).
6. Untranslatable rule: each dialect declares its supported operator set; an operator it cannot express exactly returns `core.InvalidRequestError` stating the constraint, the manager and a suggested equivalent; never rewrite meaning (for example `!=` on npm add, caret on pip are translated only when exactly expressible as a range; otherwise fail).
7. Yarn decoupling and removal of the `npm` verb import path; add an arch test that `node/yarn` does not import `node/npm`.
8. Fix `config.go:227` and `yarn/list.go:145` here only if touched by this phase; otherwise in 6.7 (tracked there).

### Exit criteria
- `upc add <dep> --version=">=2.3.4"` produces the correct argv for all five managers (golden dry-run output).
- No install hint, default language or manager command literal remains in `register.go` or CLI code; stacks keep theirs as data in their own package.
- `yarn` has no dependency on `npm`.
- Untranslatable constraints fail with a message that names the manager and constraint.
- The extended arch test passes: no `os/exec` and no tool-name literals outside the allowed packages.

### Tests
- Constraint parser fuzz/table (valid, invalid, whitespace, pre-release).
- Table: constraint x manager -> argv (including quoting and a Windows-style path in `{dir}`).
- Existing parity suite (`internal/core/contract`) extended: every manager declares add-with-version Native/Filled/Unsupported-with-reason.
- Arch test: no `os/exec` outside `execx`; yarn does not import npm.

### Documentation & testing
ADR-0012, ADR-0013; `docs/ARCHITECTURE.md` section "Command templates and constraints"; `docs/README.md` `--version` examples; `CHANGELOG.md` (Unreleased).

### Integration validation
`make check`; `UPC_E2E=1` lifecycle scripts for uv, then each stack's existing E2E (`test/testdata/e2e/*.txtar`) must pass unchanged; one new E2E adds a package with `--version` on uv, npm.

### Risks
- Constraint semantics differ (`^0.x`, pre-release, `~` meaning). Mitigation: translate only when exactly equal; otherwise fail (D2).
- Data structs that grow into a mini-DSL. Mitigation: placeholders limited to the list above.

---

## Phase 6.2: Environments and dependency groups

**Depends on:** 6.1. Config layering (D1) is built here because environments need it.

### Scope
- Config layering (ADR-0011): project `.universal-package-core/config.toml`, workspace root config, global `~/.universal-package-core/config.toml`, gitignored `.universal-package-core/local.toml`; precedence flag > app local.toml > app config > root local.toml > root config > global, merged key by key (scalars override, tables merge, lists replace; the lowest level wins and the walk up fills gaps), stopping at the git root or a workspace-root marker. Minimal workspace-root discovery is built here: the nearest ancestor with a manager-native or config workspace marker. `upc config show --origin` prints each key's source layer. ADR-0011 records an explicit note: `project.devlang` says env vars and secrets are "not our concern"; `local.toml` and secret-key rejection are a deliberate narrowing (upc still never reads or prints env values; it only keeps secrets out of committed config). No required env vars; computed env is injected into child processes via `execx.WithEnv` only.
- `Environment` = `name` + optional `file` + `groups []string` (extends `internal/config/config.go:40-52`; ADR-0015 extends ADR-0006, narrowing its env-file-only scope; the accepted ADR-0006 text is not edited). The `Workspace []string` field at `config.go:52` is migrated to its typed form here (accepting the old list as `members`) so the layering test can be written..
- `-w/--member <name>` plumbed ONCE in the shared capability-command factory (`internal/cli/capability.go`) and a minimal `upc workspace list` (members from the manager's own files via the language module, per ADR-0007 option B), built and tested here before 6.4 consumes them; workspace-root discovery depends on it.
- Commands: `upc env add <name> [--file PATH] [--groups a,b]`, `env remove`, existing `env list|use|show`; `upc install --env <name>` installs that environment's groups; `upc add <dep> --group <g>`; `--env` on `run`, `test`, `up`.
- No separate venv per environment: one PM environment per project; an environment only selects env file path and groups.
- Group mapping decision (explicit, recorded in ADR-0015):

| Manager | Groups |
|---|---|
| uv | native `[dependency-groups]` / `--group`, `--dev`, `--optional` (verify the installed uv 0.12.24 flags) |
| poetry | native groups (`poetry add --group`, `poetry install --with/--only`) |
| pip | per-group requirements files `requirements-<group>.txt` (and `requirements.txt` for the default group) via `-r` |
| npm | Option B (tested locally with npm 11.19.1; doc claims to confirm in the pilot). Group = workspace member `.universal-package-core/groups/<name>/package.json`; subset install `npm install -w <member>` / `npm ci -w <member>` (root deps skipped unless `--include-workspace-root`; one root `package-lock.json`, `npm ci -w` does not rewrite it); `add <pkg> --group x` = `npm i <pkg> -w <member>`. The workspace glob must be the literal prefix `.universal-package-core/groups/*` (never `**`; dot dirs matched by wildcards are skipped). Pitfalls: unique member names; a later plain `npm install` expands to everything; the subset is not remembered, so pass `-w` every time. `dev`/`optional` map natively. |
| yarn berry (4.x) | Option B (tested with yarn 4.18.1; docs to confirm in the pilot): `yarn workspaces focus <member>` (NOT `yarn workspace <m> install`); lockfile stays full and `--immutable` passes after focus; 2.x/3.x need the workspace-tools plugin; install state is partial and a later `yarn install` re-expands; PnP differs from the node-modules linker; needs corepack/`yarnPath`. |
| yarn classic (1.x) | Option A (tested with 1.22.22): no subset install exists (`yarn workspace g install` installs everything). Membership is upc metadata; `add --group x` = `yarn workspace <member> add` still works, install is full; the limit is recorded in ADR-0015 and surfaced in `doctor`/output. Filled, not Unsupported. |

### Files
- Modify `internal/config/config.go`, `internal/config/validate.go`, `internal/config/root.go` (root discovery walking up), add `internal/config/layers.go` (merge by precedence) and `internal/config/local.go` (local.toml; `init` adds `.universal-package-core/local.toml` and `.universal-package-core/run/` to `.gitignore` and never ignores `contracts/`), `internal/config/origin.go` (`config show --origin`), and `internal/stacks/node/shared/groups.go` (group manifests, per the pilot outcome).
- Modify `internal/components/envs/envs.go` (add groups; keep file-path and discovery logic).
- Modify `internal/cli/env.go` (add `add`, `remove`), `internal/cli/capability.go` (`--env`, `--group`), `internal/cli/dispatch.go` (inject env for effective environment).
- Per-stack group handling in each manager's `spec.go` (data from 6.1).

### Steps
1. Update the ADR-0011 and ADR-0015 drafts.
2. Pilot (uv): layered config + `env add/remove` + `install --env` + `add --group`; review; fix method.
3. Roll out poetry, pip (requirements files; creation and update rules in one place under `internal/stacks/python/shared`), then npm and yarn (arbitrary group names via step 5; dev/optional map natively).
4. `doctor`: report an environment whose groups are unknown to the manager, a group file missing (pip), and a `local.toml` not ignored by git.
5. npm/yarn groups, decided per manager (pilot order: npm, yarn berry, yarn classic): npm and berry use option B (true subset install); yarn classic uses option A (full install, limit recorded in ADR-0015 and shown by `doctor`). Confirm each doc claim in the pilot (research was tested locally only). Add the per-manager behaviour to the parity test (npm/berry true subset; classic Filled with documented full install; none Unsupported). `doctor` checks for a missing or stale group manifest and membership drift.
6. `config validate` rejects secret-looking keys in committed config (`internal/config/validate.go`, test in `config_test.go`).
7. Resolve `internal/config/config.go:227` by returning the error from `res.Set` rather than discarding it (fix, do not suppress); add a test for the failing key.

### Exit criteria
- All managers satisfy the environment/group commands, including arbitrary group names on npm and yarn (option B, or option A with its documented limit recorded in ADR-0015; never an Unsupported for named groups).
- Precedence is covered by a table test with all six layers (flag, app local, app, root local, root, global) and the merge rules (scalar, table, list).
- `env add` and `install --env` never create a second venv or `node_modules` (explicit test per manager).
- Group capability is Native or Filled for every manager, never Unsupported; option A's limit (yarn classic) is recorded in ADR-0015 and surfaced in `doctor`/output. A parity test asserts no manager reports Unsupported for groups (the Unsupported-with-reason carve-out remains only for add-with-version in 6.1).
- A secret-looking key in committed config is rejected by `config validate` with a message pointing to `local.toml` (names only, never values).

### Tests
- Layer merge precedence; workspace-root lookup from a nested directory; `local.toml` ignored by git check.
- `env add/remove` round-trip and validation of names/paths staying in the project.
- Per manager: `add --group`, `install --env` argv goldens; npm/yarn custom group names (option B install, `doctor` drift); `--env` on `test` forwards the environment; no second venv created.

### Documentation & testing
ADR-0011, ADR-0015 (note on ADR-0006); `ARCHITECTURE.md` config and environment sections; `docs/README.md` (config layering, groups, npm/yarn limit); `CHANGELOG.md`.

### Integration validation
E2E: create project, `env add dev --groups dev`, `add pytest --group dev`, `install --env dev` on uv and poetry; npm `--group dev`; `doctor` clean after each.

### Risks
- Config merge ordering mistakes. Mitigation: single merge function, exhaustive table test.
- pip requirements files diverge from reality. Mitigation: `doctor` drift check (6.7) covers them.

---

## Phase 6.3: `up`/`down` and `publish`

**Depends on:** 6.2 (environment injection), 6.1 (command data).

### Scope
- Fill `CapUp`/`CapDown` for all five managers; remove the Unsupported declarations (`uv.go:70-71` and siblings). `CapPublish` already exists in all five; this phase adds the `publish` CLI.
- Run lookup (ADR-0016 restates the neutral lookup order per stack; ADR-0008 is not edited): file in project, config script, manifest script, `[project.scripts]`, then an optional `justfile` recipe run through `execx` (`just <recipe>`; if `just` is absent, skip the source with a debug log and do not fail). Justfile support is optional and run-only; generated-justfile export is not built and is listed under deferred items in `CHANGELOG.md`.
- `up` resolution: configured `up` command, then `dev`/`start` script, then `docker-compose.yml`/`compose.yaml`; offer to create a script if none exist (through the manager, not by editing manifests directly).
- `up` runs the process and records pid/log under `.universal-package-core/run/`; `down` stops it (`compose down` for compose); down-after-crash cleans stale pid files; no orphan state on failure.
- `publish`: `build`, confirm (outward-facing; `--yes` required in non-interactive mode), publish, `--dry-run` natively where offered; never stores credentials. Uses yarn's own data spec from 6.1.
- `-w/--member`: NOT yet built (no member code in `internal/cli` or `cmd`, no `workspace` command; ADR-0007 line 14 wrongly says `workspace list` and `-w` were built; ADR-0007 is not edited, ADR-0014 notes the inaccuracy). This is scheduled as real work in 6.2 (see below), so 6.3 only adds tests that every capability honors it.
- Tool-name confinement: just and docker/compose do NOT live in `internal/components` or `internal/tools`. Core defines `RunSource` and `UpProvider`; `just` and `compose` each live in their own stack-style package (for example `internal/stacks/just`, `internal/stacks/compose`) registered in `register.go`, and the extended 6.1 arch test enforces it.

### Files
- Create `internal/stacks/just/` (recipe names via `just --summary` through execx, implements `RunSource`), `internal/stacks/compose/` (implements `UpProvider`), `internal/components/runstate/` (tool-neutral pid/log record). Run state is per app: `<app dir>/.universal-package-core/run/`, gitignored by `init`; the orchestrator keeps no shared state file.
- Modify each manager's `deps`/`run` files plus `spec.go`; new `internal/cli/publish.go`.

### Steps
Status as of 2026-11-14 (verified against the repo, not the earlier handoff):
- `publish` is already declared and implemented in all five managers (`uv/project.go:71`, pip, poetry, npm, yarn); there is no `publish` CLI command and no `--dry-run`, confirmation or `--yes` handling yet.
- `up`/`down` are Unsupported in every manager (uv: `uv.go:70-71`; pip test asserts it).
- Commit 60c0634 added `internal/core/runstate.go` (pid helpers, no tests). It deviates from this plan: run state belongs in `internal/components/runstate/` and process work in `execx` (Windows: `Signal(0)` liveness is not portable). Fix in step 2.

1. [x] Update the ADR-0016 draft (status: Partially Implemented).
2. [x] (done: `internal/components/runstate`, `execx.Controller`, Windows vet clean) Move `RunState` to `internal/components/runstate/`, make liveness/stop go through `execx` (portable), add tests (write/read/remove, stale pid, per-app dirs).
3. [x] (uv up/down + `publish` CLI done and reviewed 2026-11-14; recipe locked) Pilot (uv): `up`, `down` with a fake runner and scripted background process; review; fix the method. Then `publish` CLI on uv (`--dry-run`, confirm, `--yes`).
4. [x] `RunSource` and `UpProvider` interfaces in core (`internal/core/runsource.go`); `internal/stacks/just/` and `internal/stacks/compose/` implemented with FakeRunner tests (not yet registered or wired into the CLI up path).
5. [x] Roll out `up`/`down` by the locked recipe to all five managers (uv, pip, poetry, npm, yarn); Unsupported declarations removed; full build, vet, Windows vet and tests green 2026-11-14.
6. [x] `publish` CLI (`internal/cli/publish.go`), `up`/`down` capability rows, and `just`/`compose` registration + wiring (`internal/cli/runwire.go`, `execx.Ambient`, `runstate/provider.go`) done 2026-11-14; build, vet, Windows vet, tests green.
   Documented follow-ups (not blocking exit criteria; carry to 6.7 or later): (a) native `publish --dry-run` per manager (global `--dry-run` prints the plan only: documented fallback); (b) offer to create an up script when none exists; (c) run state under `-w` keys off `req.Dir`, not the member; (d) `internal/config` has no script/up settings, so "config script" and "configured up" lookup steps are not wired; (e) Windows `Stop` does not kill child processes and pid reuse is unguarded; (f) `down` keeps `up.log` until next `up`.
7. [x] (`internal/cli/member_test.go`; found and fixed: build/test/lint/audit/run lacked `flagMember` in `capability.go`) Test proving every capability command honors the 6.2 `-w` plumbing without per-command code.
8. [x] (`internal/register/lifecycle_parity_test.go`) Parity suite: no manager may lack `up`/`down`/`publish` silently.
9. [x] (`internal/arch/toolliterals.go`) Extend arch tests to enforce `just`/`docker`/`compose` confinement.
10. [x] (done 2026-11-14; follow-up g: `init` does not gitignore `.universal-package-core/run/`, README tells users to add it; follow-up h: ADR-0009 line 18 stale about publish) Docs: `ARCHITECTURE.md`, `docs/README.md`, `CHANGELOG.md`, ADR-0016 to Accepted when done.

### Exit criteria
- `up` then `down` works for each manager against a fixture project; crash leaves no orphan files after `down`.
- `publish --dry-run` runs the manager's native dry-run or the documented fallback; real publish never happens in tests.
- The extended arch test confirms no `just`/`docker`/`compose` literals outside their stack packages. A project without `just` behaves identically; with `just`, a recipe is a run source below the manifest sources.

### Tests
- Resolution order table (including justfile present/absent, `just` missing).
- Run-state: start, stop, stale pid cleanup, no leftover on failure (fake runner).
- Publish confirmation, `--dry-run`, refusal without `--yes` in non-interactive mode.
- Two apps in one workspace keep separate run directories and do not clash.

### Documentation & testing
ADR-0016; `ARCHITECTURE.md` (run state, compose isolation); `docs/README.md` (`up/down`, `publish`, optional justfile); update ADR-0008 reference list in ARCHITECTURE only; `CHANGELOG.md`.

### Integration validation
Opt-in E2E: fixture dev server started and stopped via `upc up/down` on uv and npm; `publish --dry-run` on uv and npm; `doctor` clean.
**Result (2026-11-15, local, real tools):** `UPC_E2E=1 go test ./...` green. New scripts `test/testdata/e2e/{node,uv}_up_down_publish.txtar` (up, log, double-up refused, down kills the process group, crash cleanup, `publish --dry-run`, refusal without `--yes`; real publish never runs) and `just_run_fallback.txtar` (recipe vs manifest precedence). Manually verified with real tools: compose `up`/`down` against a live Docker daemon (container started, then removed). Compose is not an E2E script (needs a Docker daemon and an image pull). `doctor` clean on existing scripts.

### Risks
- Cross-platform process management (Windows signals). Mitigation: all process work in `execx`; Windows verified by the 6.0 CI job.

---

## Phase 6.4: Workspace, apps and contracts

**Depends on:** 6.2 (config layering, `-w`, minimal `workspace list`), 6.3 (`up`).

### Scope (ADR-0014; config only, no package dependencies between apps)
- Config: `[workspace] members = [...]` and `[apps.<name>]` with `path`, `port`, `url`, `provides`, `consumes`, `contract`, `env`. The `Config.Workspace` type migration (`config.go:52`) is done in 6.2; 6.4 builds the `[workspace]`/`[apps]` behavior on it.
- Interpolation: `${apps.api.url}` resolved only in config values and computed child env; unresolved variable is an error, never an empty string.
- Objects: `Workspace` (members, root), `App` (name, path, port, url, provides, consumes), `ContractChecker` (interface in `core`; implementations `openapi` and `jsonschema` in separate packages), `EnvResolver` (extend `internal/components/envs`; resolves interpolation and injects env into child processes), `Orchestrator` (starts apps via capability `up`, ordered topologically by `consumes`/`provides`; `down` in reverse; cycle detection with a clear error).
- Commands: `upc workspace list|check`, `-w/--member`, `upc up --all|--app <name>`, cross-app `doctor` (port collisions, consumer without provider, contract missing or changed, unresolved variables).

### Files
- Create `internal/workspace/` (Workspace, App), `internal/workspace/orchestrator.go`, `internal/components/envs/resolver.go`, `internal/contracts/openapi/`, `internal/contracts/jsonschema/`, `internal/cli/workspace.go`.
- Modify `internal/config/config.go`, `internal/cli/doctor.go`, `internal/cli/capability.go` (`--all`, `--app`).

### Steps
1. Update ADR-0014 draft (supersedes the deferred items of ADR-0007 and partly its option B; manager-native workspaces are still detected first).
2. Pilot: two apps on uv (`api` provides, `web` consumes) with interpolation, `up --all` order and `doctor` checks; review; fix method. Add a Node member after.
3. Contract checker: store a hash/normalized snapshot under `.universal-package-core/contracts/<app>.lock`, which is COMMITTED (ADR-0014 lists committed paths: `config.toml`, `contracts/*.lock`; gitignored paths: `local.toml`, `run/`); doctor reports "contract missing" when no lock is committed; `workspace check` compares, reports "changed" with a diff summary, never auto-updates; an explicit `--update` accepts.
4. Cross-app doctor checks, each a separate finding type.

### Exit criteria
- `upc workspace list|check`, `up --all|--app`, `-w` work on a two-app fixture.
- `git check-ignore` is false for `contracts/*.lock` and true for `local.toml` and `run/` (tested).
- Doctor reports each of: port collision, consumer without provider, contract missing, contract changed, unresolved variable.
- No manager package gained a dependency on `workspace`.

### Tests
- Config parse/migration; interpolation (nested, missing, cycles); topological order and cycle error.
- Contract checker table per implementation; orchestrator with fake runner and scripted start/stop failures (rollback of started apps).

### Documentation & testing
ADR-0014; `ARCHITECTURE.md` (workspace object model); `docs/README.md` (fullstack config example); `CHANGELOG.md`.

### Integration validation
Fixture: uv api + npm web; `up --all` starts in order, `doctor` clean, edit the contract and see `workspace check` fail.

### Risks
- Scope creep into a build system. Mitigation: no package graph between apps, config only.
- Port checks race with real processes. Mitigation: check config collisions statically; runtime probe is advisory.

---

## Phase 6.5: `app` project type

**Depends on:** 6.3 (up/down), 6.4 (apps in config optional).

### Scope
`internal/projects/app` (same shape as `script` and `packagetype`): Python uv app (`uv init --app`), Node TypeScript app with `dev`, `start`, `build` scripts; templates per language package; optional Dockerfile/compose templates. Sub-types (CLI, library) map onto script/package/app by flags, not new top-level types.

### Steps
1. Pilot: Python uv app (init, templates, golden tests).
2. Node TypeScript app (follows ADR-0010 decisions).
3. Register in `register.go` (one line); templates live under each stack's `templates/app`.

### Exit criteria and tests
`upc new app` works for uv and npm/yarn; golden scaffold tests; generated project passes `install`, `build`, `up/down` in E2E; parity test includes `app`.

### Documentation & testing
`ARCHITECTURE.md` project-type table; `docs/README.md`; template doc; `CHANGELOG.md`.

### Integration validation
E2E scripts for `new app` on uv and npm; `doctor` clean.

### Risks
Template drift against tool versions. Mitigation: golden tests plus the CI E2E job (6.0).

---

## Phase 6.6: Release and completion

### Scope
- Drift detection for all managers (venv/node_modules vs lockfile; pip vs requirements files); `doctor` reports it for every manager, not only uv.
- `upc completion` verified (cobra default or explicit command): tests for bash, zsh, fish, powershell output; docs.
- Release: `.goreleaser.yaml` (darwin/linux amd64/arm64, Windows build for portability), ldflags for the `version` command, checksums, Homebrew tap formula; `release.yml` on tag; CI `goreleaser release --snapshot --clean`; tap token only as CI secret; `v0.1.0`; `docs/RELEASING.md`. Disk footprint: static binary, `-trimpath -ldflags "-s -w"`.
- Acceptance E2E testscripts: one per `project.devlang` acceptance bullet (see table at the end).

### Steps
1. Pilot: drift for pip (the weakest), then the rest by recipe.
2. Completion tests and docs.
3. goreleaser config; `goreleaser check`; snapshot build in CI.
4. Four acceptance testscripts under `test/testdata/e2e/acceptance_*.txtar`.

### Exit criteria
Snapshot release builds in CI; `brew install` from the snapshot formula works on macOS; completion tests pass; acceptance scripts pass with `UPC_E2E=1`.

### Documentation & testing
`docs/README.md` (Homebrew install, completion), `docs/RELEASING.md`, `CHANGELOG.md` 0.1.0 entry.

### Integration validation
Fresh-machine run: install from snapshot formula, run the four acceptance scenarios, `doctor` no drift.

### Risks
Tap and token setup is an external write; requires explicit user authorization at execution time.

---

## Phase 6.7: Hardening

### Scope
- CI E2E and Windows jobs moved to Phase 6.0; here only keep them green after the changes below.
- pip: confirm interpreter resolution (`python3`, `python`, Windows `py`); fix the stack if a machine has only `pip3` or only `python3` (evidence: calls are `python -m pip`, `pip/env.go:56`, `deps.go:177,253`).
- Discarded errors: fix `internal/config/config.go:227` (if not done in 6.2) and `internal/stacks/node/yarn/list.go:145` (return the unmarshal error). No suppression.
- Split test files over 300 lines by responsibility (npm_test 585, yarn_test 551, pip_test 463, uv capabilities_test 437, poetry_test 402, cli/lifecycle_test 371); re-measure first; no test deleted or weakened.
- De-duplicate `internal/stacks/node/shared` and `internal/stacks/python/shared` helpers: identify truly common code (invocation joining, `Concat`, names) and move it to a neutral component under `internal/components/`, named by responsibility; no `_helpers`/`_utils`.
- `cmd/upc` has no test: add a main-level smoke test (version, unknown command exit code).
- CHANGELOG records the plan-03 exit-code discrepancy (code exits 2 for a cross-language switch; plan 03 says otherwise).

### Steps
Pilot the test split on `uv capabilities_test.go`; review; fix the method; apply to the rest. Pilot the shared de-duplication on one function pair, review, then the rest.

### Exit criteria
No source or test file over about 300 lines (excluding golden data); CI green on three OSes; E2E job green.

### Documentation & testing
`CHANGELOG.md`, `ARCHITECTURE.md` (shared component), `docs/README.md` (supported platforms).

### Integration validation
Full CI run on all three OSes with E2E.

### Risks
Splitting tests can lose cases silently. Mitigation: compare `go test -list` names before and after.

---

## Phase 6.8: Final documentation audit and integration validation

- `plans/README.md` already has row 6 and statuses for plans 1-5; at the end of each phase update only its status row.
- README status line; ARCHITECTURE Phase 5/6 sections; CHANGELOG (including deferred items: pnpm, bun, other languages, generated-justfile export); ADRs 0011-0016 all present and cross-linked; `docs/adrs` index.
- Post-change audit across the repo: stale docs list, tests run, final `make check`, all E2E, acceptance scripts, snapshot release.
- Final audit confirms the `plans/README.md` status rows match the finished phases.

### Exit criteria
Docs match code; four acceptance statements demonstrated; `0.1.0` installable via Homebrew; deferred items recorded.

---

## Requirements traceability (`project.devlang`)

| Requirement | Delivered by |
|---|---|
| Spin up any project type with any stack | script/package (done), `app` type in 6.5; all stacks via 6.1 data |
| Install and manage packages how I need (constraints, groups) | 6.1 constraints, 6.2 groups |
| Per-stack commands, hints, defaults not hardcoded | 6.1 (D3) |
| Spin a project up/down locally in different environments | 6.2 environments, 6.3 `up/down`, 6.4 `up --all` |
| Environment never out of line between PM and upc | 6.2 doctor checks, 6.6 drift for all managers |
| One proper venv via the PM, no extra venvs (`project.devlang` L73) | 6.2 (no second venv test and exit criterion) |
| Environment never out of line, including env groups | 6.2 doctor group checks, 6.6 drift |
| Multiple apps / fullstack in one folder | 6.4 workspace and apps |
| Config in hidden dir, secrets not committed | 6.2 layering and `local.toml` |
| Publish a project | 6.3 `publish` |
| Homebrew distribution | 6.6 |
| Optional justfile support (user decision D6, not a `project.devlang` requirement) | 6.3 |
| Run-source lookup order (ADR-0008) | 6.3 (ADR-0016 restates it per stack) |
| Environment variables and secrets: not our concern (`project.devlang`) | 6.2 (ADR-0011 note: deliberate narrowing; upc never reads or prints values) |
| Quality (CI, Windows, E2E, file size) | 6.0 gate, 6.7 |
| Docs, ADRs, CHANGELOG | every phase; audit in 6.8 |

## Open questions

1. npm/yarn group design is decided per manager (npm and berry B, classic A); doc claims are confirmed in the 6.2 pilot.
2. Windows is verified by CI only; Homebrew is still the only distribution channel.
