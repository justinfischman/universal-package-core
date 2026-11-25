package bun

import (
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func TestDetectRank(t *testing.T) {
	cases := []struct {
		name  string
		files fstest.MapFS
		want  int
	}{
		{"lock file", fstest.MapFS{"bun.lock": {}}, rankLockfile},
		{"lock file beats declaration", fstest.MapFS{"bun.lock": {}, "package.json": {Data: []byte(`{"packageManager":"bun@4.0.0"}`)}}, rankLockfile},
		{"declaration", fstest.MapFS{"package.json": {Data: []byte(`{"packageManager":"bun@1.22.22"}`)}}, rankDeclared},
		{"bare manifest is npm's", fstest.MapFS{"package.json": {Data: []byte(`{}`)}}, 0},
		{"other declaration", fstest.MapFS{"package.json": {Data: []byte(`{"packageManager":"npm@11.0.0"}`)}}, 0},
		{"nothing", fstest.MapFS{}, 0},
	}
	y := newBun(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := y.DetectRank(c.files); got != c.want {
				t.Errorf("rank = %d, want %d", got, c.want)
			}
		})
	}
}

func TestMarkers(t *testing.T) {
	if got := newBun(t).Markers(); !reflect.DeepEqual(got, []string{"bun.lock", "bun.lockb", "package.json"}) && !reflect.DeepEqual(got, []string{"package.json", "bun.lock", "bun.lockb"}) {
		t.Errorf("markers = %v", got)
	}
}

func TestDiagnose(t *testing.T) {
	cases := []struct {
		name  string
		files fstest.MapFS
		want  []string
	}{
		{"locked", fstest.MapFS{"package.json": {Data: []byte(`{}`)}, "bun.lock": {}}, nil},
		{"unlocked", fstest.MapFS{"package.json": {Data: []byte(`{}`)}}, []string{"bun.lock is missing"}},
		{"other manager declared", fstest.MapFS{"package.json": {Data: []byte(`{"packageManager":"npm@11.0.0"}`)}, "bun.lock": {}}, []string{`packageManager "npm@11.0.0"`}},
		{"no manifest", fstest.MapFS{"bun.lock": {}}, nil},
	}
	y := newBun(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got []string
			for _, f := range y.Diagnose(c.files) {
				got = append(got, f.Message)
			}
			if len(got) != len(c.want) {
				t.Fatalf("findings = %q, want %q", got, c.want)
			}
			for i := range got {
				if !strings.Contains(got[i], c.want[i]) {
					t.Errorf("finding %q does not mention %q", got[i], c.want[i])
				}
			}
		})
	}
}
