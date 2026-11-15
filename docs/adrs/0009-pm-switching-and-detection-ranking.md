# ADR-0009: Package-manager switching and detection ranking

Status: Accepted (Phase 3)

## Context
Python now has three managers (uv, pip, poetry) whose files overlap: a project can hold `requirements.txt` and a `[tool.poetry]` table, or a `uv.lock` beside a `poetry.lock`. `upc` must pick one predictably, let the user change that choice, and keep file-name knowledge (such as lock file names) inside the language that owns it (ADR-0002). The native manager owns manifests and lockfiles (ADR-0004), so `upc` must not rewrite them to suit another manager.

## Decision
1. **Switching stays inside a language.** `upc config set pm <id>` (alias of `package_manager`) and the `package_managers` list accept a manager only if it belongs to the project's current language: the configured language, else the language of the configured manager, else the language the project's files show. Another language's manager fails with `core.ErrIncompatible`. A switch does not convert manifests or lockfiles; it prints a note that the new manager's own commands own them and points to `upc doctor`.
2. **Detection is ranked.** A manager may implement `core.DetectRanker`. Ranks: uv `uv.lock` 300, `[tool.uv]` 200, inline-metadata script 150; poetry `poetry.lock` 300, `[tool.poetry]` 200; pip `requirements.txt` 100. Auto-detect keeps the highest rank and logs the decision. If several managers share the highest rank, an interactive session asks; a non-interactive session or `--yes` fails listing the candidates. A configured manager always wins over detection. `doctor` reports ties and files that point at a manager other than the configured one.
3. **Marker files are confined to the language tree.** Managers pass their marker file names to `NewBasePackageManager`. `core.Registries.MarkerFiles()` maps each name to the owning language, and `arch.CheckMarkers` (the architecture test) fails if a scanned file outside `internal/stacks/<language>/` names one. Docs, `register`, `internal/arch` and testdata are exempt, as for the module-id scan.

## Consequences
- Moving between uv, pip and poetry is a config change plus the user's own migration (for example `poetry install`, `uv sync`); `upc` never edits the other manager's files. The cost is that a half-migrated project can look ambiguous, which `doctor` and the tie rule surface instead of guessing.
- Ranking lives in each manager, so a new manager adds its ranks without touching core or the CLI. Ranks only need to be comparable within one detection run.
- Core and the CLI stay free of file names like `uv.lock`; the rule is enforced by test rather than by review.
- **Exit code.** `plans/03-python-stacks-and-package-type.md` says a cross-language switch exits 1. The implementation exits 2: `core.ExitCode` maps `ErrInvalidRequest` and `ErrIncompatible` to the usage code (ARCHITECTURE section 6). This ADR records 2 as the decision; the plan is read-only during execution and is not edited.
- **Known gaps.** `list --pm` does not exist (`publish` is a CLI command since Phase 6.3, ADR-0016). A new pip `package` has no `.venv` until `upc install`; a new poetry `package` needs `upc install` before `upc test`.
