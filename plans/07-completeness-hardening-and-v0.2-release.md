# Phase 7: Completeness, hardening, and v0.2 release

> **Edit rule (user override of the usual read-only-plan rule):** during execution the ONLY section that may be edited is `## Progress ledger` at the end. Every other section is read-only. Record locked pilot steps in the ledger's Evidence column. **Ledger rule (owner requirement):** every phase and every step ends with a ledger update (status, agent, evidence or commit); a step is not done until its row is updated, and the coordinator checks this before starting the next phase.

**Depends on:** Phase 6 complete (v0.1.0 released 2026-11-18, `docs/CHANGELOG.md:5-6`, `docs/RELEASING.md:34`).
**Goal:** close every deferred and missing item (release hygiene, deferred config/workflow features, Windows, pnpm/bun, signing, stale docs) and ship v0.2.0, executed fast by subagents that each receive only their own brief.

## Decisions (owner answers recorded in 7.0; recommendation shown)

| # | Decision | ADR |
|---|---|---|
| D7.1 | License: **MIT** (owner confirmed) | none (LICENSE file) |
| D7.2 | Windows in scope **if simple** (owner confirmed); use `gopsutil` as the agreed third-party library. "Simple" = small changes, no further new dependency. The 7.1 triage must check whether `gopsutil` addresses the ~30 failures; the repo records no earlier decision on it | 0017 |
| D7.3 | Signing: **deferred until the repo is public** (owner decision). No cosign config, no ADR 0018 in v0.2.0; revisit on going public | none |
| D7.4 | Other languages: design ADR + spike only (default, owner did not object) | 0019 |
| D7.5 | Release version: v0.2.0 (default, owner did not object) | none |

Answers are written into the Progress ledger row 7.0 and copied into this table by the coordinator only if the owner asks; otherwise the ledger is authoritative.

## Verified starting state (evidence)

- upc is a Go/Cobra CLI, v0.1.0 released 2026-11-18 (`docs/CHANGELOG.md:5-6`, `docs/RELEASING.md:34`); 20 commands, 110 test files, build and tests pass (coordinator re-verifies in 7.1).
- Homebrew formula `Formula/upc.rb` in the repo tap (commit `78b4f1b`). `.goreleaser.yaml:24-25` builds darwin and linux only.
- CI: `.github/workflows/ci.yml`, `.github/workflows/release.yml`. Windows disabled in CI (about `ci.yml:28-39`), about 30 failing tests (`docs/CHANGELOG.md:24`, commit `357c146`).
- Missing: `LICENSE` (repo is public), root `README.md` (only `docs/README.md`), `SECURITY.md`, `CONTRIBUTING.md`, signing (only `checksums.txt`), upgrade/uninstall docs, `go install` undocumented and unverified.
- Stale: `docs/README.md:5` and `:141` say tag/brew pending; plan 06 Windows exit criteria contradict its decision; `plans/README.md:18` records a Plan 03 vs code exit-code mismatch (plan says exit 1, code exits 2).
- Deferred from 6.2: npm/yarn groups, doctor group checks (`docs/adrs/0015` line 3), root-level `local.toml`, `config show --origin`.
- Deferred from 6.7: native `publish --dry-run`, offer to create an up script, workspace-member comparison with the manager's native list (ADR 0014 decision 3), `--app X` with dependencies, `${...}` escape syntax.
- Future (`docs/CHANGELOG.md:25`, `:57`): pnpm, bun, other languages, justfile export, app sub-types (CLI, library).

## Cross-phase rules

- No `_helpers`/`_utils` packages or files; files near 300 lines; split by responsibility.
- Never relax types or contracts, never skip or weaken tests; fix the code. No `Any`, no suppressions.
- No symlinks in git; no secrets or tokens in the repo (signing uses OIDC, no stored keys).
- Interfaces in `core`; concrete code in its own package; wiring in `register`. One process runner: `internal/components/execx`.
- Plans other than this one stay read-only (record corrections in `docs/CHANGELOG.md` or an ADR).
- Post-change audit after every task: files changed, `git diff` check, docs/tests stale list, tests run.

