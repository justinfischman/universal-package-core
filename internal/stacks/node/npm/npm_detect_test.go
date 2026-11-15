package npm

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
		{"lock file", fstest.MapFS{"package-lock.json": {}}, rankLockfile},
		{"lock file beats manifest", fstest.MapFS{"package-lock.json": {}, "package.json": {}}, rankLockfile},
		{"manifest", fstest.MapFS{"package.json": {}}, rankManifest},
		{"nothing", fstest.MapFS{}, 0},
		{"tsconfig alone is not a manager marker", fstest.MapFS{"tsconfig.json": {}}, 0},
	}
	n := newNPM(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := n.DetectRank(c.files); got != c.want {
				t.Errorf("rank = %d, want %d", got, c.want)
			}
			if n.Detect(c.files) != (c.want > 0) {
				t.Errorf("Detect disagrees with rank %d", c.want)
			}
		})
	}
}

func TestMarkers(t *testing.T) {
	if got := newNPM(t).Markers(); !reflect.DeepEqual(got, []string{"package-lock.json", "package.json"}) {
		t.Errorf("markers = %v", got)
	}
}

func TestDiagnose(t *testing.T) {
	cases := []struct {
		name  string
		files fstest.MapFS
		want  []string
	}{
		{"locked", fstest.MapFS{"package.json": {Data: []byte(`{}`)}, "package-lock.json": {}}, nil},
		{"unlocked", fstest.MapFS{"package.json": {Data: []byte(`{}`)}}, []string{"package-lock.json is missing"}},
		{"other manager declared", fstest.MapFS{"package.json": {Data: []byte(`{"packageManager":"yarn@4.0.0"}`)}, "package-lock.json": {}},
			[]string{`packageManager "yarn@4.0.0"`}},
		{"npm declared", fstest.MapFS{"package.json": {Data: []byte(`{"packageManager":"npm@11.0.0"}`)}, "package-lock.json": {}}, nil},
		{"no manifest", fstest.MapFS{"package-lock.json": {}}, nil},
	}
	n := newNPM(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got []string
			for _, f := range n.Diagnose(c.files) {
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
