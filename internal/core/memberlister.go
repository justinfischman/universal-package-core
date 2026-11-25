package core

import "context"

// MemberLister is implemented by a package manager that can list the workspace
// members it recognizes natively (ADR-0014 decision 3). Paths are relative to
// dir and slash-separated. `workspace check` compares the list with the
// config's `members`. Managers that do not implement it are not compared.
type MemberLister interface {
	NativeMembers(ctx context.Context, dir string) ([]string, error)
}
