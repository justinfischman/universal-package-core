package cli

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/config"
)

// runIgnoreEntry is the run state directory (up.pid, up.log, provider) that
// `upc up` writes; it is machine-local and must not be committed.
const runIgnoreEntry = config.DirName + "/run/"

// ensureRunIgnored makes sure dir/.gitignore ignores the run state directory.
// It is idempotent: an existing equivalent entry (with or without a leading or
// trailing slash, or the whole upc directory) leaves the file untouched, and
// the entry is never added twice. Other content is preserved. It reports
// whether the file changed.
func ensureRunIgnored(s *Session, dir string) (bool, error) {
	path := filepath.Join(dir, ".gitignore")
	data, err := s.FS.ReadFile(path)
	exists := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	text := string(data)
	if ignoresRunDir(text) {
		return false, nil
	}
	if s.DryRun {
		return false, nil
	}
	if exists && text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	text += runIgnoreEntry + "\n"
	if err := s.FS.WriteFileAtomic(path, []byte(text), 0o644); err != nil {
		return false, err
	}
	s.Logger.Info("gitignore updated", "path", path, "entry", runIgnoreEntry)
	return true, nil
}

// ignoresRunDir reports whether a .gitignore body already has a plain pattern
// for the run directory or for the whole upc directory.
func ignoresRunDir(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		p := strings.TrimSpace(line)
		p = strings.TrimPrefix(p, "/")
		p = strings.TrimSuffix(p, "/")
		if p == config.DirName || p == config.DirName+"/run" {
			return true
		}
	}
	return false
}
