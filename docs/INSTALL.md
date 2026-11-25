# Installing upc

Platforms: macOS and Linux (amd64, arm64) via Homebrew and `scripts/install.sh`. Windows (amd64, arm64) is released as a `.zip` archive; download it from the release page, verify it against `checksums.txt`, and unpack it yourself. `install.sh` does not handle Windows.

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

## Install script (curl)

`scripts/install.sh` installs a release archive for macOS or Linux. It downloads the archive and `checksums.txt`, verifies the archive's sha256, and installs only on a match; any failure aborts with nothing installed. It does not use sudo: the default target is `$HOME/.local/bin`.

Prefer a tag-pinned script URL so the script you run matches the release you install (replace `v0.2.0`):

```sh
curl -fsSL https://raw.githubusercontent.com/justinfischman/universal-package-core/v0.2.0/scripts/install.sh | UPC_VERSION=v0.2.0 sh
```

The installer overwrites an existing `upc` in the target directory. Set `UPC_VERSION` (for example `v0.2.0`; default is the latest release) and `UPC_INSTALL_DIR` to change the version or directory. To read the script before running it, download it first and run `sh install.sh`. Releases are not signed yet (signing is deferred until the repository is public); the checksum protects against corruption, not against a compromised release.

## Verify a download manually

Each release publishes `checksums.txt` (sha256) and one SPDX SBOM per archive (`*.sbom.json`). With the archive and `checksums.txt` in one directory:

```sh
# Linux
sha256sum --ignore-missing -c checksums.txt
# macOS
shasum -a 256 --ignore-missing -c checksums.txt
```

The line for your archive must print `OK`. The SBOMs are an unsigned inventory of what is in each archive, not build provenance.

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
