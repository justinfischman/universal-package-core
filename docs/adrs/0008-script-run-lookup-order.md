# ADR-0008: `run` resolves its target in a fixed order

Status: Accepted (Phase 2)

## Context
`upc run <name>` must serve three kinds of target: a file (`hello.py`), a project script defined by the user for `upc` (`[tool.upc.scripts]`), and an installed entry point (`[project.scripts]`). The same name can exist in more than one place, so the result must be predictable.

## Decision
The uv package manager resolves `<name>` in this order and stops at the first match:
1. A file in the project directory (a regular file, no `..` in the name): `uv run <file> [args]`.
2. A `[tool.upc.scripts]` entry: a command line split into words (single and double quotes group words, no shell expansion) and run in the project environment: `uv run -- <words> [args]`.
3. A `[project.scripts]` entry: `uv run -- <name> [args]`.

If none match, the request fails with an invalid-request error that lists the available script names. Arguments after the name, including flags, pass through unchanged. `upc` only reads `pyproject.toml`; it never writes it.

## Consequences
A file shadows a same-named script, which matches what `uv run <name>` does and what a user typing a file name expects. `[tool.upc.scripts]` wins over `[project.scripts]` so a project can override an entry point for development. Other package managers define their own lookup in their own module; this order is the Python/uv one.
