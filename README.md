# upc (universal-package-core)

One place to create, install and manage packages and apps across languages. `upc` routes to the native package managers (uv, poetry, pip, npm, yarn). It never replaces them.

## Install

```sh
brew tap justinfischman/upc https://github.com/justinfischman/universal-package-core
brew install justinfischman/upc/upc
upc version
```

Supported platforms: macOS and Linux, amd64 and arm64 (`.goreleaser.yaml`). Windows is not supported yet. Upgrade, uninstall and `go install`: [docs/INSTALL.md](docs/INSTALL.md).

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

| Command | Purpose |
|---|---|
| `new`, `init` | Create a project, or set up upc for an existing one |
| `add`, `remove`, `list`, `install`, `update`, `outdated` | Manage dependencies |
| `run`, `build`, `test`, `lint`, `audit` | Run project tasks |
| `up`, `down` | Start and stop the dev process |
| `publish` | Build, then publish to a registry (asks first) |
| `doctor` | Check tools, configuration and drift |
| `config`, `env`, `workspace` | upc settings, environments, multi-app workspaces |
| `completion`, `version` | Shell completion, version |

Global flags: `--dry-run`, `--global`, `--json`, `--project <dir>`, `-v/--verbose`, `-y/--yes`.

Exit codes: 0 success, 1 failure, 2 usage or incompatible request, 3 unsupported, 4 tool missing (`internal/core/errors.go`).

## Documentation

- [docs/README.md](docs/README.md): goals, design and status
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md), [docs/adrs/](docs/adrs/)
- [docs/COMPLETION.md](docs/COMPLETION.md), [docs/CHANGELOG.md](docs/CHANGELOG.md), [docs/RELEASING.md](docs/RELEASING.md)
- [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md)

## License

[MIT](LICENSE)
