package config_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestValidateApps(t *testing.T) {
	good := map[string]config.App{
		"api": {Path: "apps/api", Port: 8000, Provides: []string{"http"}},
		"web": {Path: "apps/web", Consumes: []string{"api"}},
	}
	cases := []struct {
		name string
		apps map[string]config.App
		want string // substring of the error; "" means valid
	}{
		{"valid", good, ""},
		{"unknown consumer target", map[string]config.App{"web": {Path: "w", Consumes: []string{"ghost"}}}, `unknown app "ghost"`},
		{"self consume", map[string]config.App{"web": {Path: "w", Consumes: []string{"web"}}}, "cannot consume itself"},
		{"duplicate name ignoring case", map[string]config.App{"api": {Path: "a"}, "API": {Path: "b"}}, "duplicate app name"},
		{"duplicate path", map[string]config.App{"a": {Path: "x"}, "b": {Path: "x/"}}, "share path"},
		{"escaping path", map[string]config.App{"a": {Path: "../x"}}, "inside the workspace"},
		{"missing path", map[string]config.App{"a": {}}, "inside the workspace"},
		{"bad port", map[string]config.App{"a": {Path: "a", Port: 70000}}, "out of range"},
		{"bad name", map[string]config.App{"a b": {Path: "a"}}, "invalid app name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := config.ValidateApps(tc.apps)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, core.ErrInvalidRequest) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want invalid request containing %q", err, tc.want)
			}
		})
	}
}

func TestAppsRoundTripAndProjectOverridesByName(t *testing.T) {
	st := newStore(t)
	cfg := config.Config{Apps: map[string]config.App{
		"api": {Path: "apps/api", Port: 8000, URL: "http://localhost:${apps.api.port}", Provides: []string{"http"}, Env: map[string]string{"A": "1"}},
	}}
	if err := st.Save(config.Project, cfg); err != nil {
		t.Fatal(err)
	}
	res, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	got := res.Apps["api"]
	if got.Port != 8000 || got.Env["A"] != "1" || got.Provides[0] != "http" {
		t.Fatalf("app = %+v", got)
	}
}
