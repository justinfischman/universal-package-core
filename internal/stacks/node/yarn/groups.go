package yarn

import (
	"bytes"
	"io/fs"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/node/shared"
)

// DiagnoseGroups checks declared dependency groups against the project's
// files (ADR-0015). A yarn.lock without the berry `__metadata` block is yarn
// classic, whose install is always full (option A); the limit is reported.
func (y *Yarn) DiagnoseGroups(fsys fs.FS, groups []string) []core.Finding {
	return shared.GroupFindings(fsys, groups, isClassicLock(fsys))
}

// isClassicLock reports whether yarn.lock exists and is in the 1.x format.
func isClassicLock(fsys fs.FS) bool {
	data, err := fs.ReadFile(fsys, lockFile)
	return err == nil && !bytes.Contains(data, []byte("__metadata:"))
}
