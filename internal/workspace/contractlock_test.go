package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core"
)

const specV1 = `openapi: 3.0.0
paths:
  /pets:
    get:
      responses:
        "200": {description: ok}
`

const specV2 = specV1 + `    post:
      responses:
        "201": {description: created}
`

func lockFixture(t *testing.T, contract string) (*ContractLocks, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "api"), 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "api", "openapi.yaml")
	if err := os.WriteFile(file, []byte(contract), 0o644); err != nil {
		t.Fatal(err)
	}
	w, err := Load(root, config.Config{Apps: map[string]config.App{
		"api":    {Path: "api", Contract: "openapi.yaml"},
		"nospec": {Path: "nospec"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return NewContractLocks(w), file
}

func TestContractLocksLifecycle(t *testing.T) {
	locks, file := lockFixture(t, specV1)

	st, err := locks.Status("api")
	if err != nil || st.State != ContractMissing {
		t.Fatalf("before update: %+v, %v; want missing", st, err)
	}
	if _, err := os.Stat(locks.LockPath("api")); !os.IsNotExist(err) {
		t.Fatalf("Status must not write a lock: %v", err)
	}

	if err := locks.Update("api"); err != nil {
		t.Fatal(err)
	}
	if st, err = locks.Status("api"); err != nil || st.State != ContractOK {
		t.Fatalf("after update: %+v, %v; want ok", st, err)
	}
	if !strings.HasSuffix(filepath.ToSlash(locks.LockPath("api")), ContractLockDir+"/api.lock") {
		t.Errorf("lock path %q is outside %s", locks.LockPath("api"), ContractLockDir)
	}

	if err := os.WriteFile(file, []byte(specV2), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err = locks.Status("api")
	if err != nil || st.State != ContractChanged || !strings.Contains(st.Summary, "+ POST /pets") {
		t.Fatalf("after edit: %+v, %v; want changed with + POST /pets", st, err)
	}
	if again, _ := locks.Status("api"); again.State != ContractChanged {
		t.Fatal("Status must not refresh the lock")
	}

	if err := locks.Update("api"); err != nil {
		t.Fatal(err)
	}
	if st, _ = locks.Status("api"); st.State != ContractOK {
		t.Fatalf("after second update: %+v; want ok", st)
	}
}

func TestContractLocksIgnoreFormatting(t *testing.T) {
	locks, file := lockFixture(t, specV1)
	if err := locks.Update("api"); err != nil {
		t.Fatal(err)
	}
	reformatted := "# comment\nopenapi: '3.0.0'\npaths:\n  /pets:\n    get:\n      responses:\n        '200':\n          description: ok\n"
	if err := os.WriteFile(file, []byte(reformatted), 0o644); err != nil {
		t.Fatal(err)
	}
	if st, err := locks.Status("api"); err != nil || st.State != ContractOK {
		t.Fatalf("formatting-only edit: %+v, %v; want ok", st, err)
	}
}

func TestContractLocksErrors(t *testing.T) {
	locks, _ := lockFixture(t, specV1)
	for _, name := range []string{"ghost", "nospec"} {
		if _, err := locks.Status(name); !errors.Is(err, core.ErrInvalidRequest) {
			t.Errorf("Status(%q) = %v; want ErrInvalidRequest", name, err)
		}
		if err := locks.Update(name); !errors.Is(err, core.ErrInvalidRequest) {
			t.Errorf("Update(%q) = %v; want ErrInvalidRequest", name, err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(locks.LockPath("api")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(locks.LockPath("api"), []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := locks.Status("api"); !errors.Is(err, core.ErrInvalidRequest) {
		t.Errorf("corrupt lock: %v; want ErrInvalidRequest", err)
	}
}

func TestContractLocksRejectEscapingContract(t *testing.T) {
	root := t.TempDir()
	w, err := Load(root, config.Config{Apps: map[string]config.App{"a": {Path: "a", Contract: "../secret.yaml"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewContractLocks(w).Status("a"); !errors.Is(err, core.ErrInvalidRequest) {
		t.Fatalf("got %v; want ErrInvalidRequest", err)
	}
}
