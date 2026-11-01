package shared

import "strings"

// Runtime facts every Node package manager shares (ADR-0010).
const (
	// RuntimeBinary runs script files; the package managers ship with or sit on it.
	RuntimeBinary = "node"
	// StripTypesFlag lets the runtime execute TypeScript by erasing its types.
	StripTypesFlag = "--experimental-strip-types"
	// ScriptEngine and PackageEngine are the first Node releases that run
	// TypeScript with the strip flag (script) and discover .ts tests by default
	// (package).
	ScriptEngine  = ">=22.6.0"
	PackageEngine = ">=22.18.0"
)

// ScriptSource is the TypeScript written for a new script.
const ScriptSource = `const name: string = process.argv[2] ?? "world";
console.log("Hello, " + name + "!");
`

// FileArgs is the node command line for a script file.
func FileArgs(file string, extra []string) []string {
	if IsTypeScriptFile(file) {
		return Concat([]string{StripTypesFlag, file}, extra)
	}
	return Concat([]string{file}, extra)
}

// StartCommand is the package.json `start` script for a script file.
func StartCommand(file string) string {
	return RuntimeBinary + " " + strings.Join(FileArgs(file, nil), " ")
}
