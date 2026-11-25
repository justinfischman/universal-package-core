# ADR-0017: Windows support

Status: Accepted (implemented in Plan 07 phase 7.4 B3; unverified until the first Windows CI run)

## Context
v0.1.0 deferred Windows: the CI job and release build were disabled and about 30 tests failed (fake-fs paths, mode expectations, `upc.exe` lookup, temp-dir locks, golden paths). Plan 07 decision D7.2 puts Windows in scope if the changes stay small and the only new dependency is gopsutil. Plan 07 triage (7.1a) found that gopsutil fixes none of the test failures, so those were fixed with small standard-library changes.

## Decision
1. **Process handling uses gopsutil** (`github.com/shirou/gopsutil/v4`) for liveness, start time and whole-tree terminate then kill, replacing `ps`, process-group signals and `taskkill`. Only `detach` stays per OS.
2. **CI matrix:** the `check` job runs on ubuntu, macos and windows (`.github/workflows/ci.yml`).
3. **Release archives:** goreleaser builds `windows` for amd64 and arm64 and packages Windows archives as `.zip` (other systems stay `tar.gz`).
4. **Scoop and winget manifests are deferred.** They need a publishing repository and a verified Windows release first.

## Consequences
- Windows is claimed as supported but unverified until the first CI run on `windows-latest`; the matrix job is the check that confirms or refutes it.
- One extra dependency (gopsutil) in exchange for a single process-control path on all systems.
- Windows users install from the release `.zip`; there is no package-manager route yet.
- ADR-0016 follow-up e (Windows process-tree kill) is now served by gopsutil rather than `taskkill`.
