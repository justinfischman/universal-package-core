package openapi

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/core"
)

func snap(t *testing.T, name, content string) []byte {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := Checker{}.Snapshot(p)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	return b
}

const base = `{"openapi":"3.0.0","info":{"title":"t"},"paths":{"/a":{"get":{"summary":"x"}},"/b":{"get":{}}}}`

func TestSnapshotNormalizesYAMLAndJSON(t *testing.T) {
	yaml := "openapi: 3.0.0\ninfo:\n  title: t\npaths:\n  /b:\n    get: {}\n  /a:\n    get:\n      summary: x\n"
	if a, b := snap(t, "a.json", base), snap(t, "a.yaml", yaml); string(a) != string(b) {
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
		{"path added", `{"openapi":"3.0.0","info":{"title":"t"},"paths":{"/a":{"get":{"summary":"x"}},"/b":{"get":{}},"/c":{}}}`, []string{"+ path /c"}},
		{"path removed", `{"openapi":"3.0.0","info":{"title":"t"},"paths":{"/a":{"get":{"summary":"x"}}}}`, []string{"- path /b"}},
		{"operation added", `{"openapi":"3.0.0","info":{"title":"t"},"paths":{"/a":{"get":{"summary":"x"},"post":{}},"/b":{"get":{}}}}`, []string{"+ POST /a"}},
		{"operation removed", `{"openapi":"3.0.0","info":{"title":"t"},"paths":{"/a":{},"/b":{"get":{}}}}`, []string{"- GET /a"}},
		{"operation changed", `{"openapi":"3.0.0","info":{"title":"t"},"paths":{"/a":{"get":{"summary":"y"}},"/b":{"get":{}}}}`, []string{"~ GET /a"}},
		{"other section changed", `{"openapi":"3.0.0","info":{"title":"u"},"paths":{"/a":{"get":{"summary":"x"}},"/b":{"get":{}}}}`, []string{"~ info"}},
	}
	old := snap(t, "o.json", base)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Checker{}.Diff(old, snap(t, "n.json", tc.next))
			if want := strings.Join(tc.want, "\n"); got != want {
				t.Fatalf("Diff = %q; want %q", got, want)
			}
		})
	}
}

func TestSnapshotRejectsNonOpenAPI(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.json")
	if err := os.WriteFile(p, []byte(`{"type":"object"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (Checker{}).Snapshot(p); !errors.Is(err, core.ErrInvalidRequest) {
		t.Fatalf("got %v; want ErrInvalidRequest", err)
	}
	if _, err := (Checker{}).Snapshot(filepath.Join(t.TempDir(), "missing.json")); !errors.Is(err, core.ErrInvalidRequest) {
		t.Fatalf("missing file: got %v; want ErrInvalidRequest", err)
	}
}
