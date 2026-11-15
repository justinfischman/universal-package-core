package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check the project's tools, configuration and capabilities, and offer fixes",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := sessionFrom(cmd)
			if err != nil {
				return err
			}
			return runDoctor(cmd, s)
		},
	}
}

// doctorReport is the machine-readable result of doctor.
type doctorReport struct {
	Project        string            `json:"project"`
	Language       string            `json:"language,omitempty"`
	PackageManager string            `json:"package_manager,omitempty"`
	Source         string            `json:"source,omitempty"`
	Tool           toolReport        `json:"tool"`
	Capabilities   []capabilityEntry `json:"capabilities"`
	Findings       []findingEntry    `json:"findings"`
}

type toolReport struct {
	Binary  string `json:"binary,omitempty"`
	Found   bool   `json:"found"`
	Checked bool   `json:"checked"`
	Version string `json:"version,omitempty"`
}

type capabilityEntry struct {
	Capability core.Capability `json:"capability"`
	Level      core.Level      `json:"level"`
	Reason     string          `json:"reason,omitempty"`
}

type findingEntry struct {
	Severity core.Severity `json:"severity"`
	Message  string        `json:"message"`
	Fix      string        `json:"fix,omitempty"`
	Applied  bool          `json:"applied,omitempty"`
}

// fix is something doctor can do about a finding.
type fix struct {
	finding int
	prompt  string
	apply   func(context.Context) error
}

func runDoctor(cmd *cobra.Command, s *Session) error {
	dir, err := projectDir(s)
	if err != nil {
		return err
	}
	rep := doctorReport{Project: dir, Capabilities: []capabilityEntry{}, Findings: []findingEntry{}}
	st, err := resolveStack(s)
	if errors.Is(err, errNothingDetected) {
		rep.Findings = append(rep.Findings, findingEntry{Severity: core.SeverityWarn, Message: err.Error()})
		rep.Findings = append(rep.Findings, workspaceFindings(s, dir)...)
		return s.printDoctor(rep)
	}
	if err != nil {
		return err
	}
	rep.Language, rep.PackageManager, rep.Source = st.Language.ID(), st.PM.ID(), st.Source
	for _, c := range core.AllCapabilities {
		sup := st.PM.Capabilities()[c]
		rep.Capabilities = append(rep.Capabilities, capabilityEntry{Capability: c, Level: sup.Level, Reason: sup.Reason})
	}

	var fixes []fix
	add := func(f findingEntry, apply func(context.Context) error, prompt string) {
		rep.Findings = append(rep.Findings, f)
		if apply != nil {
			fixes = append(fixes, fix{finding: len(rep.Findings) - 1, prompt: prompt, apply: apply})
		}
	}

	ctx := execx.WithRunner(cmd.Context(), s.Runner)
	rep.Tool = probeTool(ctx, s, st.PM.Tool())
	if rep.Tool.Checked && !rep.Tool.Found {
		tool := st.PM.Tool()
		var apply func(context.Context) error
		if len(tool.InstallCmd) > 0 {
			missing := &core.ToolMissingError{Tool: tool.Binary, InstallHint: tool.InstallHint, InstallCmd: tool.InstallCmd}
			apply = func(ctx context.Context) error { return s.installTool(ctx, missing) }
		}
		add(findingEntry{Severity: core.SeverityWarn, Message: fmt.Sprintf("%s is not installed (%s)", tool.Binary, tool.InstallHint), Fix: "install " + tool.Binary},
			apply, fmt.Sprintf("Install %s with `%s`?", tool.Binary, tool.InstallHint))
	}

	dirFS := s.FS.DirFS(dir)
	res, err := s.Store.Load()
	if err != nil {
		return err
	}
	if res.PackageManager != "" && !st.PM.Detect(dirFS) {
		add(findingEntry{Severity: core.SeverityWarn, Message: fmt.Sprintf(
			"config says %s but %s has none of its marker files; it may not be this project's manager (see `upc config get`)", res.PackageManager, dir)}, nil, "")
	}
	for _, f := range driftFindings(s, dir, dirFS, res) {
		add(f, nil, "")
	}
	for _, f := range envFindings(s, dirFS, res) {
		add(f, nil, "")
	}
	for _, f := range workspaceFindings(s, dir) {
		add(f, nil, "")
	}
	if d, ok := st.PM.(core.Diagnoser); ok {
		for _, f := range d.Diagnose(dirFS) {
			entry := findingEntry{Severity: f.Severity, Message: f.Message}
			var apply func(context.Context) error
			if f.Fix != "" {
				entry.Fix = string(f.Fix)
				fixCap := f.Fix
				apply = func(ctx context.Context) error {
					_, err := s.execute(ctx, st.PM, fixCap, core.Request{Dir: dir, Yes: s.Yes})
					return err
				}
			}
			add(entry, apply, fmt.Sprintf("Fix by running `upc %s`?", f.Fix))
		}
	}

	applyFixes(ctx, s, &rep, fixes)
	return s.printDoctor(rep)
}

