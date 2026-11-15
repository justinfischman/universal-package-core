// Package openapi checks OpenAPI (and Swagger 2) contract files.
package openapi

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/contracts/document"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// Name identifies this checker in lock files.
const Name = "openapi"

// httpMethods are the keys of a path item that declare an operation.
var httpMethods = map[string]bool{
	"get": true, "put": true, "post": true, "delete": true,
	"options": true, "head": true, "patch": true, "trace": true,
}

// Checker implements core.ContractChecker for OpenAPI documents.
type Checker struct{}

var _ core.ContractChecker = Checker{}

// Name returns "openapi".
func (Checker) Name() string { return Name }

// Matches reports whether a decoded document is an OpenAPI or Swagger one.
func Matches(doc any) bool {
	m := document.Object(doc)
	_, oas := m["openapi"]
	_, swagger := m["swagger"]
	return oas || swagger
}

// Snapshot returns the document as canonical JSON (sorted keys).
func (Checker) Snapshot(path string) ([]byte, error) {
	doc, err := document.Load(path)
	if err != nil {
		return nil, err
	}
	if !Matches(doc) {
		return nil, fmt.Errorf("%w: %s is not an OpenAPI document (no openapi or swagger key)", core.ErrInvalidRequest, path)
	}
	return document.Canonical(doc)
}

// Diff lists added (+), removed (-) and changed (~) paths and operations.
func (Checker) Diff(old, new []byte) string {
	oldPaths, errOld := pathsOf(old)
	newPaths, errNew := pathsOf(new)
	if errOld != nil || errNew != nil {
		return "snapshot could not be compared (invalid JSON)"
	}
	var out []string
	for _, p := range document.Union(oldPaths, newPaths) {
		o, inOld := oldPaths[p]
		n, inNew := newPaths[p]
		switch {
		case !inOld:
			out = append(out, "+ path "+p)
		case !inNew:
			out = append(out, "- path "+p)
		default:
			out = append(out, diffPathItem(p, o, n)...)
		}
	}
	out = append(out, otherChanges(old, new)...)
	return document.Lines(out)
}

func diffPathItem(path string, o, n document.Map) []string {
	var out []string
	for _, k := range document.Union(o, n) {
		ov, inOld := o[k]
		nv, inNew := n[k]
		label := "path " + path
		if httpMethods[k] {
			label = strings.ToUpper(k) + " " + path
		}
		switch {
		case !inOld:
			out = append(out, "+ "+label)
		case !inNew:
			out = append(out, "- "+label)
		case !document.Equal(ov, nv):
			out = append(out, "~ "+label)
		}
	}
	return out
}

func pathsOf(snapshot []byte) (map[string]document.Map, error) {
	root, err := decode(snapshot)
	if err != nil {
		return nil, err
	}
	out := map[string]document.Map{}
	for p, item := range document.Object(root["paths"]) {
		out[p] = document.Object(item)
	}
	return out, nil
}

// otherChanges reports top-level sections other than paths (info, components,
// version fields, ...) that differ, so that no change is silent.
func otherChanges(old, new []byte) []string {
	o, errO := decode(old)
	n, errN := decode(new)
	if errO != nil || errN != nil {
		return nil
	}
	var out []string
	for _, k := range document.Union(o, n) {
		if k != "paths" && !document.Equal(o[k], n[k]) {
			out = append(out, "~ "+k)
		}
	}
	return out
}

func decode(snapshot []byte) (document.Map, error) {
	var root document.Map
	if err := json.Unmarshal(snapshot, &root); err != nil {
		return nil, err
	}
	return root, nil
}
