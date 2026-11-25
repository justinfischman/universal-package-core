# ADR-0019: Additional language stacks

Status: Proposed 2026-11-25 (design plus spike only, per Plan 07 D7.4; no language is registered)

## Context
`upc` ships two languages, Python (uv, poetry, pip) and Node (npm, yarn). ADR-0002 and ADR-0013 promise that a language is added by dropping a package under `internal/stacks/` and registering it. Plan 07 D7.4 asks whether that promise holds for languages that differ from Python and Node, without shipping one.

The contract a stack meets today (`internal/core`): a `Language` (id, manager ids, default manager, `Detect`), optional `Templater`, and a `PackageManager` that declares every one of the 15 capabilities as native, filled or unsupported (ADR-0003), declares marker files, and may implement `Diagnoser` and `DetectRanker`.

## Candidates
| Language | Manager | Fit | Main differences |
|---|---|---|---|
| Go | `go` modules (`gomod`) | Fits. Spiked. | No scripts, no dependency groups, no lock file (`go.sum` is a checksum record), versions are exact or `@latest` (minimal version selection), `go.work` for workspaces. |
| Rust | `cargo` | Fits (not spiked). | Native `add`, `remove`, `update`, `build`, `test`, `run`, `publish`, `audit` only with `cargo-audit`. `[dev-dependencies]` and `[build-dependencies]` map onto `Request.Group`; `Cargo.lock` is a real lock file for `DetectRank`. `[workspace]` matches ADR-0007. |
| Java | maven, gradle | Fits with cost. | Two managers per language like Node, but build files are XML or a Groovy/Kotlin script that `upc` must not edit, so `add` and `remove` would be unsupported or filled by printing guidance. |
| Ruby | bundler | Fits. | `Gemfile` groups map onto `Group`. |
| .NET | `dotnet` | Fits. | One tool covers every capability; project files are XML. |

## Decision
1. **No interface change is required for Go or Rust.** The spike (`internal/stacks/golang`) implements `Language` and `PackageManager` against the unchanged `core` package, with `Templater` omitted, and passes the architecture test. Optional interfaces (`Templater`, `Diagnoser`, `DetectRanker`) already carry the variation.
2. **A stack that has no analogue for a concept declares it unsupported or rejects the request**, never ignores it. The spike returns `ErrInvalidRequest` for `Group` and for `Script` and marks `publish`, `up`, `down`, `audit` and `outdated` unsupported with a reason.
3. **Go is the first additional language, Rust the second.** Both drive one binary that is already the manager, so the existing `invoke` and `execx` components cover them with no new shared code.
4. **Java, Ruby and .NET are deferred** until a user asks; each needs its own design for build files that upc must not rewrite.
5. **Nothing is registered or exposed in this ADR.** Registration follows ADR-0002: one line in `internal/register`, plus per-language templates and a lifecycle-parity test row.

## Gaps the spike found (not blocking; fix when a language is registered)
- **Project types.** `script` has no Go analogue (no inline-metadata scripts). A language should be able to say it offers only `package` and `app`. Today absence of `Templater` or an unknown project type is the only signal; confirm the init flow reports it as a clear error.
- **Neutral constraints (ADR-0012).** `Request.Version` ranges (`>=2.3.4`) cannot map to Go, which resolves to one version. The Go manager would not implement `Versioned`, so the CLI must refuse ranges for it, as it does for any manager that lacks it.
- **Run lookup (ADR-0008, ADR-0016).** Go has no named scripts. `run` takes the main package only; named tasks come from the justfile source.
- **Outdated and audit.** `go list -m -u -json all` could back `outdated` (needs a stream decoder, `go list` prints concatenated objects) and `govulncheck` could back `audit` as a tool the user installs. Both are deferred, so the spike declares them unsupported.
- **Word and marker leaks.** `go`, `gomod`, `go.mod` become module ids and markers once registered, so the arch word and marker tests will then police them. `go` is a very common word; `CheckWords` must be checked for false positives before registration.
- **Package naming.** `go` is a Go keyword, so the package is `golang` while the language id stays `go`.
- **Tool installation.** The spike gives an install hint only; an `InstallCmd` per platform (`brew install go`) is needed before `upc doctor` can fix a missing tool.

## Recommendation
Accept the design. Do not register Go in v0.2. Treat the spike as proof, then in a later plan: register Go with `init`, `add`, `remove`, `install`, `list`, `run`, `build`, `test`, `lint`, a `package` template and a lifecycle-parity row, resolve the project-type and constraint gaps above, then repeat for Rust.

## Consequences
- `internal/stacks/golang` stays in the tree, unregistered, with tests, so the contract fit keeps being checked on every `go test ./...`. It is dead code to users until registered; delete it if the plan to register is dropped.
- The architecture test treats it as a concrete module: it imports only `core` and `components`, never `register` or `cli`.
- No documentation or CLI listing changes until registration.

## Related
- ADR-0002 (isolation and registration), ADR-0003 (capability model), ADR-0005 (templates), ADR-0012 (neutral constraint mapper), ADR-0013 (data-driven stacks and tool-name confinement), ADR-0014 (workspaces).
