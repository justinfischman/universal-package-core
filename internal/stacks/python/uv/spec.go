package uv

import "github.com/justinfischman/universal-package-core/internal/core"

// commands is everything uv-specific that is a fact rather than logic: the
// executable, how to install it, the constraint dialect and every argv template
// (ADR-0002: a stack's knowledge stays in its package; ADR-0012: as data).
type commands struct {
	Tool    core.ToolSpec
	Dialect core.Dialect

	Add            core.CommandSpec
	Remove         core.CommandSpec
	Sync           core.CommandSpec
	Venv           core.CommandSpec
	PipInstallReqs core.CommandSpec
	PipUpgrade     core.CommandSpec // named packages
	PipUpgradeReqs core.CommandSpec
	LockAll        core.CommandSpec // lock, upgrading everything
	LockPackages   core.CommandSpec // lock, upgrading the named packages
	SyncLocked     core.CommandSpec
}

// devGroup is uv's --dev for the group "dev"; any other group is --group name.
var devGroup = core.GroupSpec{Dev: []string{"--dev"}, Named: []string{"--group", core.PlaceholderGroup}}

// namedGroup is uv sync's form: every group, including dev, is --group name.
var namedGroup = core.GroupSpec{Named: []string{"--group", core.PlaceholderGroup}}

var spec = commands{
	Tool: core.ToolSpec{
		Binary:      Binary,
		VersionArgs: []string{"--version"},
		Install: core.InstallHints{
			Darwin: core.InstallHint{Cmd: []string{"brew", "install", "uv"}},
			Linux:  unixInstall,
			Windows: core.InstallHint{
				Cmd:  []string{"powershell", "-ExecutionPolicy", "ByPass", "-c", "irm https://astral.sh/uv/install.ps1 | iex"},
				Text: `powershell -ExecutionPolicy ByPass -c "irm https://astral.sh/uv/install.ps1 | iex"`,
			},
			Other: unixInstall,
		},
	},
	Dialect: core.DialectPEP440,

	Add:            core.CommandSpec{Argv: []string{"add", "{group}", "{script}", "{pkg}{constraint}"}, Group: devGroup},
	Remove:         core.CommandSpec{Argv: []string{"remove", "{group}", "{script}", "{pkg}"}, Group: devGroup},
	Sync:           core.CommandSpec{Argv: []string{"sync", "{group}", "{script}"}, Group: namedGroup},
	Venv:           core.CommandSpec{Argv: []string{"venv"}},
	PipInstallReqs: core.CommandSpec{Argv: []string{"pip", "install", "-r", "requirements.txt"}},
	PipUpgrade:     core.CommandSpec{Argv: []string{"pip", "install", "--upgrade", "{pkg}"}},
	PipUpgradeReqs: core.CommandSpec{Argv: []string{"pip", "install", "--upgrade", "-r", "requirements.txt"}},
	LockAll:        core.CommandSpec{Argv: []string{"lock", "{script}", "--upgrade"}},
	LockPackages:   core.CommandSpec{Argv: []string{"lock", "{script}", "{pkg}"}, PerPackage: []string{"--upgrade-package"}},
	SyncLocked:     core.CommandSpec{Argv: []string{"sync", "{script}"}},
}

var unixInstall = core.InstallHint{
	Cmd:  []string{"sh", "-c", "curl -LsSf https://astral.sh/uv/install.sh | sh"},
	Text: "curl -LsSf https://astral.sh/uv/install.sh | sh",
}
