package core

import (
	"context"
	"fmt"
	"io/fs"
)

// Tool describes the external binary a package manager wraps.
type Tool struct {
	Binary      string
	VersionArgs []string // e.g. ["--version"]
	InstallHint string
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
	return d.handler(ctx, req)
}
