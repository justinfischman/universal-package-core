package execx_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestOSRunCapturesOutput(t *testing.T) {
	out, err := execx.OS{}.Run(context.Background(), execx.Cmd{Name: "go", Args: []string{"version"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.Stdout, "go version") {
		t.Errorf("stdout = %q", out.Stdout)
	}
}

func TestOSRunDryRunDoesNotExecute(t *testing.T) {
	var logs bytes.Buffer
	r := execx.OS{DryRun: true, Logger: slog.New(slog.NewTextHandler(&logs, nil))}
	// A nonexistent binary would fail if executed.
	if _, err := r.Run(context.Background(), execx.Cmd{Name: "definitely-not-a-binary"}); err != nil {
		t.Fatalf("dry-run executed: %v", err)
	}
	if !strings.Contains(logs.String(), "dry_run=true") {
		t.Errorf("dry-run not logged: %s", logs.String())
	}
}

func TestOSRunMapsNotFound(t *testing.T) {
	_, err := execx.OS{}.Run(context.Background(), execx.Cmd{Name: "definitely-not-a-binary"})
	var missing *core.ToolMissingError
	if !errors.As(err, &missing) || missing.Tool != "definitely-not-a-binary" {
		t.Fatalf("got %v", err)
	}
	if core.ExitCode(err) != core.ExitToolMissing {
		t.Errorf("exit code = %d", core.ExitCode(err))
	}
}

func TestOSRunNonZeroExit(t *testing.T) {
	out, err := execx.OS{}.Run(context.Background(), execx.Cmd{Name: "go", Args: []string{"bogus-subcommand"}})
	var exitErr *execx.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("got %v", err)
	}
	if exitErr.Code == 0 || out.ExitCode != exitErr.Code {
		t.Errorf("code = %d, output code = %d", exitErr.Code, out.ExitCode)
	}
}

func TestStreamCopiesOutput(t *testing.T) {
	var w bytes.Buffer
	r := execx.OS{Stdout: &w}
	if _, err := r.Run(context.Background(), execx.Cmd{Name: "go", Args: []string{"version"}, Stream: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(w.String(), "go version") {
		t.Errorf("streamed = %q", w.String())
	}
}

func TestFakeRunner(t *testing.T) {
	f := &execx.FakeRunner{Responses: map[string]execx.Response{
		"alpha add requests": {Output: execx.Output{Stdout: "ok"}},
		"alpha fail":         {Err: errors.New("boom")},
	}}
	out, err := f.Run(context.Background(), execx.Cmd{Name: "alpha", Args: []string{"add", "requests"}})
	if err != nil || out.Stdout != "ok" {
		t.Errorf("scripted: %v %q", err, out.Stdout)
	}
	if _, err := f.Run(context.Background(), execx.Cmd{Name: "alpha", Args: []string{"fail"}}); err == nil {
		t.Error("expected scripted error")
	}
	if _, err := f.Run(context.Background(), execx.Cmd{Name: "other"}); err != nil {
		t.Error(err)
	}
	if len(f.Calls) != 3 {
		t.Errorf("calls = %d", len(f.Calls))
	}
}

func TestWithEnvPrependsEntries(t *testing.T) {
	fake := &execx.FakeRunner{}
	r := execx.WithEnv(fake, "UPC_ENV=dev")
	if _, err := r.Run(context.Background(), execx.Cmd{Name: "x", Env: []string{"A=1"}}); err != nil {
		t.Fatal(err)
	}
	if got := fake.Calls[0].Env; !reflect.DeepEqual(got, []string{"UPC_ENV=dev", "A=1"}) {
		t.Errorf("Env = %v", got)
	}
	if execx.WithEnv(fake) != execx.Runner(fake) {
		t.Error("no entries must return the runner unchanged")
	}
}

// TestHelperProcess is the background program the OS controller tests start.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("UPC_EXECX_HELPER") != "1" {
		return
	}
	fmt.Println("helper up")
	time.Sleep(time.Minute)
}

func TestOSControllerStartsAndStopsRealProcess(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "up.log")
	r := execx.OS{}
	pid, err := r.Start(context.Background(), execx.Cmd{
		Name: os.Args[0], Args: []string{"-test.run=^TestHelperProcess$"}, Env: []string{"UPC_EXECX_HELPER=1"},
	}, logPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Stop(pid) })
	if !r.Alive(pid) {
		t.Fatal("started process is not alive")
	}
	if err := r.Stop(pid); err != nil {
		t.Fatal(err)
	}
	if r.Alive(pid) {
		t.Error("process alive after Stop")
	}
	if err := r.Stop(pid); err != nil {
		t.Errorf("stopping a gone process = %v, want nil", err)
	}
}

func TestOSStartDryRunStartsNothing(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "up.log")
	pid, err := execx.OS{DryRun: true}.Start(context.Background(), execx.Cmd{Name: "does-not-exist"}, logPath)
	if err != nil || pid != 0 {
		t.Fatalf("Start = %d, %v", pid, err)
	}
	if _, err := os.Stat(logPath); err == nil {
		t.Error("dry run created a log")
	}
}

func TestOSStartMissingToolIsToolMissing(t *testing.T) {
	_, err := execx.OS{}.Start(context.Background(), execx.Cmd{Name: "upc-no-such-binary"}, filepath.Join(t.TempDir(), "l"))
	var missing *core.ToolMissingError
	if !errors.As(err, &missing) {
		t.Fatalf("err = %v, want ToolMissingError", err)
	}
}