// probeTool runs the tool's version command. A dry run does not run anything.
func probeTool(ctx context.Context, s *Session, tool core.Tool) toolReport {
	rep := toolReport{Binary: tool.Binary}
	if s.DryRun {
		return rep
	}
	rep.Checked = true
	out, err := s.Runner.Run(ctx, execx.Cmd{Name: tool.Binary, Args: tool.VersionArgs})
	var missing *core.ToolMissingError
	switch {
	case errors.As(err, &missing):
	case err != nil:
		s.Logger.Warn("version probe failed", "tool", tool.Binary, "err", err)
		rep.Found = true
	default:
		rep.Found = true
		rep.Version = strings.TrimSpace(out.Stdout)
	}
	return rep
}

// applyFixes offers each fix. With --yes they apply; interactively the user is
// asked; non-interactively without --yes they are only suggested.
func applyFixes(ctx context.Context, s *Session, rep *doctorReport, fixes []fix) {
	if s.DryRun {
		return
	}
	for _, f := range fixes {
		ok, err := s.Prompter.Confirm(f.prompt, true)
		if err != nil || !ok {
			s.Logger.Info("fix not applied", "finding", rep.Findings[f.finding].Message, "asked", err == nil)
			continue
		}
		if err := f.apply(ctx); err != nil {
			s.Logger.Error("fix failed", "finding", rep.Findings[f.finding].Message, "err", err)
			continue
		}
		rep.Findings[f.finding].Applied = true
	}
}

func (s *Session) printDoctor(rep doctorReport) error {
	var b strings.Builder
	fmt.Fprintf(&b, "project: %s\n", rep.Project)
	if rep.Language != "" {
		fmt.Fprintf(&b, "stack: %s / %s (%s)\n", rep.Language, rep.PackageManager, rep.Source)
		switch {
		case !rep.Tool.Checked:
			fmt.Fprintf(&b, "tool %s: not checked (dry run)\n", rep.Tool.Binary)
		case rep.Tool.Found:
			fmt.Fprintf(&b, "tool %s: found %s\n", rep.Tool.Binary, rep.Tool.Version)
		default:
			fmt.Fprintf(&b, "tool %s: missing\n", rep.Tool.Binary)
		}
		b.WriteString("capabilities:\n")
		for _, c := range rep.Capabilities {
			fmt.Fprintf(&b, "  %-9s %s", c.Capability, c.Level)
			if c.Reason != "" {
				fmt.Fprintf(&b, " (%s)", c.Reason)
			}
			b.WriteString("\n")
		}
	}
	if len(rep.Findings) == 0 {
		b.WriteString("findings: none\n")
	} else {
		b.WriteString("findings:\n")
		for _, f := range rep.Findings {
			fmt.Fprintf(&b, "  %s: %s", f.Severity, f.Message)
			if f.Applied {
				b.WriteString(" [fixed]")
			} else if f.Fix != "" {
				fmt.Fprintf(&b, " [fix: %s; run with --yes to apply]", f.Fix)
			}
			b.WriteString("\n")
		}
	}
	return s.print(core.Result{Summary: strings.TrimRight(b.String(), "\n"), Data: rep})
}
