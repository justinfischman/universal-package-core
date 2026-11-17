# Releasing upc

Releases are built by GoReleaser (`.goreleaser.yaml`) from a `v*` tag via
`.github/workflows/release.yml`. Artifacts: darwin and linux (amd64, arm64),
`checksums.txt`, and a Homebrew cask. Binaries are built
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

## 2. Homebrew tap (no setup needed)

The tap lives in this repository: on a tagged release GoReleaser commits
`Casks/upc.rb` to `main` using the workflow's built-in `GITHUB_TOKEN`
(`contents: write`). There is no separate tap repository and no secret to add.
If branch protection on `main` blocks that push, allow GitHub Actions to bypass
it or switch to a fine-grained token in a secret.

## 3. Cut v0.1.0 (OWNER ACTION REQUIRED)

1. Merge the release changes to `main`; confirm CI (including
   `release-snapshot`) is green.
2. OWNER ACTION: tag and push, which triggers the publish workflow:

   ```sh
   git tag -a v0.1.0 -m "v0.1.0"
   git push origin v0.1.0
   ```

3. Confirm the GitHub release has all archives and `checksums.txt`, and that
   `Casks/upc.rb` was committed to `main`.

## 4. Verify the install

```sh
brew tap justinfischman/upc https://github.com/justinfischman/universal-package-core
brew install --cask justinfischman/upc/upc
upc version        # upc 0.1.0, with commit and date
```

## Notes

- The tap uses `homebrew_casks` (the `brews` option is deprecated). The cask
  is not code-signed; if macOS Gatekeeper blocks `upc`, check the first
  install and add a quarantine-removal post-install hook if needed.
- Pin updates: action versions are pinned to major tags in the workflows.
