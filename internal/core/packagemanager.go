package core

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
)

// Tool describes the external binary a package manager wraps.
type Tool struct {
	Binary      string
	VersionArgs []string // e.g. ["--version"]
	InstallHint string
	// InstallCmd is the command that installs the binary on this platform
	// (argv, first element is the program). Empty when upc cannot install it.
	InstallCmd []string
}

// Handler implements one capability.
type Handler func(ctx context.Context, req Request) (Result, error)

// PackageManager is a native manager belonging to exactly one language.
type PackageManager interface {
	Module
	// Language is the id of the owning language.
	Language() string
	Tool() Tool
	Capabilities() CapabilitySet
	// Detect reports whether the directory uses this manager (marker files).
	Detect(fsys fs.FS) bool
	// Execute runs a capability, failing with *UnsupportedError when it is not provided.
	Execute(ctx context.Context, c Capability, req Request) (Result, error)
}

type declaration struct {
	support Support
	handler Handler
}

// BasePackageManager implements the capability table and dispatch. Concrete
// managers embed it and call Declare for every capability in AllCapabilities.
type BasePackageManager struct {
	id       string
	language string
	tool     Tool
	markers  []string
	table    map[Capability]declaration
}

// NewBasePackageManager builds the shared base. markers are file names whose
// presence in a directory means this manager is in use.
func NewBasePackageManager(id, language string, tool Tool, markers ...string) BasePackageManager {
	return BasePackageManager{
		id: id, language: language, tool: tool, markers: markers,
		table: make(map[Capability]declaration),
	}
}

// ID implements Module.
func (b *BasePackageManager) ID() string { return b.id }

// Kind implements Module.
func (b *BasePackageManager) Kind() Kind { return KindPackageManager }

// Language implements PackageManager.
func (b *BasePackageManager) Language() string { return b.language }

// Tool implements PackageManager.
func (b *BasePackageManager) Tool() Tool { return b.tool }

// Declare records how a capability is provided. Native and Filled require a
// handler; Unsupported must not have one.
func (b *BasePackageManager) Declare(c Capability, s Support, h Handler) error {
	if !c.Valid() {
		return fmt.Errorf("%w: unknown capability %q", ErrInvalidRequest, c)
	}
	if err := s.Validate(); err != nil {
		return err
	}
	if s.Level == LevelUnsupported && h != nil {
		return fmt.Errorf("%w: %s/%s is unsupported but has a handler", ErrInvalidRequest, b.id, c)
	}
	if s.Level != LevelUnsupported && h == nil {
		return fmt.Errorf("%w: %s/%s is %s but has no handler", ErrInvalidRequest, b.id, c, s.Level)
	}
	b.table[c] = declaration{support: s, handler: h}
	return nil
}

// Capabilities implements PackageManager.
func (b *BasePackageManager) Capabilities() CapabilitySet {
	set := make(CapabilitySet, len(b.table))
	for c, d := range b.table {
		set[c] = d.support
	}
	return set
}

// Markers returns the file names whose presence signals this manager. The
// architecture test forbids them outside the owning language's tree.
func (b *BasePackageManager) Markers() []string { return append([]string(nil), b.markers...) }

// Detect implements PackageManager.
func (b *BasePackageManager) Detect(fsys fs.FS) bool {
	for _, m := range b.markers {
		if _, err := fs.Stat(fsys, m); err == nil {
			return true
		}
	}
	return false
}

// Execute implements PackageManager.
func (b *BasePackageManager) Execute(ctx context.Context, c Capability, req Request) (Result, error) {
	d, ok := b.table[c]
	if !ok {
		return Result{}, &UnsupportedError{PackageManager: b.id, Capability: c, Reason: "capability not declared"}
	}
	if d.support.Level == LevelUnsupported {
		return Result{}, &UnsupportedError{PackageManager: b.id, Capability: c, Reason: d.support.Reason}
	}
	res, err := d.handler(ctx, req)
	var missing *ToolMissingError
	if errors.As(err, &missing) && missing.Tool == b.tool.Binary {
		if missing.InstallHint == "" {
			missing.InstallHint = b.tool.InstallHint
		}
		if len(missing.InstallCmd) == 0 {
			missing.InstallCmd = b.tool.InstallCmd
		}
	}
	return res, err
}

// Severity ranks a diagnosis.
type Severity string

// The severities.
const (
	SeverityInfo Severity = "info"
	SeverityWarn Severity = "warn"
)

// Finding is one thing `doctor` noticed about a project.
type Finding struct {
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	// Fix is a capability that resolves the finding; empty when none does.
	Fix Capability `json:"fix,omitempty"`
}

// Diagnoser is optionally implemented by a PackageManager that can inspect a
// project for problems it knows about (for example a missing lockfile).
type Diagnoser interface {
	Diagnose(fsys fs.FS) []Finding
}

// DetectRanker is optionally implemented by a PackageManager to say how strongly
// a directory matches it, so overlapping markers resolve without asking: a
// lockfile outranks a manifest that several managers share. Only managers
// whose Detect is true are ranked; the highest rank wins and a tie is ambiguous.
type DetectRanker interface {
	DetectRank(fsys fs.FS) int
}
