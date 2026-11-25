# Installing upc

Platforms: macOS and Linux (amd64, arm64). Windows is not supported yet.

## Homebrew (recommended)

The tap is this repository (`Formula/upc.rb`).

```sh
brew tap justinfischman/upc https://github.com/justinfischman/universal-package-core
brew install justinfischman/upc/upc
upc version
```

### Upgrade

```sh
brew update
brew upgrade upc
```

### Uninstall

```sh
brew uninstall upc
brew untap justinfischman/upc     # optional: remove the tap
```

## go install

Needs a Go toolchain that satisfies the `go` line in `go.mod`.

```sh
go install github.com/justinfischman/universal-package-core/cmd/upc@latest
```

This installs `upc` to `$(go env GOBIN)`, or `$(go env GOPATH)/bin` when `GOBIN` is unset; make sure that directory is on your `PATH`. It was run on 2026-11-25 with an empty `GOPATH` and module cache and succeeded, resolving `v0.1.0`. A binary built this way has no release ldflags, so `upc version` prints `upc dev` instead of the release version and commit. Use Homebrew if you need the version reported. To remove it, delete the `upc` file from that directory.

## Manual binary removal

If you placed a binary yourself, find it with `command -v upc` and delete it.

## Removing configuration and state

`upc` keeps global settings in `~/.universal-package-core/` (override with `UPC_HOME`). Removing the package does not touch it. To clear it:

```sh
rm -rf ~/.universal-package-core/
```

Per-project settings and run state live in each project's `.universal-package-core/` directory (and `pyproject.toml` or `package.json` entries); delete them by hand if you no longer want them.

## Shell completion

`upc completion bash|zsh|fish|powershell`, see [COMPLETION.md](COMPLETION.md).
