package cli

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Set up upc for an existing project and install its dependencies",
		Long: "Detects the language and package manager of the project, records them in " +
			".universal-package-core/config.toml, and installs the dependencies. " +
			"It never edits manifests or lockfiles.",
		Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := sessionFrom(cmd)
			if err != nil {
				return err
			}
			return runInit(cmd, s)
		},
	}
}

// initSummary is the machine-readable outcome of init.
type initSummary struct {
	Dir            string `json:"dir"`
	Language       string `json:"language"`
	PackageManager string `json:"package_manager"`
	Source         string `json:"source"`
	ConfigWritten  bool   `json:"config_written"`
	// GitignoreUpdated is true when init added the run directory to .gitignore.
	GitignoreUpdated bool `json:"gitignore_updated"`
	Installed        bool `json:"installed"`
}

func runInit(cmd *cobra.Command, s *Session) error {
	dir, err := newBaseDir(s)
	if err != nil {
		return err
	}
	if s.Global {
		return fmt.Errorf("%w: --global applies to `config`, not to `init`", core.ErrInvalidRequest)
	}
	// Act on this directory, not on a parent project discovered from it.
	local := *s
	local.Store.ProjectRoot = dir

	st, err := resolveStackIn(&local, dir)
	detected := err == nil
	switch {
	case errors.Is(err, errNothingDetected):
		if st, err = guidedStack(&local, dir); err != nil {
			return err
		}
	case err != nil:
		return err
	}

	out := initSummary{Dir: dir, Language: st.Language.ID(), PackageManager: st.PM.ID(), Source: st.Source}
	if out.ConfigWritten, err = writeInitConfig(&local, st); err != nil {
		return err
	}
	if out.GitignoreUpdated, err = ensureRunIgnored(&local, dir); err != nil {
		return err
	}

	var commands []string
	if detected {
		res, err := local.execute(cmd.Context(), st.PM, core.CapInstall, core.Request{Dir: dir, DryRun: s.DryRun, Yes: s.Yes})
		if err != nil {
			return err
		}
		commands = res.Commands
		out.Installed = !s.DryRun
	}
	summary := fmt.Sprintf("%s project set up with %s", out.Language, out.PackageManager)
	if !detected {
		summary += "; nothing to install yet (create a project with `upc new`)"
	}
	if s.DryRun {
		summary = "dry run, nothing changed: would set up " + out.Language + " with " + out.PackageManager
	}
	return s.print(core.Result{Summary: summary, Data: out, Commands: commands})
}

// guidedStack walks through choosing a language and manager for a directory in
// which nothing was detected.
func guidedStack(s *Session, dir string) (stack, error) {
	lang, pm, err := chooseStack(s, choice{}, func(string, string) bool { return true })
	if err != nil {
		return stack{}, err
	}
	return stack{Language: lang, PM: pm, Dir: dir, Source: "chosen"}, nil
}

// writeInitConfig records the stack in the project config unless it already
// names one. An existing file lacking the choices is filled in after
// confirmation; other keys are preserved.
func writeInitConfig(s *Session, st stack) (bool, error) {
	path := filepath.Join(st.Dir, config.DirName, config.FileName)
	existing, err := s.Store.LoadScope(config.Project)
	if err != nil {
		return false, err
	}
	_, statErr := s.FS.Stat(path)
	present := statErr == nil
	if existing.PackageManager != "" || existing.Language != "" {
		return false, nil // the user already chose; doctor reports drift
	}
	if present {
		ok, err := s.Prompter.Confirm(fmt.Sprintf("%s exists without a language or package manager. Add %s/%s?", path, st.Language.ID(), st.PM.ID()), true)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	existing.Language, existing.PackageManager = st.Language.ID(), st.PM.ID()
	if err := config.Validate(existing, s.Registries); err != nil {
		return false, err
	}
	if s.DryRun {
		return false, nil
	}
	if err := s.Store.Save(config.Project, existing); err != nil {
		return false, err
	}
	s.Logger.Info("config written", "path", path, "language", st.Language.ID(), "pm", st.PM.ID())
	return true, nil
}
