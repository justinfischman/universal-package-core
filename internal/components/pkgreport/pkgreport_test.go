package pkgreport

import (
	"testing"

	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestOutdated(t *testing.T) {
	stale, summary := Outdated([]core.Package{
		{Name: "a", Version: "1", Latest: "2"}, {Name: "b", Version: "1", Latest: "1"}, {Name: "c", Version: "1"},
	})
	if len(stale) != 1 || summary != "a 1 -> 2" {
		t.Errorf("stale = %v, summary = %q", stale, summary)
	}
	if got, _ := Outdated(nil); got == nil {
		t.Error("stale must be an empty list, not nil, for JSON output")
	}
}

func TestSummary(t *testing.T) {
	got := Summary([]core.Package{{Name: "a", Version: "1"}, {Name: "b"}})
	if got != "a 1\nb" {
		t.Errorf("Summary = %q", got)
	}
}
