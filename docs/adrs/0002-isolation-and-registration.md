# ADR-0002: Isolation rule and explicit registration

Status: Accepted (pre-development)

## Context
The spec requires: language code in one package per language; project-type code in one package per type; core never imports a concrete implementation; adding a language touches one package plus registration.

## Decision
- `internal/core` defines interfaces, base structs and generic registries.
- Each concrete module lives in its own package and exposes `func New() core.X`.
- `internal/register` is the composition root and the only importer of concrete packages. `cmd/upc/main.go` calls `register.Default()` and passes the registry into `internal/cli`. No `init()`-based self-registration (explicit is testable and order-stable).
- An architecture test (`internal/core/arch_test.go`) uses `go list -deps -json` / `go/parser` to assert: core imports no concrete package; no concrete package imports a sibling language or project type; language and PM names appear only in their own package tree, `register`, and `docs/`; and marker files declared by a language's package managers (for example lock files) are named only under that language's tree (`internal/stacks/<language>/`). See ADR-0009.

## Alternatives rejected
- `init()` + blank imports: hidden coupling, global state, harder to test.
- Plugins (`plugin` pkg / external binaries): out of scope; Homebrew-only single binary.

## Consequences
One extra line in `register` per new module. Isolation is verified by CI, not by convention.
