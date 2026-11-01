package python

import (
	"regexp"
	"strings"
)

var (
	requirementNameRe = regexp.MustCompile(`^\s*([A-Za-z0-9][A-Za-z0-9._-]*)`)
	separatorRe       = regexp.MustCompile(`[-_.]+`)
)

// NormalizeName applies the PEP 503 normalization to a distribution name.
func NormalizeName(name string) string {
	return separatorRe.ReplaceAllString(strings.ToLower(name), "-")
}

// RequirementName returns the normalized distribution name at the start of a
// requirement specifier ("Rich[jupyter]>=13" -> "rich"), or "" if there is none.
func RequirementName(spec string) string {
	m := requirementNameRe.FindStringSubmatch(spec)
	if m == nil {
		return ""
	}
	return NormalizeName(m[1])
}
