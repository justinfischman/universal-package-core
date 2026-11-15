package envs

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

type mapLookup map[string]string

func (m mapLookup) Value(ref string) (string, bool) {
	v, ok := m[ref]
	return v, ok && v != ""
}

func TestResolveSimpleAndNested(t *testing.T) {
	r := EnvResolver{Lookup: mapLookup{
		"apps.api.port": "8000",
		"apps.api.url":  "http://localhost:${apps.api.port}",
	}}
	got, err := r.Resolve("base=${apps.api.url}/v1 port=${apps.api.port}")
	if err != nil {
		t.Fatal(err)
	}
	if want := "base=http://localhost:8000/v1 port=8000"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveTextWithoutVariablesIsUnchanged(t *testing.T) {
	got, err := EnvResolver{}.Resolve("plain $value")
	if err != nil || got != "plain $value" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestResolveMissingNamesTheVariable(t *testing.T) {
	r := EnvResolver{Lookup: mapLookup{"apps.api.url": ""}}
	for _, ref := range []string{"apps.api.url", "apps.ghost.url"} {
		_, err := r.Resolve("x ${" + ref + "} y")
		if !errors.Is(err, ErrUnresolved) || !strings.Contains(err.Error(), "${"+ref+"}") {
			t.Errorf("%s: err = %v", ref, err)
		}
	}
	if _, err := (EnvResolver{}).Resolve("${apps.api.url}"); !errors.Is(err, ErrUnresolved) {
		t.Errorf("nil lookup: err = %v", err)
	}
}

func TestResolveNestedMissingNamesInnerVariable(t *testing.T) {
	r := EnvResolver{Lookup: mapLookup{"apps.api.url": "http://h:${apps.api.port}"}}
	_, err := r.Resolve("${apps.api.url}")
	if !errors.Is(err, ErrUnresolved) || !strings.Contains(err.Error(), "${apps.api.port}") {
		t.Fatalf("err = %v", err)
	}
}

func TestResolveCycleNamesTheChain(t *testing.T) {
	r := EnvResolver{Lookup: mapLookup{
		"apps.a.url": "${apps.b.url}",
		"apps.b.url": "${apps.a.url}",
		"apps.s.url": "${apps.s.url}",
	}}
	_, err := r.Resolve("${apps.a.url}")
	if !errors.Is(err, ErrCycle) || !strings.Contains(err.Error(), "${apps.a.url} -> ${apps.b.url} -> ${apps.a.url}") {
		t.Fatalf("err = %v", err)
	}
	if _, err := r.Resolve("${apps.s.url}"); !errors.Is(err, ErrCycle) {
		t.Fatalf("self cycle: err = %v", err)
	}
}

func TestChildSortsAndNamesFailingKey(t *testing.T) {
	r := EnvResolver{Lookup: mapLookup{"apps.api.url": "http://x"}}
	got, err := r.Child(map[string]string{"B": "${apps.api.url}", "A": "1"})
	if err != nil || !reflect.DeepEqual(got, []string{"A=1", "B=http://x"}) {
		t.Fatalf("got %v, %v", got, err)
	}
	_, err = r.Child(map[string]string{"API": "${apps.nope.url}"})
	if !errors.Is(err, ErrUnresolved) || !strings.Contains(err.Error(), "env API") {
		t.Fatalf("err = %v", err)
	}
}
