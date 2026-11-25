package core

// NativeDryRunner is implemented by a package manager whose own command can
// check an operation without changing anything (for example a publish that
// validates the package and sends nothing). The manager returns the extra
// arguments that turn the capability into that check, and false when it has
// none. Managers that do not implement it only have upc's plan-only --dry-run.
type NativeDryRunner interface {
	NativeDryRunArgs(c Capability) ([]string, bool)
}
