package cli

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/prompt"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/core/coretest"
)

// rankedPM is a fake manager that ranks its own match.
type rankedPM struct {
	*coretest.PackageManager
	rank int
}

func (r rankedPM) DetectRank(fsys fs.FS) int {
	if r.Detect(fsys) {
		return r.rank
	}
	return 0
}

// chooser answers every Choose with a fixed option, or fails when it is empty.
type chooser struct{ answer string }

func (c chooser) Confirm(string, bool) (bool, error) { return true, nil }
func (c chooser) Choose(_ string, options []string) (string, error) {
	for _, o := range options {
		if o == c.answer {
			return o, nil
		}
	}
	return "", core.ErrInvalidRequest
}

func (h *harness) addRanked(t *testing.T, id string, rank int) {
	t.Helper()
	pm, err := coretest.NewPackageManager(id, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.deps.Registries.PackageManagers.Register(rankedPM{PackageManager: pm, rank: rank}); err != nil {
		t.Fatal(err)
	}
}

func (h *harness) mark(t *testing.T, ids ...string) {
	t.Helper()
	for _, id := range ids {
		if err := os.WriteFile(filepath.Join(h.project, id+".marker"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// ranked is called with the rank each marker earns; unranked fakes count as 0.
func TestDetectionPriority(t *testing.T) {
	cases := []struct {
		name    string
		ranked  map[string]int // extra ranked managers to register
		markers []string
		want    string // chosen manager, or "" when detection must fail
	}{
		{"single match", nil, []string{"alpha-one"}, "alpha-one"},
		{"lockfile-like rank beats manifest-like", map[string]int{"alpha-lock": 300, "alpha-req": 100}, []string{"alpha-lock", "alpha-req"}, "alpha-lock"},
		{"ranked beats unranked", map[string]int{"alpha-lock": 300}, []string{"alpha-one", "alpha-lock"}, "alpha-lock"},
		{"equal ranks are ambiguous", map[string]int{"alpha-a": 100, "alpha-b": 100}, []string{"alpha-a", "alpha-b"}, ""},
		{"unranked tie is ambiguous", nil, []string{"alpha-one", "alpha-two"}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			for id, rank := range c.ranked {
				h.addRanked(t, id, rank)
			}
			if err := os.Remove(filepath.Join(h.project, "alpha-one.marker")); err != nil {
				t.Fatal(err)
			}
			h.mark(t, c.markers...)
			code := h.run("add", "foo")
			if c.want == "" {
				if code != core.ExitUsage {
					t.Fatalf("exit %d, want usage error: %s", code, h.stderr)
				}
				for _, id := range c.markers {
					if !strings.Contains(h.stderr.String(), id) {
						t.Errorf("error should list %q: %s", id, h.stderr)
					}
				}
				if !strings.Contains(h.stderr.String(), "upc config set package_manager") {
					t.Errorf("error should say how to choose: %s", h.stderr)
				}
				return
			}
			if code != core.ExitOK {
				t.Fatalf("exit %d: %s", code, h.stderr)
			}
			if !strings.Contains(h.stderr.String(), "package manager detected") || !strings.Contains(h.stderr.String(), c.want) {
				t.Errorf("decision must be logged: %s", h.stderr)
			}
		})
	}
}

func TestAmbiguousDetectionAsksWhenInteractive(t *testing.T) {
	h := newHarness(t)
	h.mark(t, "alpha-two")
	h.deps.NewPrompter = func(bool, *os.File, io.Writer) prompt.Prompter { return chooser{answer: "alpha-two"} }
	if code := h.run("add", "foo"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if len(h.pm(t, "alpha-two").Calls) != 1 || len(h.pm(t, "alpha-one").Calls) != 0 {
		t.Error("the answer must select alpha-two")
	}
	if !strings.Contains(h.stderr.String(), "package manager chosen") {
		t.Errorf("choice must be logged: %s", h.stderr)
	}
}

func TestAmbiguousDetectionWithYesDoesNotGuess(t *testing.T) {
	h := newHarness(t)
	h.mark(t, "alpha-two")
	h.deps.NewPrompter = func(bool, *os.File, io.Writer) prompt.Prompter { return chooser{answer: "alpha-two"} }
	if code := h.run("--yes", "add", "foo"); code != core.ExitUsage {
		t.Errorf("exit %d: %s", code, h.stderr)
	}
}
