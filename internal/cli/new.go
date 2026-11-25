package cli

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func newNewCmd() *cobra.Command {
	var language, manager, name, kind string
	cmd := &cobra.Command{
		Use:   "new <type> [name]",
		Short: "Create a new project of a type (for example a script)",
		Args:  usageArgs(cobra.RangeArgs(1, 2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := sessionFrom(cmd)
			if err != nil {
				return err
			}
			if len(args) == 2 {
				if name != "" && name != args[1] {
					return fmt.Errorf("%w: name given twice (%q and --name %q)", core.ErrInvalidRequest, args[1], name)
				}
				name = args[1]
			}
			return runNew(cmd, s, args[0], name, kind, choice{language: language, manager: manager})
		},
	}
	cmd.Flags().StringVar(&language, "language", "", "language of the project (default: configured, or asked)")
	cmd.Flags().StringVar(&manager, "pm", "", "package manager of the project (default: the language's default)")
	cmd.Flags().StringVar(&name, "name", "", "project name (alternative to the positional argument)")
	cmd.Flags().StringVar(&kind, "kind", "", "app sub-type: cli or library (default: a service)")
	return cmd
}

func runNew(cmd *cobra.Command, s *Session, typeID, name, kind string, want choice) error {
	if s.Global {
		return fmt.Errorf("%w: --global applies to `config`, not to `new`", core.ErrInvalidRequest)
	}
	pt, ok := s.Registries.ProjectTypes.Get(typeID)
	if !ok {
		return fmt.Errorf("%w: unknown project type %q (known: %v)", core.ErrInvalidRequest, typeID, s.Registries.ProjectTypes.IDs())
	}
	if name == "" {
		return fmt.Errorf("%w: a name is required: upc new %s <name>", core.ErrInvalidRequest, typeID)
	}
	if kind != "" && typeID != "app" {
		return fmt.Errorf("%w: --kind applies to app projects, not %s", core.ErrInvalidRequest, typeID)
	}
	base, err := newBaseDir(s)
	if err != nil {
		return err
	}
	if want, err = withConfigDefaults(s, want); err != nil {
		return err
	}
	lang, pm, err := chooseStack(s, want, pt.Supports)
	if err != nil {
		return err
	}
	if !pt.Supports(lang.ID(), pm.ID()) {
		return fmt.Errorf("%w: %s projects cannot be created with %s/%s", core.ErrIncompatible, typeID, lang.ID(), pm.ID())
	}

	req := core.ScaffoldRequest{
		Request:        core.Request{Dir: base, DryRun: s.DryRun, Yes: s.Yes},
		Name:           name,
		Kind:           kind,
		Language:       lang.ID(),
		PackageManager: pm.ID(),
		Manager:        pm,
	}
	if t, ok := lang.(core.Templater); ok {
		if files, ok := t.Templates(typeID); ok {
			req.Templates = files
		}
	}
	s.Logger.Info("scaffold", "type", typeID, "language", lang.ID(), "pm", pm.ID(), "name", name, "dir", base)
	res, err := s.withInstall(cmd.Context(), func(ctx context.Context) (core.Result, error) { return pt.Scaffold(ctx, req) })
	if err != nil {
		return err
	}
	if !s.DryRun {
		dest := filepath.Join(base, name)
		store := config.Store{FS: s.FS, GlobalDir: s.Store.GlobalDir, ProjectRoot: dest}
		cfg := config.Config{Language: lang.ID(), PackageManager: pm.ID(), ProjectType: typeID}
		if err := store.Save(config.Project, cfg); err != nil {
			return fmt.Errorf("project created in %s, but writing its config failed: %w", dest, err)
		}
	}
	return s.print(res)
}

// newBaseDir is where a new project directory is created: --project, else the
// working directory (never a discovered parent project).
func newBaseDir(s *Session) (string, error) {
	if s.Project != "" {
		return s.Project, nil
	}
	cwd, err := s.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	return cwd, nil
}

// withConfigDefaults fills unset choices from the merged config, so a user who
// configured a language or manager is not asked again.
func withConfigDefaults(s *Session, want choice) (choice, error) {
	res, err := s.Store.Load()
	if err != nil {
		return want, err
	}
	if want.language == "" && want.manager == "" {
		want.language, want.manager = res.Language, res.PackageManager
	} else if want.manager == "" && want.language == res.Language {
		want.manager = res.PackageManager
	}
	return want, nil
}
