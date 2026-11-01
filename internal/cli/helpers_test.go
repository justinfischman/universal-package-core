package cli

import (
	"io/fs"

	"github.com/justinfischman/universal-package-core/internal/components/fsx"
)

// noRootFS hides every config directory so project discovery finds nothing.
type noRootFS struct{ fsx.FS }

func (noRootFS) Stat(string) (fs.FileInfo, error) { return nil, fs.ErrNotExist }

func (n noRootFS) DirFS(string) fs.FS { return emptyFS{} }

type emptyFS struct{}

func (emptyFS) Open(string) (fs.File, error) { return nil, fs.ErrNotExist }
