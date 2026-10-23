# ADR-0005: Templates embedded per module

Status: Accepted (pre-development); details may be refined in Phase 2

## Context
"Templates are in their own languages"; location and format are decided during the build.

## Decision
- Templates are embedded with `//go:embed`. The language package owns `templates/<projecttype>/...` for language-specific files (so the project type package never mentions a language). The project type package owns only language-neutral files (e.g. a README stub). The scaffold step asks the language module for its template FS for that project type id.
- Rendering uses `text/template` through `components/render`, with a fixed variable set (`Name`, `Module`, `Language`, `PM`, `Year`).
- Where a PM can scaffold natively (e.g. `uv init --script`), prefer routing to it over our own template; templates fill only what the PM does not create.
- Scaffolds are staged in a temp dir and moved into place so a failed `new` leaves nothing behind.

## Consequences
Isolation rule holds: language-specific templates live inside the language package. Native scaffolding is used first.
