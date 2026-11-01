// Package output prints results: data on the writer it is given (stdout),
// never mixed with logs (stderr).
package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// Printer prints a Result.
type Printer interface {
	Print(core.Result) error
}

// New returns a JSON printer when asJSON is set, a text printer otherwise.
func New(asJSON bool, w io.Writer) Printer {
	if asJSON {
		return jsonPrinter{w: w}
	}
	return textPrinter{w: w}
}

type textPrinter struct{ w io.Writer }

func (p textPrinter) Print(r core.Result) error {
	if r.Summary != "" {
		if _, err := fmt.Fprintln(p.w, r.Summary); err != nil {
			return err
		}
	}
	for _, c := range r.Commands {
		if _, err := fmt.Fprintf(p.w, "$ %s\n", c); err != nil {
			return err
		}
	}
	return nil
}

type jsonPrinter struct{ w io.Writer }

func (p jsonPrinter) Print(r core.Result) error {
	enc := json.NewEncoder(p.w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
