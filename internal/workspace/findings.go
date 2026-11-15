package workspace

import (
	"fmt"
	"sort"
)

// FindingKind is the type of a cross-app doctor finding. Each check has its own
// kind so callers can tell them apart (ADR-0014 decision 8).
type FindingKind string

// The finding kinds, one per check.
const (
	PortCollision           FindingKind = "port-collision"
	ConsumerWithoutProvider FindingKind = "consumer-without-provider"
	UnresolvedVariable      FindingKind = "unresolved-variable"
	ContractLockMissing     FindingKind = "contract-missing"
	ContractLockChanged     FindingKind = "contract-changed"
)

// Finding is one cross-app problem.
type Finding struct {
	Kind    FindingKind
	Message string
}

// Findings runs the cross-app checks in a fixed order: port collisions,
// consumers without a provider, unresolved variables, then contract locks.
func (w *Workspace) Findings() []Finding {
	var out []Finding
	out = append(out, w.portCollisions()...)
	out = append(out, w.consumersWithoutProvider()...)
	out = append(out, w.unresolvedVariables()...)
	out = append(out, w.contractFindings()...)
	return out
}

func (w *Workspace) portCollisions() []Finding {
	byPort := map[int][]string{}
	for _, n := range w.Names() {
		if p := w.apps[n].Port; p > 0 {
			byPort[p] = append(byPort[p], n)
		}
	}
	ports := make([]int, 0, len(byPort))
	for p, names := range byPort {
		if len(names) > 1 {
			ports = append(ports, p)
		}
	}
	sort.Ints(ports)
	var out []Finding
	for _, p := range ports {
		out = append(out, Finding{PortCollision, fmt.Sprintf("apps %v all use port %d", byPort[p], p)})
	}
	return out
}

func (w *Workspace) consumersWithoutProvider() []Finding {
	var out []Finding
	for _, n := range w.Names() {
		for _, target := range w.apps[n].Consumes {
			if t, ok := w.apps[target]; ok && !t.Provider() {
				out = append(out, Finding{ConsumerWithoutProvider,
					fmt.Sprintf("app %q consumes %q, which provides nothing", n, target)})
			}
		}
	}
	return out
}

func (w *Workspace) unresolvedVariables() []Finding {
	var out []Finding
	r := w.Resolver()
	for _, n := range w.Names() {
		a := w.apps[n]
		if _, err := r.Resolve(a.URL); err != nil {
			out = append(out, Finding{UnresolvedVariable, fmt.Sprintf("app %q url: %v", n, err)})
		}
		if _, err := r.Child(a.Env); err != nil {
			out = append(out, Finding{UnresolvedVariable, fmt.Sprintf("app %q %v", n, err)})
		}
	}
	return out
}
