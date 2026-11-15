package contracts

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		name, file, content, want string
	}{
		{"openapi yaml", "a.yaml", "openapi: 3.0.0\npaths: {}\n", "openapi"},
		{"swagger json", "a.json", `{"swagger":"2.0"}`, "openapi"},
		{"json schema", "s.json", `{"type":"object"}`, "jsonschema"},
		{"json schema yaml", "s.yaml", "$schema: http://json-schema.org/draft-07/schema#\nproperties: {}\n", "jsonschema"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), tc.file)
			if err := os.WriteFile(p, []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}
			c, err := Detect(p)
			if err != nil || c.Name() != tc.want {
				t.Fatalf("Detect = %v, %v; want %s", c, err, tc.want)
			}
			if byName, ok := ByName(tc.want); !ok || byName.Name() != tc.want {
				t.Fatalf("ByName(%q) failed", tc.want)
			}
		})
	}
	p := filepath.Join(t.TempDir(), "x.json")
	if err := os.WriteFile(p, []byte(`{"name":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Detect(p); !errors.Is(err, core.ErrInvalidRequest) {
		t.Fatalf("unknown type: %v", err)
	}
}
