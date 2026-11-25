package workspace

import (
	"reflect"
	"testing"
	"testing/fstest"
)

func memberTree() fstest.MapFS {
	return fstest.MapFS{
		"apps/api/member.txt":               {},
		"apps/web/member.txt":               {},
		"apps/old/member.txt":               {},
		"libs/a/b/member.txt":               {},
		"libs/c/member.txt":                 {},
		"node_modules/x/y/member.txt":       {},
		"apps/api/node_modules/z/index.txt": {},
		"README.md":                         {},
	}
}

func TestExpandMembers(t *testing.T) {
	cases := map[string]struct {
		patterns []string
		want     []string
	}{
		"literal kept as written": {[]string{"apps/api", "./apps/web/", "gone"}, []string{"apps/api", "apps/web", "gone"}},
		"star":                    {[]string{"apps/*"}, []string{"apps/api", "apps/old", "apps/web"}},
		"exclude":                 {[]string{"apps/*", "!apps/old"}, []string{"apps/api", "apps/web"}},
		"double star":             {[]string{"libs/**"}, []string{"libs", "libs/a", "libs/a/b", "libs/c"}}, // `**` also matches zero directories
		"question and class":      {[]string{"apps/ap?", "libs/[ac]"}, []string{"apps/api", "libs/a", "libs/c"}},
		"duplicates collapse":     {[]string{"apps/api", "apps/*"}, []string{"apps/api", "apps/old", "apps/web"}},
		"node_modules not walked": {[]string{"**/y", "*/*"}, []string{"apps/api", "apps/old", "apps/web", "libs/a", "libs/c"}},
		"no match":                {[]string{"nothing/*"}, nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ExpandMembers(memberTree(), tc.patterns)
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ExpandMembers(%q) = %q, %v; want %q", tc.patterns, got, err, tc.want)
			}
		})
	}
}
