package workspace

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/config"
)

func load(t *testing.T, apps map[string]config.App) *Workspace {
	t.Helper()
	w, err := Load("/ws", config.Config{Apps: apps})
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func names(apps []*App) []string { return appNames(apps) }

func TestOrderPutsConsumedAppsFirst(t *testing.T) {
	w := load(t, map[string]config.App{
		"web":    {Path: "web", Consumes: []string{"api", "auth"}},
		"api":    {Path: "api", Consumes: []string{"db"}, Provides: []string{"http"}},
		"auth":   {Path: "auth"},
		"db":     {Path: "db"},
		"worker": {Path: "worker"},
	})
	got, err := w.Order()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"db", "api", "auth", "web", "worker"}; !reflect.DeepEqual(names(got), want) {
		t.Errorf("order = %v, want %v", names(got), want)
	}
}

func TestOrderCycleNamesTheCycle(t *testing.T) {
	w := load(t, map[string]config.App{
		"a": {Path: "a", Consumes: []string{"b"}},
		"b": {Path: "b", Consumes: []string{"c"}},
		"c": {Path: "c", Consumes: []string{"a"}},
		"d": {Path: "d"},
	})
	_, err := w.Order()
	if !errors.Is(err, ErrCycle) || !strings.Contains(err.Error(), "a -> b -> c -> a") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadRejectsUnknownConsumerTarget(t *testing.T) {
	_, err := Load("/ws", config.Config{Apps: map[string]config.App{"web": {Path: "w", Consumes: []string{"ghost"}}}})
	if err == nil || !strings.Contains(err.Error(), `"ghost"`) {
		t.Fatalf("err = %v", err)
	}
}

func TestChildEnvInterpolatesNestedAppFields(t *testing.T) {
	w := load(t, map[string]config.App{
		"api": {Path: "api", Port: 8000, URL: "http://localhost:${apps.api.port}", Provides: []string{"http"}},
		"web": {Path: "web", Consumes: []string{"api"}, Env: map[string]string{"API_URL": "${apps.api.url}/v1"}},
	})
	web, _ := w.App("web")
	got, err := w.ChildEnv(web)
	if err != nil || !reflect.DeepEqual(got, []string{"API_URL=http://localhost:8000/v1"}) {
		t.Fatalf("env = %v, %v", got, err)
	}
}

func TestFindings(t *testing.T) {
	w := load(t, map[string]config.App{
		"api":  {Path: "api", Port: 8000},
		"web":  {Path: "web", Port: 8000, Consumes: []string{"api"}, Env: map[string]string{"X": "${apps.api.url}"}},
		"docs": {Path: "docs", URL: "${apps.web.nope}"},
	})
	kinds := map[FindingKind]int{}
	var unresolved []string
	for _, f := range w.Findings() {
		kinds[f.Kind]++
		if f.Kind == UnresolvedVariable {
			unresolved = append(unresolved, f.Message)
		}
	}
	want := map[FindingKind]int{PortCollision: 1, ConsumerWithoutProvider: 1, UnresolvedVariable: 2}
	if !reflect.DeepEqual(kinds, want) {
		t.Fatalf("kinds = %v, want %v", kinds, want)
	}
	if !strings.Contains(strings.Join(unresolved, "|"), "${apps.web.nope}") {
		t.Errorf("unresolved findings = %v", unresolved)
	}
}

func TestCleanWorkspaceHasNoFindings(t *testing.T) {
	w := load(t, map[string]config.App{
		"api": {Path: "api", Port: 8000, URL: "http://localhost:${apps.api.port}", Provides: []string{"http"}},
		"web": {Path: "web", Port: 3000, Consumes: []string{"api"}, Env: map[string]string{"API_URL": "${apps.api.url}"}},
	})
	if f := w.Findings(); len(f) != 0 {
		t.Fatalf("findings = %v", f)
	}
}

// fakeRunner records calls and fails Start or Stop for scripted app names.
type fakeRunner struct {
	calls     []string
	failStart map[string]error
	failStop  map[string]error
	envs      map[string][]string
}

func (f *fakeRunner) Start(_ context.Context, a *App, env []string) error {
	f.calls = append(f.calls, "start "+a.Name)
	if f.envs == nil {
		f.envs = map[string][]string{}
	}
	f.envs[a.Name] = env
	return f.failStart[a.Name]
}

func (f *fakeRunner) Stop(_ context.Context, a *App) error {
	f.calls = append(f.calls, "stop "+a.Name)
	return f.failStop[a.Name]
}

func threeApps(t *testing.T) *Workspace {
	return load(t, map[string]config.App{
		"db":  {Path: "db", Provides: []string{"sql"}},
		"api": {Path: "api", Consumes: []string{"db"}, Provides: []string{"http"}, Port: 8000, URL: "http://h:${apps.api.port}"},
		"web": {Path: "web", Consumes: []string{"api"}, Env: map[string]string{"API": "${apps.api.url}"}},
	})
}

func TestUpStartsInOrderWithResolvedEnv(t *testing.T) {
	r := &fakeRunner{}
	started, err := Orchestrator{Workspace: threeApps(t), Runner: r}.Up(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"start db", "start api", "start web"}; !reflect.DeepEqual(r.calls, want) {
		t.Errorf("calls = %v, want %v", r.calls, want)
	}
	if !reflect.DeepEqual(started, []string{"db", "api", "web"}) {
		t.Errorf("started = %v", started)
	}
	if !reflect.DeepEqual(r.envs["web"], []string{"API=http://h:8000"}) {
		t.Errorf("web env = %v", r.envs["web"])
	}
}

func TestUpFailureRollsBackStartedAppsInReverse(t *testing.T) {
	boom := errors.New("boom")
	r := &fakeRunner{failStart: map[string]error{"web": boom}}
	_, err := Orchestrator{Workspace: threeApps(t), Runner: r}.Up(context.Background())
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), `"web"`) {
		t.Fatalf("err = %v", err)
	}
	want := []string{"start db", "start api", "start web", "stop api", "stop db"}
	if !reflect.DeepEqual(r.calls, want) {
		t.Errorf("calls = %v, want %v", r.calls, want)
	}
}

