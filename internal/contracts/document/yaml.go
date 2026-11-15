package document

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"time"

	"go.yaml.in/yaml/v3"
)

const maxDepth = 512

// ParseYAML decodes a single YAML document into generic values (Map, []any,
// string, bool, json.Number, nil). The library resolves anchors, aliases and
// merge keys and rejects duplicate mapping keys. Mapping keys must be strings
// (or scalars the library reads as strings); other key types are errors.
// Numbers take their canonical form, and timestamps and binary values become
// strings, so canonical output is deterministic. A second document and an
// empty document are errors.
func ParseYAML(data []byte) (any, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var raw any
	if err := dec.Decode(&raw); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("yaml: document is empty")
		}
		return nil, fmt.Errorf("yaml: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err == nil {
		return nil, fmt.Errorf("yaml: multiple documents are not supported")
	} else if !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("yaml: %w", err)
	}
	return normalize(raw, 0)
}

func normalize(v any, depth int) (any, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("yaml: nesting too deep")
	}
	switch x := v.(type) {
	case nil, bool, string:
		return x, nil
	case int:
		return json.Number(strconv.FormatInt(int64(x), 10)), nil
	case int64:
		return json.Number(strconv.FormatInt(x, 10)), nil
	case uint64:
		return json.Number(strconv.FormatUint(x, 10)), nil
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return strconv.FormatFloat(x, 'g', -1, 64), nil
		}
		return json.Number(strconv.FormatFloat(x, 'f', -1, 64)), nil
	case time.Time:
		return x.Format(time.RFC3339Nano), nil
	case []byte:
		return string(x), nil
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			n, err := normalize(item, depth+1)
			if err != nil {
				return nil, err
			}
			out[i] = n
		}
		return out, nil
	case map[string]any:
		out := make(Map, len(x))
		for k, item := range x {
			n, err := normalize(item, depth+1)
			if err != nil {
				return nil, err
			}
			out[k] = n
		}
		return out, nil
	case map[any]any:
		return nil, fmt.Errorf("yaml: mapping keys must be strings")
	}
	return nil, fmt.Errorf("yaml: unsupported value of type %T", v)
}
