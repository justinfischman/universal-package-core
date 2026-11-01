package node

import (
	"errors"
	"io/fs"
	"reflect"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/core"
)

const sample = `{
  "name": "demo",
  "version": "1.2.3",
  "scripts": {"build": "tsc", "test": "node --test"},
  "engines": {"node": ">=22"},
  "packageManager": "npm@11.19.1"
}`

func TestReadProject(t *testing.T) {
	p, err := ReadProject(mapFS(map[string]string{"package.json": sample}))
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "demo" || p.Version != "1.2.3" || p.PackageManager != "npm@11.19.1" || p.Engines["node"] != ">=22" {
		t.Errorf("project = %+v", p)
	}
	if !reflect.DeepEqual(p.Scripts, map[string]string{"build": "tsc", "test": "node --test"}) {
		t.Errorf("scripts = %v", p.Scripts)
	}
}

func TestReadProjectErrors(t *testing.T) {
	if _, err := ReadProject(mapFS(nil)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing manifest: err = %v, want ErrNotExist", err)
	}
	for name, body := range map[string]string{"syntax": "{bad", "script type": `{"scripts":{"a":1}}`} {
		if _, err := ReadProject(mapFS(map[string]string{"package.json": body})); !errors.Is(err, core.ErrInvalidRequest) {
			t.Errorf("%s: err = %v, want ErrInvalidRequest", name, err)
		}
	}
}

func TestPackageFile(t *testing.T) {
	var pf core.PackageFile = Package{}
	if pf.Filename() != "package.json" {
		t.Errorf("filename = %q", pf.Filename())
	}
	m, err := pf.Read(mapFS(map[string]string{"package.json": sample}))
	if err != nil || m.Name != "demo" || m.Scripts["build"] != "tsc" {
		t.Errorf("manifest = %+v, err = %v", m, err)
	}
}

func TestScriptReader(t *testing.T) {
	table, err := ScriptReader{}.ReadScripts(mapFS(map[string]string{"package.json": sample}))
	if err != nil || !reflect.DeepEqual(table.Names(), []string{"build", "test"}) || len(table.EntryPoints) != 0 {
		t.Errorf("table = %+v, err = %v", table, err)
	}
	if _, err := (ScriptReader{}).ReadScripts(mapFS(nil)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing manifest: err = %v", err)
	}
}
