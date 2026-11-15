// Package document loads contract files (JSON or YAML) into generic values and
// renders them as canonical JSON. It is shared by the contract checkers.
package document

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// Map is a decoded JSON or YAML object.
type Map = map[string]any

// Load reads path and decodes it. A document starting with "{" or "[" is JSON;
// anything else is read as the YAML subset documented on ParseYAML.
func Load(path string) (any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: read contract %s: %v", core.ErrInvalidRequest, path, err)
	}
	v, err := Decode(data)
	if err != nil {
		return nil, fmt.Errorf("%w: contract %s: %v", core.ErrInvalidRequest, path, err)
	}
	return v, nil
}

// Decode decodes JSON or YAML bytes. Numbers are kept as json.Number so that
// they render exactly as written.
func Decode(data []byte) (any, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("document is empty")
	}
	if trimmed[0] == '{' || trimmed[0] == '[' {
		dec := json.NewDecoder(bytes.NewReader(trimmed))
		dec.UseNumber()
		var v any
		if err := dec.Decode(&v); err != nil {
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
		if dec.More() {
			return nil, fmt.Errorf("invalid JSON: trailing data")
		}
		return v, nil
	}
	return ParseYAML(data)
}

// Canonical renders v as indented JSON with sorted object keys and a trailing
// newline. Equal documents give byte-equal output.
func Canonical(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil { // map keys are sorted by encoding/json
		return nil, err
	}
	return buf.Bytes(), nil
}

// Keys returns the sorted keys of m.
func Keys(m Map) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Object returns v as a Map, or nil when it is not an object.
func Object(v any) Map {
	m, _ := v.(Map)
	return m
}

// Equal reports whether two decoded values have the same canonical form.
func Equal(a, b any) bool {
	x, errX := Canonical(a)
	y, errY := Canonical(b)
	return errX == nil && errY == nil && bytes.Equal(x, y)
}

// Lines joins summary lines with newlines, without a trailing newline.
func Lines(lines []string) string { return strings.Join(lines, "\n") }

// Union returns the sorted keys present in either map.
func Union[V any](a, b map[string]V) []string {
	merged := make(Map, len(a)+len(b))
	for k := range a {
		merged[k] = nil
	}
	for k := range b {
		merged[k] = nil
	}
	return Keys(merged)
}
