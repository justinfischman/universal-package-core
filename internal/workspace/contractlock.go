package workspace

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/contracts"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// ContractLockDir is where lock files live, relative to the workspace root.
// The directory is committed; see the init gitignore template.
const ContractLockDir = ".universal-package-core/contracts"

// ContractState is the result of comparing an app's contract with its lock.
type ContractState string

const (
	// ContractMissing means no lock file exists for the app.
	ContractMissing ContractState = "missing"
	// ContractOK means the contract still matches its lock.
	ContractOK ContractState = "ok"
	// ContractChanged means the contract differs from its lock.
	ContractChanged ContractState = "changed"
)

// ContractStatus is the outcome of ContractLocks.Status.
type ContractStatus struct {
	State ContractState
	// Summary lists what changed; set only when State is ContractChanged.
	Summary string
}

// ContractLocks reads and writes the committed contract lock of each app. A
// lock changes only through Update, never as a side effect of Status.
type ContractLocks struct {
	workspace *Workspace
	fs        fsx.OS
}

// NewContractLocks returns the lock store of a workspace.
func NewContractLocks(w *Workspace) *ContractLocks { return &ContractLocks{workspace: w} }

// LockPath is the absolute path of the app's lock file.
func (c *ContractLocks) LockPath(app string) string {
	return filepath.Join(c.workspace.Root, filepath.FromSlash(ContractLockDir), app+".lock")
}

// Status compares the app's current contract with its lock.
func (c *ContractLocks) Status(appName string) (ContractStatus, error) {
	app, contractPath, err := c.contractOf(appName)
	if err != nil {
		return ContractStatus{}, err
	}
	data, err := c.fs.ReadFile(c.LockPath(app.Name))
	if errors.Is(err, fs.ErrNotExist) {
		return ContractStatus{State: ContractMissing}, nil
	}
	if err != nil {
		return ContractStatus{}, fmt.Errorf("read lock for app %q: %w", appName, err)
	}
	locked, err := parseLock(data)
	if err != nil {
		return ContractStatus{}, fmt.Errorf("%w: lock of app %q: %v", core.ErrInvalidRequest, appName, err)
	}
	checker, ok := contracts.ByName(locked.checker)
	if !ok {
		return ContractStatus{}, fmt.Errorf("%w: lock of app %q names unknown checker %q", core.ErrInvalidRequest, appName, locked.checker)
	}
	snapshot, err := checker.Snapshot(contractPath)
	if err != nil {
		return ContractStatus{}, err
	}
	if sum(snapshot) == locked.sha256 && bytes.Equal(snapshot, locked.snapshot) {
		return ContractStatus{State: ContractOK}, nil
	}
	return ContractStatus{State: ContractChanged, Summary: checker.Diff(locked.snapshot, snapshot)}, nil
}

// Update writes the app's lock from its current contract. It is called only on
// explicit request.
func (c *ContractLocks) Update(appName string) error {
	app, contractPath, err := c.contractOf(appName)
	if err != nil {
		return err
	}
	checker, err := contracts.Detect(contractPath)
	if err != nil {
		return err
	}
	snapshot, err := checker.Snapshot(contractPath)
	if err != nil {
		return err
	}
	lock := contractLock{checker: checker.Name(), sha256: sum(snapshot), snapshot: snapshot}
	lockPath := c.LockPath(app.Name)
	if err := c.fs.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return fmt.Errorf("create lock directory: %w", err)
	}
	return c.fs.WriteFileAtomic(lockPath, lock.encode(), 0o644)
}

// contractOf resolves the app and the absolute path of its contract file.
func (c *ContractLocks) contractOf(name string) (*App, string, error) {
	app, ok := c.workspace.App(name)
	if !ok {
		return nil, "", fmt.Errorf("%w: unknown app %q", core.ErrInvalidRequest, name)
	}
	if app.Contract == "" {
		return nil, "", fmt.Errorf("%w: app %q declares no contract", core.ErrInvalidRequest, name)
	}
	clean := path.Clean(app.Contract)
	if path.IsAbs(app.Contract) || strings.Contains(app.Contract, `\`) || clean == ".." || strings.HasPrefix(clean, "../") {
		return nil, "", fmt.Errorf("%w: app %q contract %q must be a relative path inside the app", core.ErrInvalidRequest, name, app.Contract)
	}
	if _, err := os.Stat(filepath.Join(app.Dir, filepath.FromSlash(clean))); err != nil {
		return nil, "", fmt.Errorf("%w: app %q contract %q: %v", core.ErrInvalidRequest, name, app.Contract, err)
	}
	return app, filepath.Join(app.Dir, filepath.FromSlash(clean)), nil
}

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// contractLock is the parsed content of a lock file:
//
//	checker: <name>
//	sha256: <hex of the snapshot>
//	---
//	<normalized snapshot>
type contractLock struct {
	checker  string
	sha256   string
	snapshot []byte
}

func (l contractLock) encode() []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "checker: %s\nsha256: %s\n---\n", l.checker, l.sha256)
	b.Write(l.snapshot)
	return b.Bytes()
}

func parseLock(data []byte) (contractLock, error) {
	head, body, ok := bytes.Cut(data, []byte("\n---\n"))
	if !ok {
		return contractLock{}, errors.New("missing \"---\" separator")
	}
	var l contractLock
	for _, line := range strings.Split(string(head), "\n") {
		key, val, _ := strings.Cut(line, ": ")
		switch key {
		case "checker":
			l.checker = val
		case "sha256":
			l.sha256 = val
		}
	}
	if l.checker == "" || l.sha256 == "" {
		return contractLock{}, errors.New("missing checker or sha256 header")
	}
	l.snapshot = body
	return l, nil
}
