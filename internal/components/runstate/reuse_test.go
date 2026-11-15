package runstate

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
)

func TestStopKeepsLogAndRemovesPid(t *testing.T) {
	s, _ := newState(t, t.TempDir())
	if _, err := s.Start(context.Background(), execx.Cmd{Name: "srv", Dir: s.Dir}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.LogFile(), []byte("boom\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := s.Stop(); err != nil || out != Stopped {
		t.Fatalf("Stop = %v, %v", out, err)
	}
	if exists(s.PidFile()) {
		t.Error("pid file left behind")
	}
	if !exists(s.LogFile()) {
		t.Error("log removed by Stop; it must stay for diagnosis")
	}
}

func TestPidFileRecordsStartToken(t *testing.T) {
	s, _ := newState(t, t.TempDir())
	if _, err := s.Start(context.Background(), execx.Cmd{Name: "srv", Dir: s.Dir}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(s.PidFile())
	if err != nil || !strings.HasPrefix(string(data), "1000 start-1000") {
		t.Fatalf("pid file = %q, %v", data, err)
	}
}

func TestReusedPidIsNotRunningAndNotStopped(t *testing.T) {
	s, fake := newState(t, t.TempDir())
	if _, err := s.Start(context.Background(), execx.Cmd{Name: "srv", Dir: s.Dir}); err != nil {
		t.Fatal(err)
	}
	fake.Procs[0].Stamp = "another-process" // the pid now belongs to someone else
	if s.Running() {
		t.Error("a reused pid reported as running")
	}
	out, err := s.Stop()
	if err != nil || out != Stale {
		t.Fatalf("Stop = %v, %v; want stale", out, err)
	}
	if !fake.Procs[0].Running {
		t.Error("Stop signalled a process it did not start")
	}
	if exists(s.PidFile()) {
		t.Error("stale pid file left behind")
	}
}

func TestPidOnlyRecordStillWorks(t *testing.T) {
	s, fake := newState(t, t.TempDir())
	if _, err := s.Start(context.Background(), execx.Cmd{Name: "srv", Dir: s.Dir}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.PidFile(), []byte("1000"), 0o644); err != nil { // older record format
		t.Fatal(err)
	}
	if !s.Running() {
		t.Error("record without a token not treated as running")
	}
	if out, err := s.Stop(); err != nil || out != Stopped || fake.Procs[0].Running {
		t.Fatalf("Stop = %v, %v", out, err)
	}
}
