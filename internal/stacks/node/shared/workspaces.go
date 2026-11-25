package shared

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"slices"

	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// Workspaces returns the patterns in a manifest's `workspaces` field, which is
// either a list or an object with a `packages` list. A missing field is empty.
func Workspaces(manifest []byte) ([]string, error) {
	var doc struct {
		Workspaces json.RawMessage `json:"workspaces"`
	}
	if err := json.Unmarshal(manifest, &doc); err != nil {
		return nil, fmt.Errorf("%w: package.json: %v", core.ErrInvalidRequest, err)
	}
	if len(doc.Workspaces) == 0 {
		return nil, nil
	}
	var list []string
	if err := json.Unmarshal(doc.Workspaces, &list); err == nil {
		return list, nil
	}
	var obj struct {
		Packages []string `json:"packages"`
	}
	if err := json.Unmarshal(doc.Workspaces, &obj); err != nil {
		return nil, fmt.Errorf("%w: package.json: workspaces is neither a list nor an object", core.ErrInvalidRequest)
	}
	return obj.Packages, nil
}

// WorkspacesOf reads root's package.json and returns its workspace patterns.
func WorkspacesOf(root fs.FS) ([]string, error) {
	data, err := fs.ReadFile(root, "package.json")
	if err != nil {
		return nil, err
	}
	return Workspaces(data)
}

// RegisterWorkspace adds glob to the `workspaces` of the manifest at file,
// keeping every other field and the key order. It does nothing when the glob is
// already listed.
func RegisterWorkspace(fsys fsx.FS, file, glob string) error {
	data, err := fsys.ReadFile(file)
	if err != nil {
		return err
	}
	out, changed, err := withWorkspace(data, glob)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	if err := fsys.WriteFileAtomic(file, out, 0o644); err != nil {
		return fmt.Errorf("update %s: %w", file, err)
	}
	return nil
}

type orderedField struct {
	key   string
	value json.RawMessage
}

// withWorkspace returns the manifest with glob in its workspaces.
func withWorkspace(data []byte, glob string) ([]byte, bool, error) {
	current, err := Workspaces(data)
	if err != nil {
		return nil, false, err
	}
	if slices.Contains(current, glob) {
		return nil, false, nil
	}
	fields, err := orderedFields(data)
	if err != nil {
		return nil, false, err
	}
	next, err := nextWorkspaces(fields, current, glob)
	if err != nil {
		return nil, false, err
	}
	replaced := false
	for i := range fields {
		if fields[i].key == "workspaces" {
			fields[i].value, replaced = next, true
		}
	}
	if !replaced {
		fields = append(fields, orderedField{"workspaces", next})
	}
	out, err := renderFields(fields)
	return out, true, err
}

// nextWorkspaces renders the new value: an object keeps its other fields.
func nextWorkspaces(fields []orderedField, current []string, glob string) (json.RawMessage, error) {
	list := append(slices.Clone(current), glob)
	for _, f := range fields {
		if f.key != "workspaces" || bytes.HasPrefix(bytes.TrimSpace(f.value), []byte("[")) {
			continue
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(f.value, &obj); err != nil {
			return nil, err
		}
		packages, err := json.Marshal(list)
		if err != nil {
			return nil, err
		}
		obj["packages"] = packages
		return json.Marshal(obj)
	}
	return json.Marshal(list)
}

// orderedFields splits a JSON object into its top-level fields, in file order.
func orderedFields(data []byte) ([]orderedField, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, fmt.Errorf("%w: package.json is not a JSON object", core.ErrInvalidRequest)
	}
	var fields []orderedField
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("%w: package.json: %v", core.ErrInvalidRequest, err)
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, fmt.Errorf("%w: package.json: %v", core.ErrInvalidRequest, err)
		}
		fields = append(fields, orderedField{keyTok.(string), raw})
	}
	return fields, nil
}

// renderFields writes the fields as a two-space indented object.
func renderFields(fields []orderedField) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("{\n")
	for i, f := range fields {
		key, err := json.Marshal(f.key)
		if err != nil {
			return nil, err
		}
		var value bytes.Buffer
		if err := json.Indent(&value, f.value, "  ", "  "); err != nil {
			return nil, err
		}
		buf.WriteString("  ")
		buf.Write(key)
		buf.WriteString(": ")
		buf.Write(value.Bytes())
		if i < len(fields)-1 {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	buf.WriteString("}\n")
	return buf.Bytes(), nil
}
