# Releasing upc

Releases are built by GoReleaser (`.goreleaser.yaml`) from a `v*` tag via
`.github/workflows/release.yml`. Artifacts: darwin, linux and windows (amd64, arm64; `.tar.gz`, and `.zip` for Windows),
`checksums.txt`, one SPDX SBOM per archive (`*.sbom.json`, from syft), and a Homebrew formula. Binaries are built
with `CGO_ENABLED=0`, `-trimpath`, and `-s -w`; `version`, `commit` and `date`
are injected into `main.version`, `main.commit` and `main.date`
(`cmd/upc/main.go`), which `upc version` prints.

## 1. Test locally (no publishing)

```sh
goreleaser check
goreleaser release --snapshot --clean --skip=homebrew
dist/upc_<os>_<arch>*/upc version     # prints upc 0.0.0-SNAPSHOT-<commit>
```

`goreleaser check` needs a git remote (`origin`) to be configured. It also
exits non-zero for the accepted `brews` deprecation warning (see Notes); the
snapshot command still succeeds. CI runs the same snapshot in the
`release-snapshot` job of `ci.yml`.

Snapshot builds need `syft` on `PATH` for the `sboms:` step (CI installs it with
`anchore/sbom-action/download-syft`; locally `brew install syft`). Check that
`dist/*.sbom.json` exists next to each archive. Artifacts are not signed: cosign
signing is deferred until the repository is public (plan 07, D7.3).

## 2. Homebrew tap (no setup needed)

The tap lives in this repository: on a tagged release GoReleaser commits
`Formula/upc.rb` to `main` using the workflow's built-in `GITHUB_TOKEN`
(`contents: write`). There is no separate tap repository and no secret to add.
If branch protection on `main` blocks that push, allow GitHub Actions to bypass
it or switch to a fine-grained token in a secret.

## 3. Cut a release (OWNER ACTION REQUIRED)

v0.1.0 was cut this way on 2026-11-17. The workflow ran green and pushed
`Formula/upc.rb` to `main`. Branch protection was not in play (the plan the
repository was on does not offer it). A private repository cannot serve
the release assets, so `brew install` works only while the repository is public.

1. Merge the release changes to `main`; confirm CI (including
   `release-snapshot`) is green.
2. OWNER ACTION: tag and push, which triggers the publish workflow:

   ```sh
   git tag -a vX.Y.Z -m "vX.Y.Z"    # next planned: v0.2.0
   git push origin vX.Y.Z
   ```

3. Confirm the GitHub release has all archives and `checksums.txt`, and that
   `Formula/upc.rb` was committed to `main`.

## 4. Verify the install

```sh
brew tap justinfischman/upc https://github.com/justinfischman/universal-package-core
brew install justinfischman/upc/upc
upc version        # upc <version>, with commit and date
brew test upc
```

## 5. Verify checksums of a release

```sh
gh release download v0.2.0 --repo justinfischman/universal-package-core \
  --pattern checksums.txt --pattern 'upc_*_linux_amd64.tar.gz' --dir upc-verify
cd upc-verify
sha256sum --ignore-missing -c checksums.txt    # macOS: shasum -a 256 --ignore-missing -c checksums.txt
```

Expect `OK` for each archive. `scripts/install.sh` performs the same check and
refuses to install on a mismatch or a missing entry. Confirm the release also
lists `*.sbom.json` assets.

## Notes

- The tap uses a formula (`brews`). GoReleaser reports `brews` as deprecated in
  favour of `homebrew_casks`; the warning is accepted because a formula avoids
  the Gatekeeper quarantine that an unsigned cask would get. Revisit if `brews`
  is removed.
- While the repository is private, `brew install` of the formula needs a
  GitHub token (`HOMEBREW_GITHUB_API_TOKEN`) and the release assets are not
  publicly downloadable; make the repository public before announcing the tap.
- Pin updates: actions and the syft and goreleaser versions are pinned (actions by commit SHA with a version comment) in `release.yml` and `ci.yml`; bump them together, dependabot proposes the action updates.