## Dependency and parallelism map

```
7.0 owner gate (serial)
  |-- 7.1 recon a,b,c (parallel, read-only) ---+
  |-- 7.3 Wave A docs/hygiene (parallel, no Go)|
  +-- 7.2 pilot: config show --origin <--------+ (needs 7.1b,c)
        |
        +-- 7.4 Wave B: B1 B2 B5 start; B4 after 7.1b; B3 after 7.1a + D7.2; B6 after B3
              merge order: B1, B2, B4, B3, B6
                |
                7.5 docs+test sweep -> 7.6 integration validation -> 7.7 release (owner confirms)
```

| Stream | Owns (disjoint) | Blocked by |
|---|---|---|
| B1 | `internal/config`, doctor group checks | 7.2 |
| B2 | workflow commands (`up`, `publish`, `app`, `workspace`) | 7.2 |
| B3 | Windows fixes, `ci.yml` matrix, `.goreleaser.yaml` windows target, scoop/winget manifests | 7.1a, D7.2 |
| B4 | new packages `internal/stacks/node/pnpm`, `.../bun`, app sub-types, justfile export | 7.1b |
| B5 | `docs/adrs/0019*`, `spikes/` or `docs/spikes/` | D7.4 |
| B6 | SBOM, install script, checksum docs, `release.yml` (cosign deferred, D7.3) | B3 merged |

## Phase 7.0: Owner decisions gate (serial)

**Goal:** the coordinator asks the owner one question covering D7.1 to D7.5 with the recommended option marked, then records answers. Answered in the planning session (see Decisions table); only confirm the ledger row.
**Steps:** 1) one `AskUserQuestion`-style message with five choices and recommendations; 2) write the answers into ledger row 7.0; 3) mark B3 and B6 `skipped` in the ledger if D7.2 is no.
**Exit criteria:** five answers recorded; no code touched.
**Documentation & testing:** none needed (decision record only). **Integration validation:** coordinator confirms the ledger shows all five answers before 7.2 begins.

## Phase 7.1: Reconnaissance (parallel, read-only)

Three `cache-explorer-agent` runs; each stores findings in the cache and returns `tc_` ids plus one line. No file edits.

| Id | Question | Output |
|---|---|---|
| 7.1a | Triage the about 30 Windows test failures (`docs/CHANGELOG.md:24`, commit `357c146`); run tests with a Windows build tag or `GOOS=windows go vet` and read the failing list | failures grouped by root cause (path separators, shell, exec, file modes), `path:line` per group |
| 7.1b | Map adapter/core interfaces and `internal/register/register.go` wiring that pnpm, bun, new stacks and groups must extend | file list, interfaces to implement, wiring points, npm/yarn spec files to copy |
| 7.1c | Where exit codes are defined; confirm the 1-vs-2 cross-language-switch mismatch (`plans/README.md:18`) | `path:line` of the code and the test pinning it |

**Exit criteria:** three cache ids stored. **Documentation & testing:** none (read-only). **Integration validation:** coordinator spot-checks one `path:line` per id before use.

## Phase 7.2: Pilot, `config show --origin` end to end

**Goal:** lock the method on one representative feature before the fan-out. Single agent: `software-engineer`, then `test-engineer`, then an independent `code-reviewer`.
**Steps:**
1. Implement `config show --origin` (per-key source: flag, app local.toml, app config, root local.toml, root config, global; ADR 0011 precedence) in `internal/config` and the CLI command; mask secrets from `local.toml`.
2. Table-driven tests for each layer plus a secret-masking test; golden `--json` output.
3. Add an ADR 0011 note (status line only, no history rewrite) and a command-reference entry.
4. Review build, tests, `--help` and output; fix the method, not just the pilot; record the locked steps in ledger row 7.2.
5. Return a 5-line CHANGELOG snippet (workers never edit shared docs).
**Exit criteria:** `go build ./... && go test ./...` pass; reviewer reports no blocking findings; locked steps recorded.
**Documentation & testing:** command reference and tests above. **Integration validation:** run `upc config show --origin` in a temp project with global, root and local layers.

