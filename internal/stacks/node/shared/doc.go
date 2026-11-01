// Package shared holds the helpers that only Node package managers share:
// command invocation, name validation and package-list rendering. It is
// imported only by Node package managers (ADR-0002); language-neutral fills
// live in internal/components.
package shared
