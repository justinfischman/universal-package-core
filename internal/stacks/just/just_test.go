package just_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/just"
)

func dirWithJustfile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "justfile"), []byte("build:\n\techo hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRecipesListsSummary(t *testing.T) {
	dir := dirWithJustfile(t)
	f := &execx.FakeRunner{Responses: map[string]execx.Response{
		"just --summary": {Output: execx.Output{Stdout: "build test lint\n"}},
	}}
	got, err := just.New(f).Recipes(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"build", "test", "lint"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
	if f.Calls[0].Dir != dir {
		t.Errorf("dir %q", f.Calls[0].Dir)
	}
}

func TestRecipesSkippedWhenJustMissing(t *testing.T) {
	f := &execx.FakeRunner{Responses: map[string]execx.Response{
		"just --summary": {Err: &core.ToolMissingError{Tool: "just"}},
	}}
	got, err := just.New(f).Recipes(context.Background(), dirWithJustfile(t))
	if err != nil || got != nil {
		t.Errorf("got %v, %v; want nil, nil", got, err)
	}
}

func TestRecipesAbsentJustfileRunsNothing(t *testing.T) {
	f := &execx.FakeRunner{}
	got, err := just.New(f).Recipes(context.Background(), t.TempDir())
	if err != nil || got != nil || len(f.Calls) != 0 {
		t.Errorf("got %v, %v, calls %d", got, err, len(f.Calls))
	}
}

func TestRecipesOtherErrorsSurface(t *testing.T) {
	f := &execx.FakeRunner{Responses: map[string]execx.Response{
		"just --summary": {Err: &execx.ExitError{Code: 1}},
	}}
	if _, err := just.New(f).Recipes(context.Background(), dirWithJustfile(t)); err == nil {
		t.Error("want error")
	}
}

func TestRunPassesRecipeAndArgs(t *testing.T) {
	f := &execx.FakeRunner{}
	if err := just.New(f).Run(context.Background(), "/p", "build", []string{"--release"}); err != nil {
		t.Fatal(err)
	}
	if got := f.Calls[0]; got.String() != "just build --release" || got.Dir != "/p" || !got.Stream {
		t.Errorf("call %+v", got)
	}
}
