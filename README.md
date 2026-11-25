# upc (universal-package-core)

One place to create, install and manage packages and apps across languages. `upc` routes to the native package managers (Python: uv, pip, poetry; Node: npm, yarn, pnpm, bun). It never replaces them.

## Install

```sh
brew tap justinfischman/upc https://github.com/justinfischman/universal-package-core
brew install justinfischman/upc/upc
upc version
```

Supported platforms: macOS, Linux and Windows, amd64 and arm64 (`.goreleaser.yaml`). Windows ships as a `.zip` release archive (unpack it yourself; Windows CI is unverified until its first run, ADR-0017). On macOS and Linux you can also run the checksum-verifying `scripts/install.sh`. Upgrade, uninstall and `go install`: [docs/INSTALL.md](docs/INSTALL.md).

## Quickstart

```sh
upc new package mylib --language python --pm uv   # scaffold a project
cd mylib
upc add requests                                  # add a dependency
upc add --dev pytest                              # add a test dependency
upc test                                          # run the tests
upc doctor                                        # check tools, config and drift
```

Add `--dry-run` to any command to see what would run without changing anything. Run `upc <command> --help` for flags.

## Commands

Every command also takes the global flags below. `-w/--member` selects a workspace member.

| Command | Purpose | Flags |
|---|---|---|
| `new <type> [name]` | Create a project; `type` is `script`, `package` or `app` | `--language`, `--pm`, `--name`, `--kind cli\|library` (app only; default: a service) |
| `init` | Set up upc for an existing project | none |
| `add <package>...` | Add dependencies | `--version`, `--constraint`, `-D/--dev`, `--group`, `-w`, `--script` |
| `remove <package>...` | Remove dependencies | `-D/--dev`, `--group`, `-w`, `--script` |
| `list`, `outdated` | List dependencies, show newer versions | `-w`, `--script` |
| `install` | Install dependencies | `-D/--dev`, `--group`, `--env`, `-w`, `--script` |
| `update [package]...` | Upgrade all or the named dependencies | `-w`, `--script` |
| `run <script> [args...]` | Run a file or named script | `--env`, `-w` |
| `build`, `lint`, `audit` | Run project tasks | `-w` |
| `test` | Run the tests | `--env`, `-w` |
| `up`, `down` | Start and stop the dev process | `up`: `--all`, `--app`, `--env`, `-w`; `down`: `--all`, `--app`, `-w` |
| `publish` | Build, then publish to a registry (asks first) | `-w`, `--native-dry-run` (build, then the manager's own publish check; sends nothing) |
| `doctor` | Check tools, configuration, drift and capabilities | none |
| `config get [key]`, `config set <key> <value>`, `config validate` | Read and change upc settings | none |
| `config show` | Show every setting | `--origin` (the layer each value came from) |
| `env` (bare), `env list`, `env show`, `env use <name>`, `env remove <name>` | Manage environments | none |
| `env add <name>` | Declare an environment | `--file`, `--groups` |
| `export` | Write the project's commands as a justfile of `upc` recipes | `--force` (overwrite) |
| `workspace list` | List workspace members and apps in start order | none |
| `workspace check` | Compare each app's contract with its committed lock | `--update` (accept the current contracts) |
| `completion <shell>`, `version` | Shell completion, version | `completion`: `--no-descriptions` |

Capability parity across the seven managers, including the exceptions: [docs/README.md](docs/README.md#capability-parity).

Global flags: `--dry-run`, `--global`, `--json`, `--project <dir>`, `-v/--verbose`, `-y/--yes`.

Exit codes: 0 success, 1 failure, 2 usage or incompatible request, 3 unsupported, 4 tool missing (`internal/core/errors.go`).

## Documentation

- [docs/README.md](docs/README.md): goals, design and status
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md), [docs/adrs/](docs/adrs/)
- [docs/COMPLETION.md](docs/COMPLETION.md), [docs/CHANGELOG.md](docs/CHANGELOG.md), [docs/RELEASING.md](docs/RELEASING.md)
- [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md)

## License

[MIT](LICENSE)
