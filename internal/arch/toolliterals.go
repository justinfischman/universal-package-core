package arch

import (
	"fmt"
	"regexp"
	"strings"
)

// ToolOwners maps a tool name used as a string literal to the stack package
// that alone may spell it (ADR-0016): `just` and the `docker compose` pair.
var ToolOwners = map[string]string{
	"just":    "internal/stacks/just",
	"docker":  "internal/stacks/compose",
	"compose": "internal/stacks/compose",
}

// CheckToolLiterals reports every non-test Go file that uses a tool name as a
// quoted literal outside the tree that owns it. Prose in comments and docs is
// not matched, so the common word "just" stays usable. Test files and
// internal/arch are exempt.
func CheckToolLiterals(root string, owners map[string]string) ([]string, error) {
	res := make(map[string]*regexp.Regexp, len(owners))
	for tool := range owners {
		res[tool] = regexp.MustCompile("[\"`]" + regexp.QuoteMeta(tool) + "[\"`]")
	}
	var out []string
	err := walkSource(root, func(relPath string, data []byte) {
		if !strings.HasSuffix(relPath, ".go") || strings.HasSuffix(relPath, "_test.go") || under(relPath, "internal/arch") {
			return
		}
		for tool, owner := range owners {
			if under(relPath, owner) {
				continue
			}
			if res[tool].Match(data) {
				out = append(out, fmt.Sprintf("%s uses tool name %q outside %s", relPath, tool, owner))
			}
		}
	})
	return out, err
}