func TestUpRollbackKeepsGoingPastStopFailure(t *testing.T) {
	boom, stuck := errors.New("boom"), errors.New("stuck")
	r := &fakeRunner{failStart: map[string]error{"web": boom}, failStop: map[string]error{"api": stuck}}
	_, err := Orchestrator{Workspace: threeApps(t), Runner: r}.Up(context.Background())
	if !errors.Is(err, boom) || !errors.Is(err, stuck) {
		t.Fatalf("err = %v, want both failures", err)
	}
	if got := r.calls[len(r.calls)-1]; got != "stop db" {
		t.Errorf("last call = %q, want stop db", got)
	}
}

func TestUpFirstAppFailureStopsNothing(t *testing.T) {
	r := &fakeRunner{failStart: map[string]error{"db": errors.New("boom")}}
	if _, err := (Orchestrator{Workspace: threeApps(t), Runner: r}).Up(context.Background()); err == nil {
		t.Fatal("expected error")
	}
	if !reflect.DeepEqual(r.calls, []string{"start db"}) {
		t.Errorf("calls = %v", r.calls)
	}
}

func TestUpUnresolvedVariableRollsBackWithoutStartingTheApp(t *testing.T) {
	w := load(t, map[string]config.App{
		"api": {Path: "api"},
		"web": {Path: "web", Consumes: []string{"api"}, Env: map[string]string{"API": "${apps.api.url}"}},
	})
	r := &fakeRunner{}
	_, err := Orchestrator{Workspace: w, Runner: r}.Up(context.Background())
	if err == nil || !strings.Contains(err.Error(), "${apps.api.url}") {
		t.Fatalf("err = %v", err)
	}
	if want := []string{"start api", "stop api"}; !reflect.DeepEqual(r.calls, want) {
		t.Errorf("calls = %v, want %v", r.calls, want)
	}
}

func TestDownStopsInReverseAndContinuesPastFailure(t *testing.T) {
	stuck := errors.New("stuck")
	r := &fakeRunner{failStop: map[string]error{"api": stuck}}
	stopped, err := Orchestrator{Workspace: threeApps(t), Runner: r}.Down(context.Background())
	if !errors.Is(err, stuck) {
		t.Fatalf("err = %v", err)
	}
	if want := []string{"stop web", "stop api", "stop db"}; !reflect.DeepEqual(r.calls, want) {
		t.Errorf("calls = %v, want %v", r.calls, want)
	}
	if !reflect.DeepEqual(stopped, []string{"web", "db"}) {
		t.Errorf("stopped = %v", stopped)
	}
}

func TestSelectedAppsAndUnknownName(t *testing.T) {
	r := &fakeRunner{}
	o := Orchestrator{Workspace: threeApps(t), Runner: r}
	if _, err := o.Up(context.Background(), "web", "db"); err != nil {
		t.Fatal(err)
	}
	// web consumes api, which consumes db: the named apps bring their dependencies.
	if want := []string{"start db", "start api", "start web"}; !reflect.DeepEqual(r.calls, want) {
		t.Errorf("calls = %v, want %v", r.calls, want)
	}
	if _, err := o.Up(context.Background(), "ghost"); err == nil || !strings.Contains(err.Error(), `"ghost"`) {
		t.Errorf("err = %v", err)
	}
}

func TestUpCycleStartsNothing(t *testing.T) {
	w := load(t, map[string]config.App{
		"a": {Path: "a", Consumes: []string{"b"}},
		"b": {Path: "b", Consumes: []string{"a"}},
	})
	r := &fakeRunner{}
	if _, err := (Orchestrator{Workspace: w, Runner: r}).Up(context.Background()); !errors.Is(err, ErrCycle) {
		t.Fatalf("err = %v", err)
	}
	if len(r.calls) != 0 {
		t.Errorf("calls = %v", r.calls)
	}
}
