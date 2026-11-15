package workspace

import "strconv"

// App is one named member project of a workspace: where it lives, the port and
// URL it serves on, what it offers and which apps it needs (ADR-0014).
type App struct {
	Name string
	// Path is the directory relative to the workspace root; Dir is absolute.
	Path     string
	Dir      string
	Port     int
	URL      string // raw; may hold ${apps...} references
	Provides []string
	Consumes []string
	Contract string
	Env      map[string]string // raw; values may hold ${apps...} references
}

// Field is the raw value of a field that ${apps.<name>.<field>} can name. It
// reports false for an unknown or empty field.
func (a *App) Field(field string) (string, bool) {
	var v string
	switch field {
	case "name":
		v = a.Name
	case "path":
		v = a.Path
	case "port":
		if a.Port > 0 {
			v = strconv.Itoa(a.Port)
		}
	case "url":
		v = a.URL
	}
	return v, v != ""
}

// Provider reports whether the app offers anything to its consumers.
func (a *App) Provider() bool { return len(a.Provides) > 0 }
