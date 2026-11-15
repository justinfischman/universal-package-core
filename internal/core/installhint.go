package core

import "strings"

// InstallHint is how to install a tool on one platform: the argv upc runs when
// the user agrees, and the text shown to the user (which may quote differently
// than a plain join, for example PowerShell one-liners).
type InstallHint struct {
	Cmd  []string
	Text string
}

// InstallHints holds one hint per platform; Other covers every other GOOS.
type InstallHints struct {
	Darwin, Linux, Windows, Other InstallHint
}

// For selects the hint for a GOOS value.
func (h InstallHints) For(goos string) InstallHint {
	switch goos {
	case "darwin":
		return h.Darwin
	case "linux":
		return h.Linux
	case "windows":
		return h.Windows
	}
	return h.Other
}

// ToolSpec is the data that describes a manager's executable.
type ToolSpec struct {
	Binary      string
	VersionArgs []string
	Install     InstallHints
}

// Tool resolves the spec for a platform. Text defaults to the joined Cmd.
func (s ToolSpec) Tool(goos string) Tool {
	h := s.Install.For(goos)
	text := h.Text
	if text == "" {
		text = strings.Join(h.Cmd, " ")
	}
	return Tool{Binary: s.Binary, VersionArgs: s.VersionArgs, InstallHint: text, InstallCmd: h.Cmd}
}
