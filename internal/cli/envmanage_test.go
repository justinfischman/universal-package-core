package cli

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestEnvAddDeclaresEnvironment(t *testing.T) {
	h := newHarness(t)
	h.touch(t, ".env.dev")
	if code := h.run("env", "add", "dev", "--groups", "test,docs"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	cfg, _ := readConfig(t, h.project)
	got := cfg.Environments["dev"]
	if got.File != "" || !reflect.DeepEqual(got.Groups, []string{"test", "docs"}) {
		t.Errorf("environment = %+v", got)
	}
	h.stdout.Reset()
	if code := h.run("env", "list"); code != core.ExitOK || !strings.Contains(h.stdout.String(), "groups=test,docs") {
		t.Errorf("list: exit %d stdout %q", code, h.stdout)
	}
	if strings.Contains(h.stdout.String(), "hunter2") {
		t.Error("env values must never be printed")
	}
}

func TestEnvAddWithFileAndRefusals(t *testing.T) {
	h := newHarness(t)
	if code := h.run("env", "add", "prod", "--file", "conf/prod.env"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if cfg, _ := readConfig(t, h.project); cfg.Environments["prod"].File != "conf/prod.env" {
		t.Errorf("config = %+v", cfg)
	}
	for _, args := range [][]string{
		{"env", "add", "prod"},                      // already declared
		{"env", "add", "bad name"},                  // invalid name
		{"env", "add", "x", "--file", "../outside"}, // leaves the project
		{"env", "add", "x", "--groups", "-bad"},     // invalid group
		{"env", "add", "x", "--global"},             // per project only
		{"env", "add"},                              // missing name
	} {
		if code := h.run(args...); code != core.ExitUsage {
			t.Errorf("%v: exit %d, want %d (%s)", args, code, core.ExitUsage, h.stderr)
		}
	}
}

func TestEnvAddDryRunWritesNothing(t *testing.T) {
	h := newHarness(t)
	if code := h.run("env", "add", "dev", "--dry-run"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if _, ok := readConfig(t, h.project); ok {
		t.Error("dry run wrote config")
	}
}

func TestEnvRemoveDeletesDeclarationButNotFile(t *testing.T) {
	h := newHarness(t)
	h.touch(t, ".env.dev")
	h.writeProjectConfig(t, h.project, "[environments.dev]\ngroups = [\"test\"]\n[environments.qa]\n")
	if code := h.run("env", "remove", "dev"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	cfg, _ := readConfig(t, h.project)
	if _, ok := cfg.Environments["dev"]; ok || len(cfg.Environments) != 1 {
		t.Errorf("environments = %+v", cfg.Environments)
	}
	if code := h.run("env", "list"); code != core.ExitOK || !strings.Contains(h.stdout.String(), ".env.dev [convention]") {
		t.Errorf("env file must stay and be rediscovered: exit %d stdout %q", code, h.stdout)
	}
	if code := h.run("env", "remove", "dev"); code != core.ExitUsage {
		t.Errorf("removing an undeclared environment: exit %d, want %d", code, core.ExitUsage)
	}
}

func TestInstallEnvPassesGroups(t *testing.T) {
	h := newHarness(t)
	h.touch(t, ".env.dev")
	h.writeProjectConfig(t, h.project, "[environments.dev]\ngroups = [\"test\", \"docs\"]\n")
	if code := h.run("install", "--env", "dev", "--group", "lint"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	reqs := h.pm(t, "alpha-one").Requests
	if len(reqs) != 1 || !reflect.DeepEqual(reqs[0].AllGroups(), []string{"lint", "test", "docs"}) {
		t.Errorf("requests = %+v", reqs)
	}
	if cfg, _ := readConfig(t, h.project); cfg.ActiveEnvironment != "" {
		t.Errorf("--env must not persist a selection: %+v", cfg)
	}
}

func TestEnvFlagSelectsEnvironmentForRunTestUp(t *testing.T) {
	for _, args := range [][]string{{"run", "tool"}, {"test"}, {"up"}} {
		h := newHarness(t)
		fake := &execx.FakeRunner{}
		h.useRunner(fake)
		h.touch(t, ".env.dev")
		for _, c := range []core.Capability{core.CapRun, core.CapTest, core.CapUp} {
			err := h.pm(t, "alpha-one").Declare(c, core.Native(), func(ctx context.Context, _ core.Request) (core.Result, error) {
				r, ok := execx.FromContext(ctx)
				if !ok {
					t.Fatal("no runner in context")
				}
				_, err := r.Run(ctx, execx.Cmd{Name: "tool", Env: []string{"A=1"}})
				return core.Result{}, err
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		if code := h.run(withEnv(args, "dev")...); code != core.ExitOK {
			t.Fatalf("%v: exit %d: %s", args, code, h.stderr)
		}
		if len(fake.Calls) != 1 || !reflect.DeepEqual(fake.Calls[0].Env, []string{"UPC_ENV=dev", "A=1"}) {
			t.Errorf("%v: calls = %+v", args, fake.Calls)
		}
	}
}

func TestEnvFlagRefusesUnknownEnvironment(t *testing.T) {
	h := newHarness(t)
	for _, args := range [][]string{{"install"}, {"run", "x"}, {"test"}, {"up"}} {
		if code := h.run(withEnv(args, "nope")...); code != core.ExitUsage {
			t.Errorf("%v: exit %d, want %d (%s)", args, code, core.ExitUsage, h.stderr)
		}
	}
	if len(h.pm(t, "alpha-one").Requests) != 0 {
		t.Error("a refused --env must not reach the manager")
	}
}

// withEnv puts --env right after the command name: run, test and up stop flag
// parsing at the first positional so the rest reaches the underlying tool.
func withEnv(args []string, name string) []string {
	return append([]string{args[0], "--env", name}, args[1:]...)
}
