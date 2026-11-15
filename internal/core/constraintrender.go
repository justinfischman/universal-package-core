package core

import (
	"fmt"
	"strconv"
	"strings"
)

// Dialect is the constraint syntax a manager understands. Each dialect states
// which operators it can express; Render fails rather than change a meaning.
type Dialect string

// DialectPEP440 is the PEP 440 specifier syntax ("pkg>=1.2,<2").
const DialectPEP440 Dialect = "pep440"

// Render writes the constraint in the dialect's syntax, without the package
// name or any separator (the command template supplies those). manager names
// the caller in error messages. A clause the dialect cannot express exactly
// returns ErrInvalidRequest naming the constraint, the manager and, when one
// exists, an equivalent the user can pass instead.
func (c Constraint) Render(d Dialect, manager string) (string, error) {
	if d != DialectPEP440 {
		return "", fmt.Errorf("%w: %s: unknown constraint dialect %q", ErrInvalidRequest, manager, d)
	}
	var out []string
	for _, cl := range c.Clauses {
		parts, err := renderPEP440(cl)
		if err != nil {
			return "", fmt.Errorf("%w: %s cannot express constraint %q exactly: %v", ErrInvalidRequest, manager, c.String(), err)
		}
		out = append(out, parts...)
	}
	return strings.Join(out, ","), nil
}

func renderPEP440(cl Clause) ([]string, error) {
	switch cl.Op {
	case OpGE, OpGT, OpLE, OpLT, OpEQ, OpNE:
		return []string{string(cl.Op) + cl.Version}, nil
	case OpCompat:
		if !strings.Contains(cl.Version, ".") {
			return nil, fmt.Errorf("~= needs at least two release segments; use >=%s,<%s", cl.Version, bumpedFirst(cl.Version))
		}
		return []string{"~=" + cl.Version}, nil
	case OpCaret, OpTilde:
		return rangeFor(cl)
	}
	return nil, fmt.Errorf("unsupported operator %q", cl.Op)
}

// rangeFor expands ^ and ~ into the exact PEP 440 range they mean, for plain
// numeric releases of one to three segments; anything else cannot be expanded
// without guessing.
func rangeFor(cl Clause) ([]string, error) {
	nums, ok := releaseParts(cl.Version)
	if !ok {
		return nil, fmt.Errorf("%s needs a plain numeric version of up to three parts; use an explicit range", cl.Op)
	}
	bump := bumpIndex(cl.Op, nums)
	upper := make([]string, 3)
	for i := range upper {
		switch {
		case i < bump:
			upper[i] = strconv.Itoa(nums[i])
		case i == bump:
			upper[i] = strconv.Itoa(nums[i] + 1)
		default:
			upper[i] = "0"
		}
	}
	return []string{">=" + cl.Version, "<" + strings.Join(upper, ".")}, nil
}

// bumpIndex is the release part that the upper bound increments.
func bumpIndex(op Op, nums []int) int {
	if op == OpTilde {
		if len(nums) == 1 {
			return 0
		}
		return 1
	}
	for i, n := range nums {
		if n != 0 {
			return i
		}
	}
	return len(nums) - 1 // ^0.0.3 -> <0.0.4, ^0 -> <1
}

func releaseParts(v string) ([]int, bool) {
	segs := strings.Split(v, ".")
	if len(segs) > 3 {
		return nil, false
	}
	nums := make([]int, len(segs))
	for i, s := range segs {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			return nil, false
		}
		nums[i] = n
	}
	return nums, true
}

// bumpedFirst is the version with its first numeric part incremented, for
// suggestions only; it falls back to the input when it is not numeric.
func bumpedFirst(v string) string {
	first, _, _ := strings.Cut(v, ".")
	n, err := strconv.Atoi(first)
	if err != nil {
		return v
	}
	return strconv.Itoa(n + 1)
}
