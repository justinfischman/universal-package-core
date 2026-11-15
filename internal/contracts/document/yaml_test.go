package document

import (
	"strings"
	"testing"
)

func TestParseYAML(t *testing.T) {
	src := `# c
openapi: 3.0.0
list:
  - a
  - b: 1
    c: [x, "y z", 2]
  -
    - nested
same:
- q
- {k: v, n: null}
text: |
  line1
  # kept
  line2
folded: >-
  a
  b
quoted: 'it''s # not comment'  # real comment
empty:
n: 007
`
	v, err := ParseYAML([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := Canonical(v)
	want := `{
  "empty": null,
  "folded": "a b",
  "list": [
    "a",
    {
      "b": 1,
      "c": [
        "x",
        "y z",
        2
      ]
    },
    [
      "nested"
    ]
  ],
  "n": "007",
  "openapi": "3.0.0",
  "quoted": "it's # not comment",
  "same": [
    "q",
    {
      "k": "v",
      "n": null
    }
  ],
  "text": "line1\n# kept\nline2\n"
}
`
	if string(got) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestParseYAMLRejectsUnsupported(t *testing.T) {
	for name, src := range map[string]string{
		"anchor":    "a: &x 1\n",
		"alias":     "a: *x\n",
		"tab":       "a:\n\tb: 1\n",
		"duplicate": "a: 1\na: 2\n",
		"multidoc":  "a: 1\n---\nb: 2\n",
		"multiline": "a: one\n  two\n",
		"flow open": "a: [1, 2\n",
		"empty":     "# nothing\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseYAML([]byte(src)); err == nil {
				t.Fatal("want error")
			} else if !strings.Contains(err.Error(), "yaml") {
				t.Fatalf("unexpected error %v", err)
			}
		})
	}
}

func TestDecodeJSON(t *testing.T) {
	if _, err := Decode([]byte(`{"a":1} x`)); err == nil {
		t.Fatal("trailing data must fail")
	}
	v, err := Decode([]byte(`{"b":1.50,"a":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := Canonical(v); !strings.Contains(string(got), "1.50") {
		t.Fatalf("number not preserved: %s", got)
	}
}
