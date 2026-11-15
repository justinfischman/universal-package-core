// Package contracts chooses the ContractChecker for a contract file.
package contracts

import (
	"fmt"

	"github.com/justinfischman/universal-package-core/internal/contracts/document"
	"github.com/justinfischman/universal-package-core/internal/contracts/jsonschema"
	"github.com/justinfischman/universal-package-core/internal/contracts/openapi"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// All lists the available checkers.
func All() []core.ContractChecker {
	return []core.ContractChecker{openapi.Checker{}, jsonschema.Checker{}}
}

// ByName returns the checker registered under name, as stored in lock files.
func ByName(name string) (core.ContractChecker, bool) {
	for _, c := range All() {
		if c.Name() == name {
			return c, true
		}
	}
	return nil, false
}

// Detect picks the checker from the file content, so apps need no explicit
// contract type: a document with an "openapi" or "swagger" key is OpenAPI;
// one with a JSON Schema keyword is JSON Schema.
func Detect(path string) (core.ContractChecker, error) {
	doc, err := document.Load(path)
	if err != nil {
		return nil, err
	}
	switch {
	case openapi.Matches(doc):
		return openapi.Checker{}, nil
	case jsonschema.Matches(doc):
		return jsonschema.Checker{}, nil
	}
	return nil, fmt.Errorf("%w: cannot tell the contract type of %s (expected an OpenAPI or JSON Schema document)", core.ErrInvalidRequest, path)
}
