package config

import (
	"path/filepath"

	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// FindRoot walks up from start looking for a project root. A directory holding
// DirName wins; otherwise the nearest directory a registered package manager
// or language detects. globalDir is skipped so the global config is never mistaken for a
// project. It returns "" when nothing is found.
func FindRoot(fsys fsx.FS, start, globalDir string, regs core.Registries) string {
	for _, byMarker := range []bool{false, true} {
		for dir := start; ; dir = filepath.Dir(dir) {
			if matches(fsys, dir, globalDir, regs, byMarker) {
				return dir
			}
			if parent := filepath.Dir(dir); parent == dir {
				break
			}
		}
	}
	return ""
}

func matches(fsys fsx.FS, dir, globalDir string, regs core.Registries, byMarker bool) bool {
	if !byMarker {
		if filepath.Join(dir, DirName) == globalDir {
			return false
		}
		info, err := fsys.Stat(filepath.Join(dir, DirName))
		return err == nil && info.IsDir()
	}
	dirFS := fsys.DirFS(dir)
	for _, pm := range regs.PackageManagers.List() {
		if pm.Detect(dirFS) {
			return true
		}
	}
	for _, lang := range regs.Languages.List() {
		if lang.Detect(dirFS) {
			return true
		}
	}
	return false
}
