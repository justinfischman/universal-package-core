# Releasing upc

Releases are built by GoReleaser (`.goreleaser.yaml`) from a `v*` tag via
`.github/workflows/release.yml`. Artifacts: darwin and linux (amd64, arm64),
windows (amd64), `checksums.txt`, and a Homebrew cask. Binaries are built
with `CGO_ENABLED=0`, `-trimpath`, and `-s -w`; `version`, `commit` and `date`
are injected into `main.version`, `main.commit` and `main.date`
(`cmd/upc/main.go`), which `upc version` prints.

## 1. Test locally (no publishing)

```sh
goreleaser check
goreleaser release --snapshot --clean --skip=homebrew
dist/upc_<os>_<arch>*/upc version     # prints upc 0.0.0-SNAPSHOT-<commit>
```

`goreleaser check` needs a git remote (`origin`) to be configured. CI runs the
same snapshot in the `release-snapshot` job of `ci.yml`.

## 2. One-time Homebrew tap setup (OWNER ACTION REQUIRED)

These steps create remote resources and secrets. They are not automated and
must be done explicitly by the repository owner.

1. OWNER ACTION: create a public GitHub repository `justinfischman/homebrew-tap`.
2. Done: `.goreleaser.yaml` (`homebrew_casks[0]`) already names `justinfischman`.
3. OWNER ACTION: create a fine-grained personal access token with
   Contents: read/write on `justinfischman/homebrew-tap` only.
4. OWNER ACTION: add it as the Actions secret `HOMEBREW_TAP_GITHUB_TOKEN` in
   this repository. The token is read only from that environment variable;
   never commit it.

## 3. Cut v0.1.0 (OWNER ACTION REQUIRED)

1. Merge the release changes to `main`; confirm CI (including
   `release-snapshot`) is green.
2. OWNER ACTION: tag and push, which triggers the publish workflow:

   ```sh
   git tag -a v0.1.0 -m "v0.1.0"
   git push origin v0.1.0
   ```

3. Confirm the GitHub release has all archives and `checksums.txt`, and that
   the tap repository received `Casks/upc.rb`.

## 4. Verify the install

```sh
brew install --cask justinfischman/tap/upc
upc version        # upc 0.1.0, with commit and date
```

## Notes

- The tap uses `homebrew_casks` (the `brews` option is deprecated). The cask
  is not code-signed; if macOS Gatekeeper blocks `upc`, check the first
  install and add a quarantine-removal post-install hook if needed.
- Pin updates: action versions are pinned to major tags in the workflows.
