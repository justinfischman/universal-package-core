package uv

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// Each case is one add with --version: the neutral constraint must reach uv as
// exactly one argv element, never a shell string.
func TestAddVersionRendersOneArgvElement(t *testing.T) {
	tests := []struct {
		name    string
		version string
		pkg     string
		group   string
		want    []string
	}{
		{"ge", ">=2.3.4", "requests", "", []string{"add", "requests>=2.3.4"}},
		{"gt and lt need no quoting", ">1,<2", "requests", "", []string{"add", "requests>1,<2"}},
		{"spaces are normalised away", " >= 2.3.4 , < 3 ", "requests", "", []string{"add", "requests>=2.3.4,<3"}},
		{"bare version pins", "2.3.4", "requests", "", []string{"add", "requests==2.3.4"}},
		{"not equal", "!=1.5", "requests", "", []string{"add", "requests!=1.5"}},
		{"compatible release", "~=1.4.2", "requests", "", []string{"add", "requests~=1.4.2"}},
		{"caret becomes a range", "^1.2.3", "requests", "", []string{"add", "requests>=1.2.3,<2.0.0"}},
		{"tilde becomes a range", "~1.2.3", "requests", "", []string{"add", "requests>=1.2.3,<1.3.0"}},
		{"pre-release", ">=1.0.0rc1", "requests", "", []string{"add", "requests>=1.0.0rc1"}},
		{"extras stay in the package", ">=2", "requests[security]", "", []string{"add", "requests[security]>=2"}},
		{"with dev group", ">=2", "pytest", "dev", []string{"add", "--dev", "pytest>=2"}},
		{"with named group", ">=2", "sphinx", "docs", []string{"add", "--group", "docs", "sphinx>=2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := fixture(t, projectFiles)
			fake := &execx.FakeRunner{}
			_, err := call(t, core.CapAdd, core.Request{Dir: dir, Packages: []string{tt.pkg}, Version: tt.version, Group: tt.group}, fake)
			if err != nil {
				t.Fatal(err)
			}
			if len(fake.Calls) != 1 || fake.Calls[0].Name != "uv" || !reflect.DeepEqual(fake.Calls[0].Args, tt.want) {
				t.Errorf("calls = %+v, want uv %q", fake.Calls, tt.want)
			}
		})
	}
}

func TestAddVersionUntranslatableFailsBeforeRunning(t *testing.T) {
	for _, version := range []string{"^1.2.3rc1", "~=1", "~1.2.3.4", "nonsense", ">=1 <2"} {
		fake := &execx.FakeRunner{}
		_, err := call(t, core.CapAdd, core.Request{Dir: fixture(t, projectFiles), Packages: []string{"requests"}, Version: version}, fake)
		if !errors.Is(err, core.ErrInvalidRequest) {
			t.Errorf("%q: error = %v, want ErrInvalidRequest", version, err)
		}
		if len(fake.Calls) != 0 {
			t.Errorf("%q: ran %v", version, fake.Calls)
		}
	}
	_, err := call(t, core.CapAdd, core.Request{Dir: fixture(t, projectFiles), Packages: []string{"r"}, Version: "~=1"}, &execx.FakeRunner{})
	if err == nil || !strings.Contains(err.Error(), "uv") || !strings.Contains(err.Error(), "~=1") {
		t.Errorf("error %v must name uv and the constraint", err)
	}
}

func TestAddVersionNeedsExactlyOneUnversionedPackage(t *testing.T) {
	for _, pkgs := range [][]string{{"a", "b"}, {"a>=1"}, {"a@1.0"}} {
		_, err := call(t, core.CapAdd, core.Request{Dir: fixture(t, projectFiles), Packages: pkgs, Version: ">=2"}, &execx.FakeRunner{})
		if !errors.Is(err, core.ErrInvalidRequest) {
			t.Errorf("%v: error = %v, want ErrInvalidRequest", pkgs, err)
		}
	}
}

func TestRemoveRejectsVersion(t *testing.T) {
	_, err := call(t, core.CapRemove, core.Request{Dir: fixture(t, projectFiles), Packages: []string{"a"}, Version: ">=2"}, &execx.FakeRunner{})
	if !errors.Is(err, core.ErrInvalidRequest) {
		t.Errorf("error = %v, want ErrInvalidRequest", err)
	}
}

func TestUVIsVersioned(t *testing.T) {
	var m core.PackageManager = newUV(t)
	v, ok := m.(core.Versioned)
	if !ok || v.ConstraintDialect() != core.DialectPEP440 {
		t.Errorf("uv must implement core.Versioned with the PEP 440 dialect")
	}
}
