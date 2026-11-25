package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// exportVerbs are the commands a generated recipe file offers.
var exportVerbs = []core.Capability{
	core.CapInstall, core.CapBuild, core.CapTest, core.CapLint, core.CapAudit,
	core.CapOutdated, core.CapRun, core.CapUp, core.CapDown,
}

func newExportCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Write the project's commands as a recipe file (a justfile)",
		Long: "Write a recipe file whose recipes call upc, for example a justfile with " +
			"`build`, `test` and `up`. An existing file is kept unless --force is given.",
		Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := sessionFrom(cmd)
			if err != nil {
				return err
			}
			return runExport(cmd, s, force)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing recipe file")
	return cmd
}

func runExport(cmd *cobra.Command, s *Session, force bool) error {
	dir, err := projectDir(s)
	if err != nil {
		return err
	}
	exporter, ok := firstExporter(s.Registries.RunSources)
	if !ok {
		return fmt.Errorf("%w: no recipe format is registered", core.ErrUnsupported)
	}
	verbs := make([]string, len(exportVerbs))
	for i, c := range exportVerbs {
		verbs[i] = string(c)
	}
	file := exporter.RecipeFile(cmd.Root().Name(), verbs)
	path := filepath.Join(dir, file.Name)
	if exporter.Exists(dir) && !force {
		return fmt.Errorf("%w: %s already exists; use --force to overwrite it", core.ErrInvalidRequest, path)
	}
	if _, err := s.FS.Stat(dir); err != nil && errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: project directory %s does not exist", core.ErrInvalidRequest, dir)
	}
	if s.DryRun {
		return s.print(core.Result{Summary: "dry run, nothing changed: would write " + path})
	}
	if err := s.FS.WriteFileAtomic(path, []byte(file.Body), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return s.print(core.Result{Summary: "wrote " + path, Data: map[string]string{"path": path}})
}

func firstExporter(sources []core.RunSource) (core.RecipeExporter, bool) {
	for _, src := range sources {
		if e, ok := src.(core.RecipeExporter); ok {
			return e, true
		}
	}
	return nil, false
}
