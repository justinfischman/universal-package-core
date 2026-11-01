package core

import "fmt"

// Capability is an operation a package manager can perform.
type Capability string

// The capability set. Adding one here breaks the contract test until every
// package manager declares it (ADR-0003).
const (
	CapInit     Capability = "init"
	CapInstall  Capability = "install"
	CapAdd      Capability = "add"
	CapRemove   Capability = "remove"
	CapList     Capability = "list"
	CapUpdate   Capability = "update"
	CapOutdated Capability = "outdated"
	CapAudit    Capability = "audit"
	CapRun      Capability = "run"
	CapBuild    Capability = "build"
	CapTest     Capability = "test"
	CapLint     Capability = "lint"
	CapUp       Capability = "up"
	CapDown     Capability = "down"
	CapPublish  Capability = "publish"
)

// AllCapabilities is the authoritative, ordered capability list.
var AllCapabilities = []Capability{
	CapInit, CapInstall, CapAdd, CapRemove, CapList, CapUpdate, CapOutdated,
	CapAudit, CapRun, CapBuild, CapTest, CapLint, CapUp, CapDown, CapPublish,
}

// Level says how a capability is provided.
type Level string

// The support levels.
const (
	// LevelNative is routed straight to the package manager.
	LevelNative Level = "native"
	// LevelFilled is implemented by upc because the manager lacks it.
	LevelFilled Level = "filled"
	// LevelUnsupported fails loudly with a reason.
	LevelUnsupported Level = "unsupported"
)

// Support is a package manager's declaration for one capability.
type Support struct {
	Level  Level
	Reason string // required for LevelUnsupported
}

// Native declares a capability routed to the manager.
func Native() Support { return Support{Level: LevelNative} }

// Filled declares a capability implemented by upc.
func Filled() Support { return Support{Level: LevelFilled} }

// Unsupported declares a capability the manager cannot provide.
func Unsupported(reason string) Support {
	return Support{Level: LevelUnsupported, Reason: reason}
}

// Validate reports a malformed declaration.
func (s Support) Validate() error {
	switch s.Level {
	case LevelNative, LevelFilled:
		return nil
	case LevelUnsupported:
		if s.Reason == "" {
			return fmt.Errorf("%w: unsupported declaration needs a reason", ErrInvalidRequest)
		}
		return nil
	default:
		return fmt.Errorf("%w: unknown support level %q", ErrInvalidRequest, s.Level)
	}
}

// CapabilitySet maps every capability to its declaration.
type CapabilitySet map[Capability]Support

// Missing returns the capabilities in AllCapabilities that are not declared.
func (s CapabilitySet) Missing() []Capability {
	var missing []Capability
	for _, c := range AllCapabilities {
		if _, ok := s[c]; !ok {
			missing = append(missing, c)
		}
	}
	return missing
}

// Valid reports whether c is a known capability.
func (c Capability) Valid() bool {
	for _, known := range AllCapabilities {
		if c == known {
			return true
		}
	}
	return false
}
