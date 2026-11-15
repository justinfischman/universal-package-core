package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/justinfischman/universal-package-core/internal/components/envs"
	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// projectScope is the shared precondition of the commands that edit the
// project's own config: a project exists and --global is not in effect.
func projectScope(s *Session, command string) error {
	if s.Global {
		return fmt.Errorf("%w: environments are per project; --global is not supported for `env %s`", core.ErrInvalidRequest, command)
	}
	if s.Store.ProjectRoot == "" {
		return fmt.Errorf("%w: no project found; run inside a project or pass --project <dir>", core.ErrInvalidRequest)
	}
	return nil
}

func newEnvAddCmd() *cobra.Command {
	var file string
	var groups []string
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Declare an environment (name, optional env file, dependency groups)",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := sessionFrom(cmd)
			if err != nil {
				return err
			}
			name := args[0]
			if err := projectScope(s, "add"); err != nil {
				return err
			}
			decl, err := validateDeclaration(name, file, groups)
			if err != nil {
				return err
			}
			own, err := s.Store.LoadScope(config.Project)
			if err != nil {
				return err
			}
			if _, dup := own.Environments[name]; dup {
				return fmt.Errorf("%w: environment %q is already declared; run `upc env remove %s` first", core.ErrInvalidRequest, name, name)
			}
			if s.DryRun {
				return s.print(core.Result{Summary: fmt.Sprintf("would declare environment %s", name)})
			}
			if own.Environments == nil {
				own.Environments = map[string]config.Environment{}
			}
			own.Environments[name] = config.Environment{File: decl.File, Groups: decl.Groups}
			if err := s.Store.Save(config.Project, own); err != nil {
				return err
			}
			s.Logger.Info("environment declared", "name", name)
			return s.print(core.Result{Summary: "declared environment " + describeDecl(name, decl), Data: decl.report(name)})
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "env file path relative to the project (default .env.<name>)")
	cmd.Flags().StringSliceVar(&groups, "groups", nil, "dependency groups this environment installs, comma separated")
	return cmd
}

func newEnvRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a declared environment (the env file is left untouched)",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := sessionFrom(cmd)
			if err != nil {
				return err
			}
			name := args[0]
			if err := projectScope(s, "remove"); err != nil {
				return err
			}
			own, err := s.Store.LoadScope(config.Project)
			if err != nil {
				return err
			}
			if _, ok := own.Environments[name]; !ok {
				return fmt.Errorf("%w: environment %q is not declared in the project config; `upc env list` shows what exists", core.ErrInvalidRequest, name)
			}
			if s.DryRun {
				return s.print(core.Result{Summary: fmt.Sprintf("would remove environment %s", name)})
			}
			delete(own.Environments, name)
			if err := s.Store.Save(config.Project, own); err != nil {
				return err
			}
			s.Logger.Info("environment removed", "name", name)
			summary := "removed environment " + name
			if own.ActiveEnvironment == name || own.DefaultEnvironment == name {
				summary += " (it was still selected; run `upc env use <name>` to pick another)"
			}
			return s.print(core.Result{Summary: summary, Data: map[string]string{"removed": name}})
		},
	}
}

// declaration is a validated `env add` request.
type declaration struct {
	File   string
	Groups []string
}

type declReport struct {
	Name   string   `json:"name"`
	File   string   `json:"file,omitempty"`
	Groups []string `json:"groups,omitempty"`
}

func (d declaration) report(name string) declReport {
	return declReport{Name: name, File: d.File, Groups: d.Groups}
}

// validateDeclaration checks the name, file and groups of a new environment
// without touching the disk.
func validateDeclaration(name, file string, groups []string) (declaration, error) {
	if err := envs.ValidName(name); err != nil {
		return declaration{}, fmt.Errorf("%w: %v", core.ErrInvalidRequest, err)
	}
	if file != "" {
		if err := envs.ValidFile(file); err != nil {
			return declaration{}, fmt.Errorf("%w: %v", core.ErrInvalidRequest, err)
		}
	}
	seen := map[string]bool{}
	var clean []string
	for _, g := range groups {
		if err := envs.ValidGroup(g); err != nil {
			return declaration{}, fmt.Errorf("%w: %v", core.ErrInvalidRequest, err)
		}
		if !seen[g] {
			seen[g] = true
			clean = append(clean, g)
		}
	}
	return declaration{File: file, Groups: clean}, nil
}

func describeDecl(name string, d declaration) string {
	out := name
	if d.File != "" {
		out += " (" + d.File + ")"
	}
	if len(d.Groups) > 0 {
		out += " groups: " + strings.Join(d.Groups, ",")
	}
	return out
}
