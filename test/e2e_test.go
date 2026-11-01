package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

// upcBin is built once by TestMain and exposed to scripts as `upc`.
var upcBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "upc-e2e-")
	if err != nil {
		panic(err)
	}
	upcBin = filepath.Join(dir, "upc")
	build := exec.Command("go", "build", "-o", upcBin, "../cmd/upc")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		_ = os.RemoveAll(dir)
		panic(err)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata",
		Setup: func(env *testscript.Env) error {
			env.Setenv("PATH", filepath.Dir(upcBin)+string(os.PathListSeparator)+env.Getenv("PATH"))
			env.Setenv("UPC_HOME", filepath.Join(env.WorkDir, "upc-home"))
			return nil
		},
	})
}
