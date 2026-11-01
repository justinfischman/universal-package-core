// Package scripts is the language-neutral lookup of a named script to run
// (ADR-0008). A package manager supplies a Reader for its own manifest; the
// lookup order and command-line splitting live here once.
package scripts

import (
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// Table is the named scripts a manifest declares.
type Table struct {
	// Commands are command lines run inside the project environment
	// (for example [tool.upc.scripts]).
	Commands map[string]string
	// EntryPoints are installed executables (for example [project.scripts]).
	EntryPoints map[string]string
}

// Names returns every declared script name, sorted and de-duplicated.
func (t Table) Names() []string {
	seen := map[string]bool{}
	var names []string
	for _, m := range []map[string]string{t.Commands, t.EntryPoints} {
		for n := range m {
			if !seen[n] {
				seen[n] = true
				names = append(names, n)
			}
		}
	}
	sort.Strings(names)
	return names
}

// Reader reads the script table from a project directory. A missing manifest
// is reported as an error wrapping fs.ErrNotExist and means "no scripts".
type Reader interface {
	ReadScripts(fsys fs.FS) (Table, error)
}

// Kind is how a resolved script is run.
type Kind string

// The lookup kinds, in lookup order.
const (
	KindFile       Kind = "file"        // a file in the project directory
	KindCommand    Kind = "command"     // a command line from the table
	KindEntryPoint Kind = "entry-point" // an installed executable
)

// Target is what a script name resolved to.
type Target struct {
	Kind Kind
	Name string
	// Words is the split command line, set for KindCommand only.
	Words []string
}

// Resolve looks name up in a fixed order: a file in the project directory,
// then a command in the table, then an entry point.
func Resolve(fsys fs.FS, name string, r Reader) (Target, error) {
	if strings.TrimSpace(name) == "" || strings.HasPrefix(name, "-") {
		return Target{}, fmt.Errorf("%w: %q is not a valid script name or file", core.ErrInvalidRequest, name)
	}
	if info, err := fs.Stat(fsys, name); err == nil && !info.IsDir() && !strings.Contains(name, "..") {
		return Target{Kind: KindFile, Name: name}, nil
	}
	table, err := r.ReadScripts(fsys)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Target{}, err
	}
	if line, ok := table.Commands[name]; ok {
		words, err := Split(line)
		if err != nil {
			return Target{}, fmt.Errorf("%w: script %q: %v", core.ErrInvalidRequest, name, err)
		}
		return Target{Kind: KindCommand, Name: name, Words: words}, nil
	}
	if _, ok := table.EntryPoints[name]; ok {
		return Target{Kind: KindEntryPoint, Name: name}, nil
	}
	avail := ""
	if names := table.Names(); len(names) > 0 {
		avail = " (available: " + strings.Join(names, ", ") + ")"
	}
	return Target{}, fmt.Errorf("%w: %q is not a file or a declared script%s", core.ErrInvalidRequest, name, avail)
}

// Split splits a command line into words, honoring single and double quotes.
// It does no expansion: the words go straight to the program.
func Split(line string) ([]string, error) {
	var words []string
	var cur strings.Builder
	var quote rune
	inWord := false
	for _, r := range line {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote, inWord = r, true
		case r == ' ' || r == '\t':
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated %c quote", quote)
	}
	if inWord {
		words = append(words, cur.String())
	}
	if len(words) == 0 {
		return nil, errors.New("empty command")
	}
	return words, nil
}