## Phase 7.3: Wave A, docs and release hygiene (parallel, no Go packages)

Agent: `documentation-maintainer`, one brief per row. Runs alongside 7.1 to 7.4; touches no Go source.

| Task | Path | Acceptance |
|---|---|---|
| LICENSE | `LICENSE` | text matches D7.1; copyright holder from `git config user.name` or owner |
| Root README | `README.md` | exact `brew tap` and `brew install` commands copied from `Formula/upc.rb` and `docs/RELEASING.md`; quickstart; command table matching `upc --help`; platform support matches `.goreleaser.yaml` |
| Security policy | `SECURITY.md` | reporting channel, supported versions, no secrets |
| Contributing | `CONTRIBUTING.md` | build/test via `scripts/check.sh`, branch and commit conventions, plan rule |
| Upgrade/uninstall | `docs/INSTALL.md` (or README section) | brew upgrade/uninstall, manual binary removal, config dir cleanup `~/.universal-package-core/` |
| `go install` | same doc | run `go install <module>/cmd/upc@latest` in a temp GOPATH, record the real result; document or file the failure |
| Stale docs | `docs/README.md:5`, `:141` | say v0.1.0 released and installable via brew |
| Exit-code mismatch | `docs/CHANGELOG.md` Known gaps and an ADR note | states code exits 2 (per 7.1c); `plans/README.md` untouched |

**Exit criteria:** all files exist, links resolve, every command quoted in docs was run. **Documentation & testing:** this wave is documentation; verify each documented command by execution. **Integration validation:** `scripts/check.sh` still passes; coordinator runs a link check.

## Phase 7.4: Wave B, code workstreams (each in its own git worktree and branch)

Per stream: `software-engineer` implements, `test-engineer` adds tests, an independent `code-reviewer` (fresh agent) reviews. Branch names `feature/<stream>-<topic>`. Streams own disjoint packages.

| Stream | Scope | Acceptance |
|---|---|---|
| B1 config/doctor | npm/yarn dependency groups; doctor group checks (ADR 0015); root-level `local.toml` in the layering walk | golden tests per manager; doctor reports group drift; precedence tests |
| B2 workflow | native `publish --dry-run`; offer to create an up script when none exists; `--app X` with dependencies; `${...}` escape syntax; ADR 0014 d3 workspace-member comparison with the manager's native list | each flag has golden dry-run output; `workspace check` fails on mismatch |
| B3 Windows (if D7.2) | fix failures by 7.1a root-cause group; re-enable the CI Windows job; add goreleaser windows target; scoop and winget manifests | CI green on macOS, Ubuntu, Windows; snapshot builds windows archives |
| B4 new stacks | pnpm and bun (copy npm/yarn spec pattern from 7.1b); app sub-types CLI and library; justfile export | contract parity test passes for each new manager; E2E testscript each |
| B5 languages | design ADR 0019 plus one spike of a non-Python/Node stack on a throwaway branch | ADR accepted; spike notes list interface gaps; no production code |
| B6 supply chain (after B3) | SBOM, curl install script, checksum verification docs (cosign deferred, D7.3) | snapshot shows SBOM; install script verifies `checksums.txt`; no key material in repo |

**Shared-file rule:** workers never edit `docs/CHANGELOG.md`, `docs/README.md` or shared `register` wiring files. Each returns a 5-line changelog snippet and the wiring diff; the coordinator batches them. Merge order B1, B2, B4, B3, B6 (B5 any time); rebase between merges; run `go build ./... && go test ./...` after each merge.
**Pilot rule:** B4 starts with pnpm alone, review, then bun. B3 starts with the largest root-cause group.
**Documentation & testing:** each stream ships its tests and ADR notes. **Integration validation:** per-merge build and tests; per-stream E2E testscript where applicable.

