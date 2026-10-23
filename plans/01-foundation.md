# Phase 1: Foundation

**Depends on:** nothing. **Blocks:** all later phases.
**Goal:** a building, tested, linted CLI skeleton with the full core abstraction layer, so Phase 2 only adds modules.

## Repo fixes (first)

1. `cmd/upc/main.go`: change `package upc` to `package main`; `main()` calls `register.Default()` then `cli.Execute(ctx, reg, os.Args[1:], os.Stdout, os.Stderr)` and `os.Exit(code)`. Use `signal.NotifyContext` (SIGINT/SIGTERM) for the context.
2. `.gitignore`: replace `/target` with `/bin`, `/dist`, `*.test`, `coverage.out`.
3. `go.mod`: keep module path; add `github.com/spf13/cobra` and `github.com/pelletier/go-toml/v2`; `go mod tidy`. Dev-only: `github.com/rogpeppe/go-internal` (testscript).
4. Add `scripts/check.sh` (fmt, vet, lint, test, arch test) and a `Makefile` with `build`, `test`, `lint`, `check`. `golangci-lint` 2.15.0 is installed locally (v2 config format: `version: "2"`); add `.golangci.yml` (govet, staticcheck, errcheck, revive, depguard for isolation backup, gocritic, ineffassign).
5. Add `.github/workflows/ci.yml` running `scripts/check.sh` on macOS and Linux (and Windows build-only; the spec says any OS).

## Package build order

Build in this order; each step compiles and has tests before the next.

### 1. `internal/core`
- `module.go`: `Kind`, `Module`.
- `capability.go`: `Capability`, `AllCapabilities`, `Support` (`Native|Filled|Unsupported`), `CapabilitySet`.
- `request.go`: `Request`, `Result` (fields per ADR-0003: `Dir`, `Packages []string`, `Group`, `Script`, `Args []string`, `DryRun`, `Yes`; `Result{Summary string; Data any; Commands []string}`).
- `errors.go`: sentinel and typed errors with `errors.Is/As`: `ErrUnsupported`, `ErrInvalidRequest`, `ErrIncompatible`, `ToolMissingError{Tool, InstallHint}`, plus `ExitCode(err) int` mapping (ARCHITECTURE section 6).
- `packagemanager.go`: `PackageManager` interface + `BasePackageManager` embedded struct implementing the capability table (`Native/Filled/Unsupported/Dispatch`).
- `language.go`, `projecttype.go`, `packagefile.go`: interfaces from ARCHITECTURE section 3.
- `registry.go`: generic `Registry[T Module]` with `Register` (rejects duplicate ids), `Get`, `List` (sorted), plus `Registries{Languages, PackageManagers, ProjectTypes}`.
- **Tests:** registry (dup, missing, ordering); base PM (dispatch, unsupported error text); `ExitCode` table; capability set completeness helper.

### 2. `internal/components`
Each is an interface + default implementation + fake, in its own subpackage.
- `execx`: `Runner.Run(ctx, Cmd) (Output, error)`; `Cmd{Name, Args, Dir, Env, Stream bool}`. Default uses `exec.CommandContext`; respects `DryRun` (logs and returns without running); logs `slog.Info("route", ...)` with name/args/dir; maps `exec.ErrNotFound` to `core.ToolMissingError`. `FakeRunner` records calls and returns scripted outputs (use in all later tests).
- `logx`: `New(verbose, json bool, w io.Writer) *slog.Logger`; `--verbose` = debug level; default = info; logger passed down, not global.
- `prompt`: `Prompter{Confirm(msg, def) (bool, error); Choose(msg, opts) (string, error)}`. `Terminal` impl; `NonInteractive` impl (auto-yes when `--yes`, otherwise returns an error naming the flag). Choose implementation: TTY detection with `golang.org/x/term`.
- `fsx`: `FS` interface (read, write atomic, mkdirall, stat, walk) + OS impl; tests use temp dirs.
- `render`: `Render(fs.FS, dest, data)` with `text/template`; refuses to overwrite existing files unless allowed; stages in temp dir and moves.
- `output`: `Printer{Print(Result)}` with text and JSON modes (data stdout, logs stderr).
- **Tests:** each component incl. dry-run, not-found mapping, atomic write failure leaves no partial file, JSON output golden.

### 3. `internal/config`
- `Scope` (`Global|Project`), `Paths` (honors `UPC_HOME`), `Config` struct (`Language, PackageManager, ProjectType, Environments map[string]string, Workspace []string`).
- `Load(fs, paths) (Resolved, error)`: merge global then project; record source per key for `config show`-style debugging.
- `Save(scope, Config)`: atomic write, dir created lazily.
- `Validate(Config, core.Registries) error`: language exists; PM exists and belongs to that language; project type exists; **cross-language PM change refused** via `core.ErrIncompatible`.
- Root discovery: walk up for `.universal-package-core/`, fall back to PM `Detect` marker files via the registry.
- **Tests:** merge precedence, unknown field rejected, lazy dir creation, validation matrix, root discovery.

### 4. `internal/cli`
- `NewRoot(deps)`: persistent flags `--dry-run --json --yes --verbose --project --global`; `EnableTraverseRunHooks = true`; `SilenceUsage`/`SilenceErrors` set, errors printed once by `Execute` and mapped to exit codes.
- `deps` struct (injected): registries, runner, prompter factory, fs, logger factory. No globals.
- Commands this phase: `version` (injected build info via ldflags), `completion` (cobra built-in), `config get|set|validate`.
- Stub commands for later phases are NOT added (no dead code); each phase registers its own.
- Command constructors return `*cobra.Command`, one file per command, `commands/` subpackage per group once >5 files.
- **Tests:** flags parse into deps; exit codes; `--json`; `config set` rejects cross-language; persistent hooks run for subcommands (regression for the cobra hook gotcha).

### 5. `internal/register`
- `Default() core.Registries`: empty in this phase except a test-only fake set used by contract tests (fakes live in `internal/core/coretest`, not in register).

### 6. Contract and architecture tests
- `core/contract`: `RunPackageManagerSuite(t, pm)` (capability parity, id stable, tool info present), `RunLanguageSuite`, `RunProjectTypeSuite`. Phase 2+ calls these for every module via `register.Default()`.
- `core/arch_test.go`: implements ADR-0002 checks with `go list -deps -json ./...`: core imports no concrete package; no cross-language imports; forbidden-word scan (case-insensitive language/PM names) outside own tree, `register`, `docs`, tests of that module. The word list is read from registered module ids, so it grows automatically.

## Pilot

Build `version` end to end first (main -> register -> cli -> output -> exit code -> test -> testscript in `test/`). Confirm the wiring pattern, then add `config` commands with the same pattern.

## Documentation & Testing
- Update `ARCHITECTURE.md` (actual signatures replace descriptions), `CHANGELOG.md`, `docs/README.md` (dev setup: `make check`).
- ADR-0006 stub is NOT created now (opened in Phase 5). Add ADR for any deviation (e.g. term/prompt library choice).
- Add `test/version.txtar` testscript running the real binary.

## Integration validation
`make check` green on macOS; `go build ./cmd/upc && ./bin/upc version --json && ./bin/upc config validate`; shell completion generates for zsh and bash; CI green.

## Exit criteria
- Binary builds; `version`, `completion`, `config` work; non-interactive and `--json` behave.
- Arch and contract suites run (against fakes) and pass.
- Docs match code. Post-change audit reported.
