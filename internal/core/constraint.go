package core

import (
	"fmt"
	"regexp"
	"strings"
)

// Op is a version-constraint operator in upc's neutral notation.
type Op string

// The operators a neutral constraint can use.
const (
	OpGE     Op = ">="
	OpGT     Op = ">"
	OpLE     Op = "<="
	OpLT     Op = "<"
	OpEQ     Op = "=="
	OpNE     Op = "!="
	OpCompat Op = "~=" // PEP 440 compatible release
	OpTilde  Op = "~"  // semver tilde: patch-level changes (minor when only a major is given)
	OpCaret  Op = "^"  // semver caret: changes that keep the left-most non-zero part
)

// operators lists the operators longest first so "<=" is not read as "<".
var operators = []Op{OpGE, OpLE, OpEQ, OpNE, OpCompat, OpGT, OpLT, OpTilde, OpCaret}

// versionText accepts release numbers with optional pre-release or local
// suffixes (1.0.0rc1, 2.0.0-beta.1, 1.0+local). It must start with a digit.
var versionText = regexp.MustCompile(`^[0-9][0-9A-Za-z.+!_-]*$`)

// Clause is one operator applied to one version.
type Clause struct {
	Op      Op
	Version string
}

// Constraint is a neutral, manager-independent version constraint: one or more
// clauses that must all hold (">=1.2,<2"). Managers render it into their own
// syntax with Render.
type Constraint struct {
	Clauses []Clause
}

// ParseConstraint reads a neutral constraint. Clauses are separated by commas;
// whitespace around operators and commas is ignored. A bare version means "==".
func ParseConstraint(s string) (Constraint, error) {
	if strings.TrimSpace(s) == "" {
		return Constraint{}, fmt.Errorf("%w: version constraint is empty", ErrInvalidRequest)
	}
	var c Constraint
	for _, part := range strings.Split(s, ",") {
		cl, err := parseClause(strings.TrimSpace(part))
		if err != nil {
			return Constraint{}, fmt.Errorf("%w: version constraint %q: %v", ErrInvalidRequest, s, err)
		}
		c.Clauses = append(c.Clauses, cl)
	}
	return c, nil
}

func parseClause(text string) (Clause, error) {
	if text == "" {
		return Clause{}, fmt.Errorf("empty clause")
	}
	op := OpEQ
	for _, candidate := range operators {
		if strings.HasPrefix(text, string(candidate)) {
			op, text = candidate, strings.TrimSpace(strings.TrimPrefix(text, string(candidate)))
			break
		}
	}
	if !versionText.MatchString(text) {
		return Clause{}, fmt.Errorf("%q is not a version", text)
	}
	return Clause{Op: op, Version: text}, nil
}

// String is the neutral form, which ParseConstraint reads back.
func (c Constraint) String() string {
	parts := make([]string, len(c.Clauses))
	for i, cl := range c.Clauses {
		parts[i] = string(cl.Op) + cl.Version
	}
	return strings.Join(parts, ",")
}
