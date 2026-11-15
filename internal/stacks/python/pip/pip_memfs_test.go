package pip

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

const proj = "/proj"

// memFS is an in-memory fsx.FS keyed by full path. failWrite makes every
// atomic write fail, to prove files are left unchanged.
type memFS struct {
	files     map[string]string
	failWrite error
	writes    int
}

var _ fsx.FS = (*memFS)(nil)

func newMem(files map[string]string) *memFS {
	m := &memFS{files: map[string]string{}}
	for name, body := range files {
		m.files[filepath.Join(proj, name)] = body
	}
	return m
}

func (m *memFS) ReadFile(p string) ([]byte, error) {
	if b, ok := m.files[p]; ok {
		return []byte(b), nil
	}
	return nil, fs.ErrNotExist
}

func (m *memFS) WriteFileAtomic(p string, data []byte, _ fs.FileMode) error {
	if m.failWrite != nil {
		return m.failWrite
	}
	m.writes++
	m.files[p] = string(data)
	return nil
}

func (m *memFS) Stat(p string) (fs.FileInfo, error) {
	if _, ok := m.files[p]; ok {
		return fstest.MapFS{"f": {}}.Stat("f")
	}
	return nil, fs.ErrNotExist
}

func (m *memFS) DirFS(dir string) fs.FS {
	out := fstest.MapFS{}
	for p, body := range m.files {
		if rel, err := filepath.Rel(dir, p); err == nil && !strings.HasPrefix(rel, "..") {
			out[filepath.ToSlash(rel)] = &fstest.MapFile{Data: []byte(body)}
		}
	}
	return out
}

func (m *memFS) MkdirAll(string, fs.FileMode) error       { return nil }
func (m *memFS) Remove(string) error                      { return nil }
func (m *memFS) RemoveAll(string) error                   { return nil }
func (m *memFS) MkdirTemp(string, string) (string, error) { return "", errors.New("unused") }
func (m *memFS) Rename(string, string) error              { return errors.New("unused") }

const py = ".venv/bin/python"

var venv = map[string]string{".venv/pyvenv.cfg": "", ".venv/bin/python": ""}

func with(base map[string]string, extra map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func newPIP(t *testing.T, m *memFS) *PIP {
	t.Helper()
	p, err := newFor(m, "linux")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func call(t *testing.T, m *memFS, c core.Capability, req core.Request, fake *execx.FakeRunner) (core.Result, error) {
	t.Helper()
	req.Dir = proj
	return newPIP(t, m).Execute(execx.WithRunner(context.Background(), fake), c, req)
}

func argv(fake *execx.FakeRunner) []string {
	out := make([]string, len(fake.Calls))
	for i, c := range fake.Calls {
		out[i] = c.String()
	}
	return out
}

func respond(m map[string]string) *execx.FakeRunner {
	f := &execx.FakeRunner{Responses: map[string]execx.Response{}}
	for cmd, out := range m {
		f.Responses[cmd] = execx.Response{Output: execx.Output{Stdout: out}}
	}
	return f
}
