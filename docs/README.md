# universal-package-core (`upc`)

One place to create, install and manage packages and apps across languages. `upc` routes to the native package managers (uv, poetry, pip, npm, yarn, ...). It never replaces them.

**Status: Phase 6 (6.0-6.7 built; 6.8 final audit pending).** Working today: Node (TypeScript) with `npm` or `yarn` (classic), and Python with `uv`, `pip` (keeps `requirements.txt` and a `.venv` for you) and `poetry`; the `script`, `package` and `app` project types: `new`, `init`, `add`, `remove`, `list`, `install`, `update`, `outdated`, `run`, `build`, `test`, `lint`, `audit`, `up`, `down`, `publish`, `doctor` (including environment drift), `config` (including `config set pm`), `env list|use|show|add|remove` with `--env`, `workspace check`, `version`, `completion` and the global flags. Not yet released: the Homebrew tap and `v0.1.0` wait for owner actions (`docs/RELEASING.md`). Deferred: npm/yarn dependency groups, `doctor` group checks, `local.toml`, `config show --origin`, pnpm and bun. See `plans/06-agnostic-core-environments-multiapp-release.md`.

**Supported platforms:** macOS and Linux (built, linted and tested in CI, plus an E2E job against real uv, poetry, npm and yarn). Windows is verified by CI only (build, vet, lint and unit tests); Homebrew is the only distribution channel.

## Goals
- Create easily installable apps and packages (installable by `upc`).
- Manage packages in other projects with one tool.

## How it works
- The native PM owns manifests, lockfiles, versions, sync and upgrades. `upc` reads live PM state and does not cache a competing copy.
- If a PM or runtime is missing, `upc` asks to install it, then continues.
- Language, PM and project-type code each live in their own package; adding a new one touches one package plus registration. See `docs/ARCHITECTURE.md`.

## Usage
```sh
upc new script hello --yes        # python + uv by default; creates ./hello/hello.py
upc new app web --language python --pm uv --yes   # server, Dockerfile, compose.yaml, dev/start scripts
cd web && upc up 8080             # start the dev/start script in the background with a port argument
upc down                          # stop it
upc new package mylib --pm poetry --yes   # src layout, tests, README, license placeholder
upc new package mylib --language node --pm npm --yes   # TypeScript library: tsc build, node:test tests
upc build && upc test                 # generic commands, routed to the project manager
upc config set pm uv                  # switch managers within a language
cd hello
upc add rich                      # uv add --script hello.py rich
upc add rich --dry-run            # print the routed command, change nothing
upc list                          # dependency tree
upc run hello.py                  # file, then [tool.upc.scripts], then [project.scripts]
upc outdated --json               # machine-readable
upc remove rich
upc doctor                        # tool, config drift, capability matrix; --yes applies safe fixes

cd existing-uv-project && upc init   # records the choice, runs install, edits no manifest
upc add rich -v                   # --verbose logs every routed command
```
If `uv` is missing, `upc` offers to install it and then continues; with `--yes` it installs without asking, and non-interactively without `--yes` it fails with instructions (exit code 4). Unsupported capabilities (for example `up` and `down`) fail with exit code 3. `test` and `lint` need pytest / ruff to be declared by the project (or a `test`/`lint` script); otherwise they fail with exit code 3 and say how to enable them. A new `pip` package has no `.venv` and a new `poetry` package has no installed dependencies until `upc install`; run it before `upc test`. A cross-language `config set pm` is refused with exit code 2.

### Run, up, down and publish
```sh
upc run build-docs                # file, config/manifest script, then a justfile recipe (optional)
upc up                            # starts the "up", "dev" or "start" script in the background
upc up --dry-run                  # print the command, start nothing
upc down                          # stops what `upc up` started; a crashed process just has its record cleaned
upc up -w api                     # -w/--member works on up, down, publish and the other capability commands
upc publish --dry-run             # print the build and publish commands, run nothing
upc publish                       # build, ask for confirmation, then publish with the manager's own login
upc publish --yes                 # skip the question (required when not interactive)
```
- `up` records `up.pid` and `up.log` under `.universal-package-core/run/` in the app directory (`upc init` adds it to `.gitignore`). `down` removes `up.pid` and keeps `up.log`. `scripts.<name>` and `up` in `config.toml` (argv lists) take precedence over manifest scripts.
- A `justfile` is optional. If one exists and `just` is installed, its recipes are the last source `upc run` tries, after the project's own scripts; without `just` nothing changes.
- With no `up`/`dev`/`start` script, `upc up` falls back to `docker-compose.yml` or `compose.yaml` (`docker compose up`); `upc down` then runs `docker compose down`.
- `publish --dry-run` currently only prints the plan; it does not call the manager's native dry run. Credentials are never stored.

### Node (TypeScript) with npm
```sh
upc new package mylib --language node --pm npm --yes   # npm init, tsconfig.json, src/index.ts, test/index.test.ts, typescript installed
cd mylib
upc add zod                       # npm install zod (--dev: -D)
upc build                         # the build script, else tsc when tsconfig.json exists
upc test                          # npm test (node:test)
upc outdated --json               # npm outdated --json (exit 1 from npm is handled)
upc lint                          # runs the lint script; without one: exit 3 with guidance

upc new script tool --language node --pm npm --yes   # tool/tool.ts with a private package.json
cd tool && upc run tool.ts World  # node --experimental-strip-types tool.ts World
upc run start                     # a package.json script, through npm run
```
Node 22.6 or newer runs scripts and 22.18 or newer runs package tests (ADR-0010). `upc` reads `package.json` but changes it only through `npm`. A new package installs `typescript` and needs network access. `config set pm npm` in a Python project (and the reverse) is refused with exit code 2.

