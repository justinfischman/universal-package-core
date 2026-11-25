package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

// exeSuffix is the executable extension of the host platform.
func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// upcBin is built once by TestMain and exposed to scripts as `upc`.
var upcBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "upc-e2e-")
	if err != nil {
		panic(err)
	}
	upcBin = filepath.Join(dir, "upc"+exeSuffix())
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

// TestE2EScripts runs the scripts that need real package managers. They are
// opt-in because they need the tools on PATH and network access.
func TestE2EScripts(t *testing.T) {
	if os.Getenv("UPC_E2E") != "1" {
		t.Skip("set UPC_E2E=1 to run the real package manager scripts")
	}
	testscript.Run(t, testscript.Params{
		Dir: "testdata/e2e",
		Setup: func(env *testscript.Env) error {
			env.Setenv("PATH", filepath.Dir(upcBin)+string(os.PathListSeparator)+env.Getenv("PATH"))
			env.Setenv("UPC_HOME", filepath.Join(env.WorkDir, "upc-home"))
			// Share the host's tool caches so the scripts stay fast.
			for _, k := range []string{"HOME", "XDG_CACHE_HOME"} {
				if v, ok := os.LookupEnv(k); ok {
					env.Setenv(k, v)
				}
			}
			return nil
		},
	})
}
