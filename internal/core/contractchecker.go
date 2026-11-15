package core

// ContractChecker normalizes one kind of contract file (OpenAPI, JSON Schema)
// so that two versions can be compared and a committed lock can detect drift
// (ADR-0014). Implementations are stateless.
type ContractChecker interface {
	// Name is the stable identifier stored in lock files, e.g. "openapi".
	Name() string
	// Snapshot reads the contract at path and returns its normalized form:
	// equal contracts yield byte-equal snapshots.
	Snapshot(path string) ([]byte, error)
	// Diff summarizes what changed between two snapshots as human-readable
	// lines. It returns "" when they are equivalent.
	Diff(old, new []byte) string
}
