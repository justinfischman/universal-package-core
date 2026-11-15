// Package jsonschema checks JSON Schema contract files.
package jsonschema

import (
	"encoding/json"
	"fmt"

	"github.com/justinfischman/universal-package-core/internal/contracts/document"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// Name identifies this checker in lock files.
const Name = "jsonschema"

// structural keywords are compared recursively, not as part of a block's own
// keywords.
var structural = map[string]bool{"properties": true, "required": true, "items": true, "$defs": true, "definitions": true}

// Checker implements core.ContractChecker for JSON Schema documents.
type Checker struct{}

var _ core.ContractChecker = Checker{}

// Name returns "jsonschema".
func (Checker) Name() string { return Name }

// Matches reports whether a decoded document looks like a JSON Schema: an
// object with a schema keyword.
func Matches(doc any) bool {
	m := document.Object(doc)
	for _, k := range []string{"$schema", "type", "properties", "$defs", "definitions", "items", "allOf", "anyOf", "oneOf", "$ref"} {
		if _, ok := m[k]; ok {
			return true
		}
	}
	return false
}

// Snapshot returns the schema as canonical JSON (sorted keys).
func (Checker) Snapshot(path string) ([]byte, error) {
	doc, err := document.Load(path)
	if err != nil {
		return nil, err
	}
	if !Matches(doc) {
		return nil, fmt.Errorf("%w: %s is not a JSON Schema document (no schema keyword)", core.ErrInvalidRequest, path)
	}
	return document.Canonical(doc)
}

// Diff lists added (+), removed (-) and changed (~) properties, and changes
// to required lists. Nested properties are named with dotted paths.
func (Checker) Diff(old, new []byte) string {
	var o, n document.Map
	if json.Unmarshal(old, &o) != nil || json.Unmarshal(new, &n) != nil {
		return "snapshot could not be compared (invalid JSON)"
	}
	return document.Lines(diffSchema("", o, n))
}

func diffSchema(prefix string, o, n document.Map) []string {
	var out []string
	if !document.Equal(ownKeywords(o), ownKeywords(n)) {
		out = append(out, "~ keywords "+label(prefix))
	}
	out = append(out, diffRequired(prefix, o["required"], n["required"])...)
	oldProps, newProps := document.Object(o["properties"]), document.Object(n["properties"])
	for _, name := range document.Union(oldProps, newProps) {
		child := join(prefix, name)
		ov, inOld := oldProps[name]
		nv, inNew := newProps[name]
		switch {
		case !inOld:
			out = append(out, "+ property "+child)
		case !inNew:
			out = append(out, "- property "+child)
		case !document.Equal(ov, nv):
			out = append(out, "~ property "+child)
			out = append(out, diffSchema(child, document.Object(ov), document.Object(nv))...)
		}
	}
	if !document.Equal(o["items"], n["items"]) {
		out = append(out, diffSchema(join(prefix, "[]"), document.Object(o["items"]), document.Object(n["items"]))...)
	}
	for _, key := range []string{"$defs", "definitions"} {
		out = append(out, diffDefs(key, document.Object(o[key]), document.Object(n[key]))...)
	}
	return out
}

func diffDefs(key string, o, n document.Map) []string {
	var out []string
	for _, name := range document.Union(o, n) {
		child := key + "/" + name
		ov, inOld := o[name]
		nv, inNew := n[name]
		switch {
		case !inOld:
			out = append(out, "+ definition "+child)
		case !inNew:
			out = append(out, "- definition "+child)
		case !document.Equal(ov, nv):
			out = append(out, "~ definition "+child)
			out = append(out, diffSchema(child, document.Object(ov), document.Object(nv))...)
		}
	}
	return out
}

func diffRequired(prefix string, o, n any) []string {
	oldSet, newSet := names(o), names(n)
	var out []string
	for _, name := range document.Union(oldSet, newSet) {
		_, inOld := oldSet[name]
		_, inNew := newSet[name]
		switch {
		case !inOld:
			out = append(out, fmt.Sprintf("+ required %s: %s", label(prefix), name))
		case !inNew:
			out = append(out, fmt.Sprintf("- required %s: %s", label(prefix), name))
		}
	}
	return out
}

// ownKeywords is the block without its structural children.
func ownKeywords(m document.Map) document.Map {
	out := document.Map{}
	for k, v := range m {
		if !structural[k] {
			out[k] = v
		}
	}
	return out
}

func names(v any) map[string]struct{} {
	out := map[string]struct{}{}
	list, _ := v.([]any)
	for _, item := range list {
		if s, ok := item.(string); ok {
			out[s] = struct{}{}
		}
	}
	return out
}

func join(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func label(prefix string) string {
	if prefix == "" {
		return "(root)"
	}
	return prefix
}