## Phase 7.5: Documentation and testing sweep

Agents: `documentation-maintainer` and `test-engineer` in parallel (disjoint files).

| Task | Acceptance |
|---|---|
| Command reference vs `upc --help` | every command and flag matches, including new ones |
| ADR statuses and CHANGELOG | batched snippets merged; 0017 to 0019 statuses correct |
| Coverage gaps | `internal/core/coretest` and new code reviewed; added tests for uncovered branches |
| Test files over 300 lines | re-measure and split by responsibility only if touched |

**Exit criteria:** docs match the binary; coverage gaps closed or listed. **Integration validation:** `scripts/check.sh`.

## Phase 7.6: Integration validation

Coordinator plus independent `code-reviewer` and `security-auditor` (fresh agents).

| Check | Method |
|---|---|
| Full build/test | `go build ./... && go test ./...`, `golangci-lint`, `go vet` |
| E2E | `UPC_E2E=1` against real uv, poetry, npm, yarn (plus pnpm, bun) with versions logged |
| Release dry run | `goreleaser release --snapshot --clean`; all targets built; checksums and SBOM present (no signature; D7.3 deferred) |
| Security | `security-auditor` pass over exec, config secrets, install script, workflows |
| Fresh machine | `brew install` and `brew upgrade` on macOS and Linux (and Windows via scoop/winget if D7.2) |

**Exit criteria:** zero blocking findings; E2E green. **Documentation & testing:** record results in the ledger. **Integration validation:** this phase is the validation.

## Phase 7.7: Release v0.2.0

Follow `docs/RELEASING.md`. **The owner confirms before tagging and publishing.**
**Steps:** 1) coordinator proposes tag and CHANGELOG; 2) owner confirms; 3) tag and publish; 4) verify `brew install` and `brew upgrade` from the new tag, verify signature; 5) update CHANGELOG, `docs/README.md` and the ledger.
**Exit criteria:** release assets present, brew verification passed, docs updated. **Documentation & testing:** CHANGELOG and README version references. **Integration validation:** post-release brew check.

## Agent briefing

- Every brief contains: goal, exact paths, acceptance criteria, report format. Pass paths, never pasted file content.
- Report format: 20 lines or fewer, `path:line` evidence, no file contents.
- Cache first: `agent-tool-cache` lookup before exploring; store significant results with tag and subject; cite `tc_` ids.
- Zero-thrashing: at most one retry, then stop and report the blocker with evidence.
- Continue an existing agent via `SendMessage` for dependent follow-ups; spawn fresh agents for reviews.
- Workers do not commit unless the coordinator says so; commit messages are Conventional Commits, 72 characters or fewer.
- Agents: `cache-explorer-agent` (7.1), `documentation-maintainer` (7.3, 7.5), `software-engineer` and `test-engineer` (7.2, 7.4), `code-reviewer` and `security-auditor` (7.2, 7.4, 7.6), `dependency-maintainer` (B4 if new modules are needed).

## Risks

| Risk | Mitigation |
|---|---|
| Windows scope creep | D7.2 gate; B3 groups failures by cause; cap at the 7.1a list, new findings go to a follow-up row |
| Merge conflicts across streams | disjoint package ownership, worktrees, shared files batched by the coordinator, fixed merge order |
| E2E tool-version drift (uv, poetry, npm, yarn, pnpm, bun) | log versions per run, pin versions in `ci.yml`, update `plans/README.md` conventions via CHANGELOG note |
| Signing misconfiguration blocks release | test in snapshot first; keyless OIDC needs `id-token: write` only |
| Spike turns into implementation | B5 limited to ADR plus throwaway branch (D7.4) |

## Exit criteria (phase)

