// Package prompt asks the user questions without ever blocking a
// non-interactive run: with --yes it auto-confirms, otherwise it fails with a
// message naming the flag that would answer it.
package prompt

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// Prompter asks confirm and choose questions.
type Prompter interface {
	Confirm(msg string, def bool) (bool, error)
	Choose(msg string, options []string) (string, error)
}

// New picks the implementation: --yes or a non-terminal input means
// NonInteractive, otherwise Terminal.
func New(yes bool, in *os.File, out io.Writer) Prompter {
	if yes || in == nil || !term.IsTerminal(int(in.Fd())) {
		return NonInteractive{Yes: yes}
	}
	return Terminal{In: in, Out: out}
}

// NonInteractive never reads input.
type NonInteractive struct {
	Yes bool
}

// Confirm returns true when --yes was given, otherwise an error naming the flag.
func (n NonInteractive) Confirm(msg string, _ bool) (bool, error) {
	if n.Yes {
		return true, nil
	}
	return false, fmt.Errorf("%w: %q needs confirmation; run interactively or pass --yes", core.ErrInvalidRequest, msg)
}

// Choose cannot guess a selection, so it always errors.
func (n NonInteractive) Choose(msg string, _ []string) (string, error) {
	return "", fmt.Errorf("%w: %q needs a choice; run interactively or pass the value explicitly", core.ErrInvalidRequest, msg)
}

// Terminal reads answers from In and writes questions to Out.
type Terminal struct {
	In  io.Reader
	Out io.Writer

	reader *bufio.Reader
}

func (t Terminal) printf(format string, a ...any) error {
	_, err := fmt.Fprintf(t.Out, format, a...)
	return err
}

func (t Terminal) readLine() (string, error) {
	r := t.reader
	if r == nil {
		r = bufio.NewReader(t.In)
	}
	line, err := r.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// Confirm asks a yes/no question; an empty answer returns def.
func (t Terminal) Confirm(msg string, def bool) (bool, error) {
	hint := "[y/N]"
	if def {
		hint = "[Y/n]"
	}
	if err := t.printf("%s %s ", msg, hint); err != nil {
		return false, err
	}
	ans, err := t.readLine()
	if err != nil {
		return false, err
	}
	switch strings.ToLower(ans) {
	case "":
		return def, nil
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	}
	return false, fmt.Errorf("%w: answer %q is not yes or no", core.ErrInvalidRequest, ans)
}

// Choose lists options and returns the one picked by number.
func (t Terminal) Choose(msg string, options []string) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("%w: no options for %q", core.ErrInvalidRequest, msg)
	}
	menu := msg + "\n"
	for i, o := range options {
		menu += fmt.Sprintf("  %d) %s\n", i+1, o)
	}
	if err := t.printf("%schoice: ", menu); err != nil {
		return "", err
	}
	ans, err := t.readLine()
	if err != nil {
		return "", err
	}
	n, convErr := strconv.Atoi(ans)
	if convErr != nil || n < 1 || n > len(options) {
		return "", fmt.Errorf("%w: choice %q is not between 1 and %d", core.ErrInvalidRequest, ans, len(options))
	}
	return options[n-1], nil
}
