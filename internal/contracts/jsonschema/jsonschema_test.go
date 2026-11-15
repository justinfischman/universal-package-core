package jsonschema

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/core"
)

func snap(t *testing.T, content string) []byte {
	t.Helper()
	p := filepath.Join(t.TempDir(), "s.json")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := Checker{}.Snapshot(p)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	return b
}

const base = `{"type":"object","required":["id"],"properties":{"id":{"type":"string"},"addr":{"type":"object","properties":{"zip":{"type":"string"}}}}}`

func TestSnapshotKeyOrderIndependent(t *testing.T) {
	a := snap(t, `{"type":"object","properties":{"b":{},"a":{}}}`)
	b := snap(t, `{"properties":{"a":{},"b":{}},"type":"object"}`)
	if string(a) != string(b) {
		t.Fatalf("snapshots differ:\n%s\n%s", a, b)
	}
}

func TestDiff(t *testing.T) {
	tests := []struct {
		name string
		next string
		want []string
	}{
		{"identical", base, nil},
		{"property added", `{"type":"object","required":["id"],"properties":{"id":{"type":"string"},"name":{},"addr":{"type":"object","properties":{"zip":{"type":"string"}}}}}`, []string{"+ property name"}},
		{"property removed", `{"type":"object","required":["id"],"properties":{"addr":{"type":"object","properties":{"zip":{"type":"string"}}}}}`, []string{"- property id"}},
		{"property type changed", `{"type":"object","required":["id"],"properties":{"id":{"type":"integer"},"addr":{"type":"object","properties":{"zip":{"type":"string"}}}}}`, []string{"~ property id", "~ keywords id"}},
		{"nested property added", `{"type":"object","required":["id"],"properties":{"id":{"type":"string"},"addr":{"type":"object","properties":{"zip":{"type":"string"},"city":{}}}}}`, []string{"~ property addr", "+ property addr.city"}},
		{"required added", `{"type":"object","required":["id","addr"],"properties":{"id":{"type":"string"},"addr":{"type":"object","properties":{"zip":{"type":"string"}}}}}`, []string{"+ required (root): addr"}},
		{"required removed", `{"type":"object","properties":{"id":{"type":"string"},"addr":{"type":"object","properties":{"zip":{"type":"string"}}}}}`, []string{"- required (root): id"}},
	}
	old := snap(t, base)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Checker{}.Diff(old, snap(t, tc.next))
			if want := strings.Join(tc.want, "\n"); got != want {
				t.Fatalf("Diff = %q; want %q", got, want)
			}
		})
	}
}

func TestSnapshotRejectsNonSchema(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.json")
	if err := os.WriteFile(p, []byte(`{"name":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (Checker{}).Snapshot(p); !errors.Is(err, core.ErrInvalidRequest) {
		t.Fatalf("got %v; want ErrInvalidRequest", err)
	}
}
