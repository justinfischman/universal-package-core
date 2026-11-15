package core

import (
	"errors"
	"strings"
	"testing"
)

func TestParseConstraintRoundTrip(t *testing.T) {
	tests := map[string]string{
		">=2.3.4":        ">=2.3.4",
		" >= 2.3.4 ":     ">=2.3.4",
		">=1.2 , <2":     ">=1.2,<2",
		"2.3.4":          "==2.3.4",
		"!=1.0":          "!=1.0",
		"~=1.4.2":        "~=1.4.2",
		"^1.2.3":         "^1.2.3",
		"~1.2":           "~1.2",
		"<=3":            "<=3",
		">1.0.0rc1":      ">1.0.0rc1",
		"==2.0.0-beta.1": "==2.0.0-beta.1",
		">=1.0,!=1.5,<2": ">=1.0,!=1.5,<2",
	}
	for in, want := range tests {
		c, err := ParseConstraint(in)
		if err != nil {
			t.Errorf("ParseConstraint(%q): %v", in, err)
			continue
		}
		if got := c.String(); got != want {
			t.Errorf("ParseConstraint(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseConstraintRejects(t *testing.T) {
	for _, in := range []string{"", "  ", ">=", ">=,<2", "abc", ">=v1", ">=1.0 <2", "=>1", ">=1;rm", "$(x)", ">=1,", "=== 1"} {
		if _, err := ParseConstraint(in); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("ParseConstraint(%q) error = %v, want ErrInvalidRequest", in, err)
		}
	}
}

func TestRenderPEP440(t *testing.T) {
	tests := []struct{ in, want string }{
		{">=2.3.4", ">=2.3.4"},
		{">1,<=3", ">1,<=3"},
		{"2.3.4", "==2.3.4"},
		{"!=1.5", "!=1.5"},
		{"~=1.4", "~=1.4"},
		{"^1.2.3", ">=1.2.3,<2.0.0"},
		{"^0.2.3", ">=0.2.3,<0.3.0"},
		{"^0.0.3", ">=0.0.3,<0.0.4"},
		{"^0", ">=0,<1.0.0"},
		{"^1.2", ">=1.2,<2.0.0"},
		{"~1.2.3", ">=1.2.3,<1.3.0"},
		{"~1.2", ">=1.2,<1.3.0"},
		{"~1", ">=1,<2.0.0"},
		{"^1.2.3,!=1.4.0", ">=1.2.3,<2.0.0,!=1.4.0"},
		{">=1.0.0rc1", ">=1.0.0rc1"},
	}
	for _, tt := range tests {
		c, err := ParseConstraint(tt.in)
		if err != nil {
			t.Fatalf("parse %q: %v", tt.in, err)
		}
		got, err := c.Render(DialectPEP440, "mgr")
		if err != nil || got != tt.want {
			t.Errorf("Render(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
		}
	}
}

// Anything that cannot be said exactly fails, naming manager and constraint.
func TestRenderPEP440Untranslatable(t *testing.T) {
	for _, in := range []string{"^1.2.3rc1", "~1.2.3.4", "~=1", "^1.x"} {
		c, err := ParseConstraint(in)
		if err != nil {
			continue // rejected earlier, also clearly
		}
		_, err = c.Render(DialectPEP440, "mgr")
		if !errors.Is(err, ErrInvalidRequest) || !strings.Contains(err.Error(), "mgr") || !strings.Contains(err.Error(), in) {
			t.Errorf("Render(%q) error = %v, want ErrInvalidRequest naming the manager and the constraint", in, err)
		}
	}
	if _, err := (Constraint{Clauses: []Clause{{Op: ">=", Version: "1"}}}).Render("other", "mgr"); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("unknown dialect error = %v", err)
	}
}
