package document

import (
	"encoding/json"
	"fmt"
	"strings"
)

// blockScalar reads the more-indented lines after a "|" or ">" header.
func (p *yamlParser) blockScalar(header string, parentIndent int) (any, error) {
	fold := header[0] == '>'
	var body []string
	blockIndent := -1
	for !p.eof() {
		l := p.cur()
		if !l.blank() || strings.TrimSpace(l.raw) != "" {
			if l.indent <= parentIndent {
				break
			}
			if blockIndent < 0 {
				blockIndent = l.indent
			}
		}
		if strings.TrimSpace(l.raw) == "" {
			body = append(body, "")
		} else {
			body = append(body, l.raw[min(blockIndent, l.indent):])
		}
		p.pos++
	}
	for len(body) > 0 && body[len(body)-1] == "" {
		body = body[:len(body)-1]
	}
	text := strings.Join(body, "\n")
	if fold {
		paras := strings.Split(text, "\n\n")
		for i, para := range paras {
			paras[i] = strings.ReplaceAll(para, "\n", " ")
		}
		text = strings.Join(paras, "\n")
	}
	if !strings.Contains(header, "-") && text != "" {
		text += "\n"
	}
	return text, nil
}

// splitKey splits "key: value" into its parts.
func splitKey(text string) (key, rest string, ok bool) {
	if text == "" || text[0] == '[' || text[0] == '{' || text[0] == '|' || text[0] == '>' {
		return "", "", false
	}
	if text[0] == '"' || text[0] == '\'' {
		end := closingQuote(text)
		if end < 0 || end+1 >= len(text) || text[end+1] != ':' || (end+2 < len(text) && text[end+2] != ' ') {
			return "", "", false
		}
		k, err := scalar(text[:end+1])
		if err != nil {
			return "", "", false
		}
		return fmt.Sprint(k), strings.TrimSpace(text[end+2:]), true
	}
	if strings.HasSuffix(text, ":") {
		return strings.TrimSpace(text[:len(text)-1]), "", true
	}
	if i := strings.Index(text, ": "); i >= 0 {
		return strings.TrimSpace(text[:i]), strings.TrimSpace(text[i+2:]), true
	}
	return "", "", false
}

// closingQuote returns the index of the quote closing the one at s[0], or -1.
func closingQuote(s string) int {
	q := s[0]
	for i := 1; i < len(s); i++ {
		switch {
		case q == '"' && s[i] == '\\':
			i++
		case s[i] == q && q == '\'' && i+1 < len(s) && s[i+1] == '\'':
			i++
		case s[i] == q:
			return i
		}
	}
	return -1
}

// scalar converts a plain or quoted scalar. Quoted scalars are always strings.
func scalar(s string) (any, error) {
	switch s[0] {
	case '"':
		var out string
		if closingQuote(s) != len(s)-1 || json.Unmarshal([]byte(s), &out) != nil {
			return nil, fmt.Errorf("invalid double-quoted string %s", s)
		}
		return out, nil
	case '\'':
		if closingQuote(s) != len(s)-1 {
			return nil, fmt.Errorf("invalid single-quoted string %s", s)
		}
		return strings.ReplaceAll(s[1:len(s)-1], "''", "'"), nil
	}
	switch s {
	case "null", "Null", "NULL", "~":
		return nil, nil
	case "true", "True", "TRUE":
		return true, nil
	case "false", "False", "FALSE":
		return false, nil
	}
	if (s[0] == '-' || (s[0] >= '0' && s[0] <= '9')) && json.Valid([]byte(s)) {
		return json.Number(s), nil
	}
	return s, nil
}
