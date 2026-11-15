package cli

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/spf13/cobra"

	"github.com/justinfischman/universal-package-core/internal/components/envs"
	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// envVar is the variable wrapped commands receive with the active name.
const envVar = "UPC_ENV"

func newEnvCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Add, remove, list, select and show the project's environments",
		Args:  usageArgs(cobra.NoArgs),
		RunE:  func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	cmd.AddCommand(newEnvListCmd(), newEnvUseCmd(), newEnvShowCmd(), newEnvAddCmd(), newEnvRemoveCmd())
	return cmd
}

// envEntry is one environment as listed.
type envEntry struct {
	envs.Entry
	Active bool `json:"active"`
}

type envReport struct {
	Active  string     `json:"active,omitempty"`
	Default string     `json:"default,omitempty"`
	Envs    []envEntry `json:"environments"`
}

// loadEnvs discovers the project's environments from the merged config and the
// project directory. It lists names and paths only; no env file is opened.
func loadEnvs(s *Session) (config.Resolved, []envs.Entry, error) {
	dir, err := projectDir(s)
	if err != nil {
		return config.Resolved{}, nil, err
	}
	res, err := s.Store.Load()
	if err != nil {
		return config.Resolved{}, nil, err
	}
	entries, err := discoverEnvs(s.FS.DirFS(dir), res.Config)
	return res, entries, err
}

func discoverEnvs(root fs.FS, c config.Config) ([]envs.Entry, error) {
	declared := make(map[string]envs.Declared, len(c.Environments))
	for name, e := range c.Environments {
		declared[name] = envs.Declared{File: e.File, Groups: e.Groups}
	}
	return envs.Discover(root, declared)
}

func newEnvListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List declared and discovered environments",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := sessionFrom(cmd)
			if err != nil {
				return err
			}
			res, entries, err := loadEnvs(s)
			if err != nil {
				return err
			}
			active := res.EffectiveEnvironment()
			rep := envReport{Active: active, Default: res.DefaultEnvironment, Envs: []envEntry{}}
			var b strings.Builder
			for _, e := range entries {
				rep.Envs = append(rep.Envs, envEntry{Entry: e, Active: e.Name == active})
				mark := " "
				if e.Name == active {
					mark = "*"
				}
				note := ""
				if !e.Exists {
					note = " (file missing)"
				}
				fmt.Fprintf(&b, "%s %-12s %s [%s]%s%s\n", mark, e.Name, e.File, e.Source, groupNote(e), note)
			}
			if len(entries) == 0 {
				b.WriteString("no environments declared or found\n")
			}
			return s.print(core.Result{Summary: strings.TrimRight(b.String(), "\n"), Data: rep})
		},
	}
}

func newEnvShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show the active environment (name and file path, never values)",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := sessionFrom(cmd)
			if err != nil {
				return err
			}
			res, entries, err := loadEnvs(s)
			if err != nil {
				return err
			}
			name := res.EffectiveEnvironment()
			if name == "" {
				return s.print(core.Result{Summary: "no active environment (run `upc env use <name>`)", Data: envReport{Envs: []envEntry{}}})
			}
			e, ok := envs.Find(entries, name)
			if !ok {
				return fmt.Errorf("%w: active environment %q does not exist; run `upc env list`", core.ErrInvalidRequest, name)
			}
			summary := fmt.Sprintf("%s (%s) [%s]%s", e.Name, e.File, e.Source, groupNote(e))
			return s.print(core.Result{Summary: summary, Data: envEntry{Entry: e, Active: true}})
		},
	}
}

func newEnvUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Select the active environment for this project",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := sessionFrom(cmd)
			if err != nil {
				return err
			}
			name := args[0]
			if s.Global {
				return fmt.Errorf("%w: environments are per project; --global is not supported for `env use`", core.ErrInvalidRequest)
			}
			if s.Store.ProjectRoot == "" {
				return fmt.Errorf("%w: no project found; run inside a project or pass --project <dir>", core.ErrInvalidRequest)
			}
			if err := envs.ValidName(name); err != nil {
				return fmt.Errorf("%w: %v", core.ErrInvalidRequest, err)
			}
			_, entries, err := loadEnvs(s)
			if err != nil {
				return err
			}
			if err := envs.Check(entries, name); err != nil {
				return fmt.Errorf("%w: %v", core.ErrInvalidRequest, err)
			}
			if s.DryRun {
				return s.print(core.Result{Summary: fmt.Sprintf("would select environment %s", name)})
			}
			own, err := s.Store.LoadScope(config.Project)
			if err != nil {
				return err
			}
			own.ActiveEnvironment = name
			if err := s.Store.Save(config.Project, own); err != nil {
				return err
			}
			s.Logger.Info("environment selected", "name", name)
			return s.print(core.Result{Summary: fmt.Sprintf("active environment: %s", name), Data: map[string]string{"active": name}})
		},
	}
}

// envFindings is doctor's environment check: a selected (or default)
// environment must still exist and have its file.
func envFindings(s *Session, root fs.FS, res config.Resolved) []findingEntry {
	name := res.EffectiveEnvironment()
	if name == "" {
		return nil
	}
	entries, err := discoverEnvs(root, res.Config)
	if err != nil {
		return []findingEntry{{Severity: core.SeverityWarn, Message: err.Error()}}
	}
	if err := envs.Check(entries, name); err != nil {
		s.Logger.Debug("environment check failed", "name", name)
		return []findingEntry{{Severity: core.SeverityWarn, Message: "active " + err.Error()}}
	}
	return nil
}

// groupNote is the " groups=a,b" suffix of a listing, or "" without groups.
func groupNote(e envs.Entry) string {
	if len(e.Groups) == 0 {
		return ""
	}
	return " groups=" + strings.Join(e.Groups, ",")
}

// selectEnv makes name the environment for this invocation only (`--env`),
// without saving it. It must exist and have its file. Only the name and the
// groups are used; the env file is never opened.
func (s *Session) selectEnv(name string) (envs.Entry, error) {
	if err := envs.ValidName(name); err != nil {
		return envs.Entry{}, fmt.Errorf("%w: %v", core.ErrInvalidRequest, err)
	}
	_, entries, err := loadEnvs(s)
	if err != nil {
		return envs.Entry{}, err
	}
	if err := envs.Check(entries, name); err != nil {
		return envs.Entry{}, fmt.Errorf("%w: %v", core.ErrInvalidRequest, err)
	}
	e, _ := envs.Find(entries, name)
	s.EnvOverride = name
	return e, nil
}
