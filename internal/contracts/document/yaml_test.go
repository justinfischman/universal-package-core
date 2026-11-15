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
  "n": 7,
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

func TestParseYAMLResolvesAnchorsAndMerges(t *testing.T) {
	src := `base: &b
  a: 1
  b: 2
use: *b
merged:
  <<: *b
  b: 3
multi:
  <<: [*b, {a: 9, c: 4}]
tagged: !custom value
stamp: 2024-01-02
bin: !!binary aGk=
`
	v, err := ParseYAML([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := Canonical(v)
	want := `{
  "base": {
    "a": 1,
    "b": 2
  },
  "bin": "hi",
  "merged": {
    "a": 1,
    "b": 3
  },
  "multi": {
    "a": 1,
    "b": 2,
    "c": 4
  },
  "stamp": "2024-01-02T00:00:00Z",
  "tagged": "value",
  "use": {
    "a": 1,
    "b": 2
  }
}
`
	if string(got) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestParseYAMLNumbersAreCanonical(t *testing.T) {
	v, err := ParseYAML([]byte("a: 1.50\nb: -0\nc: 1e3\nd: 0x1F\ne: .inf\nf: 18446744073709551615\n"))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := Canonical(v)
	want := "{\n  \"a\": 1.5,\n  \"b\": 0,\n  \"c\": 1000,\n  \"d\": 31,\n  \"e\": \"+Inf\",\n  \"f\": 18446744073709551615\n}\n"
	if string(got) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestParseYAMLRejects(t *testing.T) {
	for name, src := range map[string]string{
		"duplicate":     "a: 1\na: 2\n",
		"multidoc":      "a: 1\n---\nb: 2\n",
		"flow open":     "a: [1, 2\n",
		"empty":         "# nothing\n",
		"blank":         "",
		"complex key":   "? [a, b]\n: 1\n",
		"bad merge":     "a:\n  <<: 1\n",
		"unknown alias": "a: *x\n",
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
