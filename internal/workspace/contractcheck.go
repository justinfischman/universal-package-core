package workspace

import "fmt"

// AppContract is the lock comparison of one app that declares a contract. Err
// is set when the comparison itself failed (unreadable or invalid contract or
// lock); Status is meaningful only when Err is nil.
type AppContract struct {
	App    string
	Status ContractStatus
	Err    error
}

// Failing reports whether the app needs attention: its lock is missing or
// stale, or the comparison failed.
func (a AppContract) Failing() bool { return a.Err != nil || a.Status.State != ContractOK }

// CheckAll compares every app that declares a contract with its lock, in app
// name order. It never writes a lock.
func (c *ContractLocks) CheckAll() []AppContract {
	var out []AppContract
	for _, n := range c.workspace.Names() {
		if a, _ := c.workspace.App(n); a.Contract == "" {
			continue
		}
		st, err := c.Status(n)
		out = append(out, AppContract{App: n, Status: st, Err: err})
	}
	return out
}

// contractFindings reports each app whose contract lock is missing or stale.
// A contract that cannot be compared at all is reported as contract-changed:
// the committed lock can no longer vouch for it.
func (w *Workspace) contractFindings() []Finding {
	var out []Finding
	for _, r := range NewContractLocks(w).CheckAll() {
		switch {
		case r.Err != nil:
			out = append(out, Finding{ContractLockChanged, fmt.Sprintf("app %q contract cannot be compared with its lock: %v", r.App, r.Err)})
		case r.Status.State == ContractMissing:
			out = append(out, Finding{ContractLockMissing, fmt.Sprintf("app %q has no contract lock; run `upc workspace check --update` and commit it", r.App)})
		case r.Status.State == ContractChanged:
			out = append(out, Finding{ContractLockChanged, fmt.Sprintf("app %q contract differs from its lock: %s", r.App, r.Status.Summary)})
		}
	}
	return out
}
