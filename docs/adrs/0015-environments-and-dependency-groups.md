# ADR-0015: Environments and dependency groups

Status: Accepted 2026-11-15 (Phase 6.2 landed; npm/yarn arbitrary groups and `doctor` group checks deferred)

## Context
ADR-0006 (accepted, not edited) defines an environment as a name plus an env file. Users also want dependency groups (dev, test, docs) selected per environment. Managers differ: uv and poetry have native groups, pip has none, npm and yarn have none beyond dev/optional.

## Decision
1. **Environment = name + optional env file + dependency groups** (`groups []string`). This extends ADR-0006 and narrows its env-file-only scope; the ADR-0006 text stays as written.
2. **No venv per environment.** One package-manager environment per project; an environment only selects an env file path and groups.
3. **Commands:** `env add|remove|list|use|show`, `install --env <name>`, `add <dep> --group <g>`, `--env` on `run`, `test`, `up`.
4. **Group mapping per manager:**

| Manager | Groups |
|---|---|
| uv | Native: `[dependency-groups]`, `--group`, `--dev`, `--optional`. |
| poetry | Native: `poetry add --group`, `poetry install --with/--only`. |
| pip | Per-group `requirements-<group>.txt` (default group `requirements.txt`) via `-r`. |
| npm | Option B: a group is a workspace member `.universal-package-core/groups/<name>/package.json`. Subset install `npm install -w <member>` / `npm ci -w <member>`; `add <pkg> --group x` is `npm i <pkg> -w <member>`. The workspace glob is the literal prefix `.universal-package-core/groups/*`, never `**`. Pitfalls: member names must be unique; a later plain `npm install` expands to everything; the subset is not remembered, so `-w` is passed every time. |
| yarn berry (4.x) | Option B: `yarn workspaces focus <member>` (not `yarn workspace <m> install`). The lockfile stays full; install state is partial and a later `yarn install` re-expands; 2.x/3.x need the workspace-tools plugin; PnP differs from node-modules. |
| yarn classic (1.x) | Option A: no subset install exists. Membership is upc metadata; `add --group x` is `yarn workspace <member> add`; install is always full. The limit is surfaced in `doctor` and output. The capability is Filled, not Unsupported. |

5. **Verification.** The npm and yarn doc claims were tested locally (npm 11.19.1, yarn 4.18.1, yarn 1.22.22) and are confirmed against documentation in the 6.2 pilot.

## Consequences
- The design is decided per manager (npm and berry B, classic A).
- `-w/--member` is plumbed once in the shared capability-command factory.
- The `Environment` type in `internal/config` gains `groups`.

## Related
- ADR-0006 (environment model): extended by this ADR, text left as written.
- ADR-0011 (config layering): environments merge by key across layers.
- ADR-0014 (workspaces): `-w/--member` and the member mechanism behind npm and yarn groups.
- ADR-0016 (run lookup and `up`/`down`): `--env` applies to `run`, `test` and `up`.