With yarn: `upc new package mylib --language node --pm yarn --yes`, then the same commands (`yarn add`, `yarn test`, ...). A directory with both `yarn.lock` and `package-lock.json` asks which manager to use.

### Environments
```sh
upc env list                      # declared ([environments.<name>] in .universal-package-core/config.toml) and .env.<name> files
upc env use dev                   # validates it exists, then records it as active in the project config
upc env show                      # active environment: name and file path
upc env add qa --file config/qa.env --groups test,docs   # declare an environment (groups are dependency groups)
upc env remove qa                 # drop the declaration; the env file is never touched
upc install --env qa              # install that environment's groups (uv, poetry, pip; npm/yarn reject groups)
upc up --env qa 8123              # --env also works on run, test and up, for one command; flags go before the script/args
```
`upc` never reads or prints env values. While an environment is active, every command it runs for you gets `UPC_ENV=<name>`. `doctor` warns when the active environment or its file is gone. Config: `[environments.dev] file = "config/dev.env"` and `default_environment = "dev"` (ADR-0006).

### Workspaces and apps
A folder with several apps (for example a uv `api` and an npm `web`) is described in `.universal-package-core/config.toml`:
```toml
[workspace]
members = ["apps/api", "apps/web"]

[apps.api]
path = "apps/api"
port = 8000
url = "http://localhost:${apps.api.port}"
provides = ["http"]
contract = "openapi.yaml"          # relative to the app; OpenAPI or JSON Schema

[apps.web]
path = "apps/web"
port = 3000
consumes = ["api"]                 # api starts first

[apps.web.env]
API_URL = "${apps.api.url}/v1"     # resolved when web starts
```
```sh
upc workspace list                # members and apps in start order
upc up --all                      # start every app, providers first; a failed start stops the ones already started
upc up --app web                  # start only web (not the apps it consumes)
upc down --all                    # stop in reverse order
upc workspace check               # fail if a contract lock is missing or the contract changed
upc workspace check --update      # accept the current contracts and write the locks
```
- Interpolation: `${apps.<name>.<field>}` with field `name`, `path`, `port` or `url`, in `url` and `env` values. An unresolved variable is an error, never an empty string. `${` has no escape syntax, so a literal `${` cannot be written in these values.
- `--app <name>` starts or stops only that app.
- Contracts: `.universal-package-core/contracts/<app>.lock` is committed. `workspace check` never writes it; only `--update` does. `doctor` reports `contract-missing` and `contract-changed`, next to `port-collision`, `consumer-without-provider` and `unresolved-variable`.
- YAML contracts are parsed by `go.yaml.in/yaml/v3`: anchors, aliases and merge keys are resolved; numbers take their canonical form; multi-document files, empty files, duplicate keys and non-string keys are errors (see ADR-0014).

## Commands
`init`, `new {script|package|app}`, `add|remove|list|install`, `update|outdated|audit`, `run`, `build|test|lint`, `up|down`, `env`, `config`, `workspace`, `doctor`, `publish`, `completion`, `version`.

Global flags: `--dry-run`, `--json`, `--yes`, `--verbose`, `--project {path}`, `--global`. Non-interactive mode for CI.

Supported stacks: Python with `uv`, `pip` and `poetry`; Node with `npm` and `yarn` (classic 1.x; berry only for update, audit and publish). Next: other PMs. Project types: `script`, `package` and `app` are built.

## Storage
Global `~/.universal-package-core/`, per-project `.universal-package-core/` (project wins). Filesystem only.

## Development
Requires Go 1.27+ and `golangci-lint` v2.
- `make build` builds `bin/upc` (version info via ldflags).
- `make check` runs gofmt, `go vet`, `golangci-lint` and all tests, including the architecture test and the default testscript E2E in `test/`.
- `UPC_E2E=1 go test ./test/...` also runs the real-manager scripts in `test/testdata/e2e` (needs `uv` and network).
- Golden files: `go test ./internal/components/output -update` rewrites them; review the diff before committing.
- `UPC_HOME` overrides the global config directory, which keeps tests and experiments away from `~/.universal-package-core/`.

## Install
Homebrew only. **Planned:** the tap does not exist yet, so `brew install` does not work until the owner creates it and cuts `v0.1.0` (steps in `docs/RELEASING.md`). Until then build from source with `make build`.

Shell completion: `upc completion bash|zsh|fish|powershell`, see `docs/COMPLETION.md`.

### Doctor: drift
`upc doctor` reports drift for every manager by reading files only (no tool is run): the lockfile or requirements file against `.venv` / `node_modules`, and manager files that disagree with the configured manager. uv and poetry check transitive packages; npm and yarn check only declared dependencies; projects without `node_modules` (including Plug'n'Play) report nothing. See `docs/ARCHITECTURE.md` section 16.

## Documentation map
- `docs/ARCHITECTURE.md`: design and layering
- `docs/adrs/`: decisions
- `docs/COMPLETION.md`: shell completion
- `docs/RELEASING.md`: release and Homebrew tap steps
- `docs/CHANGELOG.md`: changes
- `plans/`: ordered phase plans
- `project.devlang`: source of truth for requirements
