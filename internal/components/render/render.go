// Package render writes embedded templates to disk. Every file is rendered in
// memory first and conflicts are checked before anything is written; if a
// write fails, files created so far are removed, so no half-scaffold remains.
package render

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// TemplateSuffix marks files that are rendered; the suffix is stripped from the
// output name. Other files are copied verbatim.
const TemplateSuffix = ".tmpl"

// Options controls Render.
type Options struct {
	// Overwrite allows replacing files that already exist.
	Overwrite bool
}

type file struct {
	dest string
	data []byte
	perm fs.FileMode
}

// Render renders every file under src into dest using data and returns the
// destination paths written, in order.
func Render(fsys fsx.FS, src fs.FS, dest string, data any, opts Options) ([]string, error) {
	files, err := collect(src, dest, data)
	if err != nil {
		return nil, err
	}
	if !opts.Overwrite {
		for _, f := range files {
			if _, err := fsys.Stat(f.dest); err == nil {
				return nil, fmt.Errorf("%w: %s already exists", core.ErrInvalidRequest, f.dest)
			}
		}
	}

	var written []string
	for _, f := range files {
		if err := fsys.WriteFileAtomic(f.dest, f.data, f.perm); err != nil {
			rollback(fsys, written)
			return nil, fmt.Errorf("write %s: %w", f.dest, err)
		}
		written = append(written, f.dest)
	}
	return written, nil
}

func rollback(fsys fsx.FS, paths []string) {
	for i := len(paths) - 1; i >= 0; i-- {
		_ = fsys.Remove(paths[i])
	}
}

func collect(src fs.FS, dest string, data any) ([]file, error) {
	var files []file
	err := fs.WalkDir(src, ".", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		raw, err := fs.ReadFile(src, p)
		if err != nil {
			return err
		}
		out := p
		if strings.HasSuffix(p, TemplateSuffix) {
			raw, err = execute(p, raw, data)
			if err != nil {
				return err
			}
			out = strings.TrimSuffix(p, TemplateSuffix)
		}
		files = append(files, file{dest: filepath.Join(dest, filepath.FromSlash(path.Clean(out))), data: raw, perm: 0o644})
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, errors.New("render: template source has no files")
	}
	return files, nil
}

func execute(name string, raw []byte, data any) ([]byte, error) {
	t, err := template.New(name).Option("missingkey=error").Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("render template %s: %w", name, err)
	}
	return buf.Bytes(), nil
}
