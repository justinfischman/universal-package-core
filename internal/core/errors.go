package core

import (
	"errors"
	"fmt"
)

// Sentinel errors. Match with errors.Is.
var (
	// ErrUnsupported marks a capability a package manager cannot provide.
	ErrUnsupported = errors.New("unsupported capability")
	// ErrInvalidRequest marks bad input from the caller (usage error).
	ErrInvalidRequest = errors.New("invalid request")
	// ErrIncompatible marks a refused combination, e.g. a cross-language PM switch.
	ErrIncompatible = errors.New("incompatible")
)

// Exit codes (ARCHITECTURE section 6).
const (
	ExitOK          = 0
	ExitFailure     = 1
	ExitUsage       = 2
	ExitUnsupported = 3
	ExitToolMissing = 4
)

// UnsupportedError names the package manager and capability that failed.
type UnsupportedError struct {
	PackageManager string
	Capability     Capability
	Reason         string
}

func (e *UnsupportedError) Error() string {
	msg := fmt.Sprintf("%s does not support %q", e.PackageManager, e.Capability)
	if e.Reason != "" {
		msg += ": " + e.Reason
	}
	return msg
}

// Is makes errors.Is(err, ErrUnsupported) true.
func (e *UnsupportedError) Is(target error) bool { return target == ErrUnsupported }

// ToolMissingError means a required binary is not installed.
type ToolMissingError struct {
	Tool        string
	InstallHint string
	// InstallCmd installs the tool (argv); empty when it cannot be automated.
	InstallCmd []string
}

func (e *ToolMissingError) Error() string {
	msg := fmt.Sprintf("required tool %q not found", e.Tool)
	if e.InstallHint != "" {
		msg += " (" + e.InstallHint + ")"
	}
	return msg
}

// ExitCode maps an error to a process exit code.
func ExitCode(err error) int {
	var missing *ToolMissingError
	switch {
	case err == nil:
		return ExitOK
	case errors.Is(err, ErrUnsupported):
		return ExitUnsupported
	case errors.As(err, &missing):
		return ExitToolMissing
	case errors.Is(err, ErrInvalidRequest), errors.Is(err, ErrIncompatible):
		return ExitUsage
	default:
		return ExitFailure
	}
}
