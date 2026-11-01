package python

import (
	"io"
	"io/fs"
	"path"
	"regexp"
	"strings"
)

// scriptBlock matches the opening line of a PEP 723 inline metadata block.
var scriptBlock = regexp.MustCompile(`(?m)^# /// script[ \t]*\r?$`)

// maxScriptRead bounds how much of each file is inspected for the block.
const maxScriptRead = 64 << 10

// HasScriptBlock reports whether source contains a PEP 723 script block.
func HasScriptBlock(source []byte) bool { return scriptBlock.Match(source) }

// ScriptFiles lists the top-level *.py files in fsys that carry a PEP 723
// block, sorted by name.
func ScriptFiles(fsys fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".py") {
			continue
		}
		f, err := fsys.Open(path.Clean(e.Name()))
		if err != nil {
			continue
		}
		head, err := io.ReadAll(io.LimitReader(f, maxScriptRead))
		_ = f.Close()
		if err == nil && HasScriptBlock(head) {
			out = append(out, e.Name())
		}
	}
	return out, nil
}
