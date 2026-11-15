package shared

import "sort"

// AppEngine is the first Node release that runs the app's TypeScript with the
// strip flag (ADR-0010 decision 2): `dev` runs src/main.ts without a build.
const AppEngine = ScriptEngine

// AppScripts are the package.json scripts of a new app: dev runs the source
// through Node's type stripping, build compiles with tsc and start runs the
// compiled output.
var AppScripts = map[string]string{
	"dev":   RuntimeBinary + " " + StripTypesFlag + " src/main.ts",
	"build": "tsc",
	"start": RuntimeBinary + " dist/main.js",
}

// AppScriptNames is AppScripts' keys in a stable order.
func AppScriptNames() []string {
	names := make([]string, 0, len(AppScripts))
	for name := range AppScripts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// AppDevDependencies are installed as development dependencies of a new app so
// `build` works at once.
var AppDevDependencies = []string{"typescript", "@types/node"}
