package config_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/core/coretest"
)

func TestScriptsAndUpRoundTripAndMerge(t *testing.T) {
	s := newStore(t)
	s.Registries, _ = coretest.Registries()
	global := config.Config{Scripts: map[string][]string{"lint": {"golint"}, "dev": {"a"}}, Up: []string{"global-up"}}
	project := config.Config{Scripts: map[string][]string{"dev": {"b", "--x"}}, Up: []string{"srv", "--port", "1"}}
	if err := s.Save(config.Global, global); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(config.Project, project); err != nil {
		t.Fatal(err)
	}
	got, err := s.LoadScope(config.Project)
	if err != nil || !reflect.DeepEqual(got, project) {
		t.Fatalf("round trip = %+v, %v", got, err)
	}
	res, err := s.LoadMerged()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{"lint": {"golint"}, "dev": {"b", "--x"}}
	if !reflect.DeepEqual(res.Scripts, want) {
		t.Errorf("merged scripts = %v, want %v", res.Scripts, want)
	}
	if !reflect.DeepEqual(res.Up, project.Up) {
		t.Errorf("merged up = %v", res.Up)
	}
	if !reflect.DeepEqual(global.Scripts["dev"], []string{"a"}) {
		t.Error("merge mutated the base layer")
	}
}

func TestValidateCommands(t *testing.T) {
	regs, _ := coretest.Registries()
	good := config.Config{Scripts: map[string][]string{"dev": {"srv"}}, Up: []string{"srv"}}
	if err := config.Validate(good, regs); err != nil {
		t.Errorf("valid commands rejected: %v", err)
	}
	bad := []config.Config{
		{Scripts: map[string][]string{"dev": {}}},
		{Scripts: map[string][]string{"dev": {" "}}},
		{Scripts: map[string][]string{" ": {"x"}}},
		{Up: []string{}},
		{Up: []string{""}},
	}
	for _, c := range bad {
		if err := config.Validate(c, regs); !errors.Is(err, core.ErrInvalidRequest) {
			t.Errorf("Validate(%+v) = %v, want ErrInvalidRequest", c, err)
		}
	}
}
