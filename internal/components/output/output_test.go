package output_test

import (
	"bytes"
	"flag"
	"os"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/output"
	"github.com/justinfischman/universal-package-core/internal/core"
)

var update = flag.Bool("update", false, "rewrite golden files")

func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := "testdata/" + name
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != string(got) {
		t.Errorf("%s mismatch\nwant:\n%s\ngot:\n%s", name, want, got)
	}
}

var sample = core.Result{
	Summary:  "added requests",
	Data:     map[string]any{"package": "requests"},
	Commands: []string{"alpha add requests"},
}

func TestJSONGolden(t *testing.T) {
	var buf bytes.Buffer
	if err := output.New(true, &buf).Print(sample); err != nil {
		t.Fatal(err)
	}
	golden(t, "result.json", buf.Bytes())
}

func TestTextGolden(t *testing.T) {
	var buf bytes.Buffer
	if err := output.New(false, &buf).Print(sample); err != nil {
		t.Fatal(err)
	}
	golden(t, "result.txt", buf.Bytes())
}

func TestEmptyResultText(t *testing.T) {
	var buf bytes.Buffer
	if err := output.New(false, &buf).Print(core.Result{}); err != nil || buf.Len() != 0 {
		t.Errorf("got %q, %v", buf.String(), err)
	}
}
