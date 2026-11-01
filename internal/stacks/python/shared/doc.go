// Package shared holds the fills and helpers that only Python package managers
// share: command invocation, virtual environment lookup, pytest and ruff
// planning, and pip-style JSON parsing. It is imported only by Python package
// managers (ADR-0002); language-neutral fills live in internal/components.
package shared
