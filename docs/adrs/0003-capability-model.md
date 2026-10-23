# ADR-0003: Capability set and parity

Status: Accepted (pre-development)

## Context
PMs differ (pip has no `run`/`build`; uv has `audit`, `build`, `publish`). The spec requires a core capability set where each PM is native, filled by `upc`, or fails loudly.

## Decision
- `core.Capability` is a string enum; `core.AllCapabilities` is the authoritative list.
- `core.BasePackageManager` (embedded) holds a table `Capability -> {Support, Handler}`. Constructors call `Native(cap, h)`, `Filled(cap, h)` or `Unsupported(cap, reason)`.
- A contract test requires every registered PM to have an entry for every capability. A missing entry fails the test; `Unsupported` returns `core.ErrUnsupported` (exit code 3) naming PM and capability.
- Handlers share one `core.Request`/`core.Result` shape rather than one type per capability. Trade-off: less compile-time safety per capability, but a uniform registry and one place to add fields. Handlers validate the fields they need and return `core.ErrInvalidRequest`.
- "Filled" logic that is shared across PMs (e.g. `run` for a script table) lives in `internal/components` or a core helper, never copied per PM.
- `doctor` reports the matrix of Native/Filled/Unsupported per PM.

## Consequences
New capability: add to `AllCapabilities`, tests go red until every PM declares it. Parity is machine-checked.