- LICENSE, root README, SECURITY, CONTRIBUTING, upgrade/uninstall docs exist and are verified.
- All deferred 6.2 and 6.7 items implemented with tests, or explicitly deferred in the ledger with an owner decision.
- pnpm and bun supported (contract parity and E2E pass); languages ADR accepted.
- Windows decision implemented (CI green) or recorded as out of scope.
- Release is published with checksums and SBOM (signing deferred, D7.3); v0.2.0 installable via brew; docs match the binary.

## Progress ledger

| Task | Agent | Status | Evidence/commit |
|---|---|---|---|
| 7.0 Owner decisions (D7.1 to D7.5) | change-coordinator | done | D7.1 MIT; D7.2 Windows if simple + gopsutil; D7.3 cosign deferred until public; D7.4 ADR+spike; D7.5 v0.2.0. Commit 94aaf85 |
| 7.1a Windows failure triage | cache-explorer-agent | done (partial) | tc_a0cf24f475: 6 groups G1-G6, small stdlib fixes, gopsutil fixes none of G1-G5; unverified (no Windows run), B3 starts by re-enabling CI Windows job |
| 7.1b Adapter/register interface map | cache-explorer-agent | done | tc_b3a5543235, tc_f07ac9cbcc (register.go line numbers approximate) |
| 7.1c Exit-code map and mismatch | cache-explorer-agent | done | tc_04ad099e09: code exits 2 (errors.go:71), only plan 03:40 says 1; docs fix only |
| 7.2 Pilot `config show --origin` (locked steps here) | software-engineer, test-engineer, code-reviewer | done | f8f503b, 1d1ea9d merged; review no blockers; temp-project E2E passed. LOCKED: (1) read layer code first, add read-only logic rather than change merge path; (2) domain logic in one `internal/config` file with typed values, thin CLI file; (3) mask by layer, never print local.toml values; (4) return errors, no discards; (5) golden JSON free of absolute paths; (6) validate flags before session setup; (7) unique test helper names per package; (8) test edge cases for parity with merge path, malformed files, --global; (9) independent review + temp-project run before merge; (10) worker returns 5-line CHANGELOG snippet (pending batch, see 7.4 merge). GOTCHA: worktrees under `.claude/worktrees` are scanned by `internal/arch` TestNoModuleNamesLeakOutsideTheirTree; remove each worktree after merging (done) or `go test ./...` fails on main |
| 7.3 LICENSE | documentation-maintainer | done | 43712db |
| 7.3 Root README.md | documentation-maintainer | done | 44fb526 |
| 7.3 SECURITY.md | documentation-maintainer | done | 44fb526 |
| 7.3 CONTRIBUTING.md | documentation-maintainer | done | 44fb526 |
| 7.3 Upgrade/uninstall docs | documentation-maintainer | done | cea5903 |
| 7.3 Verify and document `go install` | documentation-maintainer | done | cea5903 (ran go install, v0.1.0) |
| 7.3 Fix stale `docs/README.md:5,:141` | documentation-maintainer | done | 136e650 (also fixed :7) |
| 7.3 Reconcile exit-code mismatch note | documentation-maintainer | done | 136e650 (CHANGELOG Known gaps) |
| 7.4 B1 config/doctor | software-engineer, test-engineer, code-reviewer | pending | |
| 7.4 B2 workflow commands | software-engineer, test-engineer, code-reviewer | pending | |
| 7.4 B3 Windows enablement | software-engineer, test-engineer, code-reviewer | pending | |
| 7.4 B4 pnpm, bun, sub-types, justfile export | software-engineer, test-engineer, code-reviewer | pending | |
| 7.4 B5 languages ADR and spike | software-engineer, code-reviewer | pending | |
| 7.4 B6 supply chain | software-engineer, code-reviewer, security-auditor | pending | |
| 7.4 Merge B1, B2, B4, B3, B6 and batch changelog | change-coordinator | pending | |
| 7.5 Docs and testing sweep | documentation-maintainer, test-engineer | pending | |
| 7.6 Integration validation | change-coordinator, code-reviewer, security-auditor | pending | |
| 7.7 Release v0.2.0 and brew verification | change-coordinator | pending | |
