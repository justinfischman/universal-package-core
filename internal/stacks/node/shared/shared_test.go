package shared

import (
	"errors"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestScriptFileName(t *testing.T) {
	good := map[string]string{"hello": "hello.ts", "tool.ts": "tool.ts", "run.mjs": "run.mjs"}
	for in, want := range good {
		got, err := ScriptFileName(in)
		if err != nil || got != want {
			t.Errorf("ScriptFileName(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", " ", "-x", "../x", "a/b", `a\b`, "x.py", "x.sh"} {
		if _, err := ScriptFileName(in); !errors.Is(err, core.ErrInvalidRequest) {
			t.Errorf("ScriptFileName(%q) err = %v, want ErrInvalidRequest", in, err)
		}
	}
}

func TestPackageNameFor(t *testing.T) {
	cases := map[string]string{"hello.ts": "hello", "My Tool.ts": "my-tool", "a_b.mjs": "a_b", "--x--.js": "x"}
	for in, want := range cases {
		if got, err := PackageNameFor(in); err != nil || got != want {
			t.Errorf("PackageNameFor(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := PackageNameFor("...ts"); !errors.Is(err, core.ErrInvalidRequest) {
		t.Errorf("err = %v, want ErrInvalidRequest", err)
	}
}

func TestValidateProjectName(t *testing.T) {
	for _, in := range []string{"demo", "demo-lib", "a_b1"} {
		if err := ValidateProjectName(in); err != nil {
			t.Errorf("%q: %v", in, err)
		}
	}
	for _, in := range []string{"", "Demo", "1demo", "-x", "a b", "a/b", "@scope/x"} {
		if err := ValidateProjectName(in); !errors.Is(err, core.ErrInvalidRequest) {
			t.Errorf("%q: err = %v, want ErrInvalidRequest", in, err)
		}
	}
}

func TestIsTypeScriptFile(t *testing.T) {
	for in, want := range map[string]bool{"a.ts": true, "a.mts": true, "a.cts": true, "a.js": false, "a.mjs": false, "a": false} {
		if IsTypeScriptFile(in) != want {
			t.Errorf("IsTypeScriptFile(%q) = %v", in, !want)
		}
	}
}
