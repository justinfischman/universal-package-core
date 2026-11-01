// Package fsx wraps filesystem access behind an interface and provides
// atomic writes, so a failed write never leaves a partial file.
package fsx

import (
	"io/fs"
	"os"
	"path/filepath"
)

// FS is the filesystem surface upc needs.
type FS interface {
	ReadFile(path string) ([]byte, error)
	// WriteFileAtomic writes via a temp file in the same directory and renames it.
	WriteFileAtomic(path string, data []byte, perm fs.FileMode) error
	MkdirAll(path string, perm fs.FileMode) error
	Stat(path string) (fs.FileInfo, error)
	Remove(path string) error
	// RemoveAll removes a path and everything under it; a missing path is not an error.
	RemoveAll(path string) error
	// MkdirTemp creates a new temporary directory inside dir.
	MkdirTemp(dir, pattern string) (string, error)
	// Rename moves oldPath to newPath.
	Rename(oldPath, newPath string) error
	// DirFS exposes a directory as an fs.FS for read-only detection.
	DirFS(dir string) fs.FS
}

// OS is the real filesystem.
type OS struct{}

// ReadFile implements FS.
func (OS) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

// MkdirAll implements FS.
func (OS) MkdirAll(path string, perm fs.FileMode) error { return os.MkdirAll(path, perm) }

// Stat implements FS.
func (OS) Stat(path string) (fs.FileInfo, error) { return os.Stat(path) }

// Remove implements FS.
func (OS) Remove(path string) error { return os.Remove(path) }

// RemoveAll implements FS.
func (OS) RemoveAll(path string) error { return os.RemoveAll(path) }

// MkdirTemp implements FS.
// The directory is made readable like any other (0755) because staged output
// is renamed into place as a normal project directory.
func (OS) MkdirTemp(dir, pattern string) (string, error) {
	path, err := os.MkdirTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	if err := os.Chmod(path, 0o755); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

// Rename implements FS.
func (OS) Rename(oldPath, newPath string) error { return os.Rename(oldPath, newPath) }

// DirFS implements FS.
func (OS) DirFS(dir string) fs.FS { return os.DirFS(dir) }

// WriteFileAtomic implements FS. The parent directory is created if missing.
func (OS) WriteFileAtomic(path string, data []byte, perm fs.FileMode) (err error) {
	dir := filepath.Dir(path)
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".upc-*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmp.Name(), perm); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
