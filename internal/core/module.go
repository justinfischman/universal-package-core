// Package core holds the interfaces, base types, registry and errors shared by
// every language, package manager and project type. It imports nothing concrete.
package core

// Kind classifies a Module.
type Kind string

// The module kinds.
const (
	KindLanguage       Kind = "language"
	KindPackageManager Kind = "pm"
	KindProjectType    Kind = "projecttype"
)

// Module is the base identity every registrable thing has.
type Module interface {
	// ID is the stable, unique (within its Kind) identifier, e.g. "alpha".
	ID() string
	Kind() Kind
}
