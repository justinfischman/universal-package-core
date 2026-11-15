package document

import (
	"fmt"
	"strings"
)

// ParseYAML decodes the YAML subset that API contracts use: block mappings and
// sequences, single-line flow collections, plain/quoted scalars and literal or
// folded block scalars. Anchors, aliases, tags, multi-line plain scalars, tabs
// for indentation and multiple documents are rejected with an error rather
// than misread, because a lock must never be built from a wrong parse.
func ParseYAML(data []byte) (any, error) {
	p := &yamlParser{}
	for i, raw := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		l, err := tokenize(raw, i+1)
		if err != nil {
			return nil, err
		}
		p.lines = append(p.lines, l)
	}
	p.skipBlank()
	if p.eof() {
		return nil, fmt.Errorf("yaml: document is empty")
	}
	v, err := p.block(p.cur().indent)
	if err != nil {
		return nil, err
	}
	p.skipBlank()
	if !p.eof() {
		return nil, p.errf("unexpected content after the document root")
	}
	return v, nil
}

type yamlLine struct {
	no     int
	indent int
	text   string // comment stripped, trimmed
	raw    string
}

func (l yamlLine) blank() bool { return l.text == "" }

func (l yamlLine) seqItem() bool { return l.text == "-" || strings.HasPrefix(l.text, "- ") }

func tokenize(raw string, no int) (yamlLine, error) {
	indent := len(raw) - len(strings.TrimLeft(raw, " "))
	rest := raw[indent:]
	if strings.HasPrefix(rest, "\t") {
		return yamlLine{}, fmt.Errorf("yaml line %d: tabs are not allowed for indentation", no)
	}
	text := strings.TrimRight(stripComment(rest), " \t")
	if text == "---" || text == "..." {
		if no > 1 && text == "---" {
			return yamlLine{}, fmt.Errorf("yaml line %d: multiple documents are not supported", no)
		}
		text = ""
	}
	return yamlLine{no: no, indent: indent, text: text, raw: raw}, nil
}

// stripComment removes a trailing "# comment" that is outside quotes.
func stripComment(s string) string {
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote == '"' && c == '\\':
			i++
		case quote == '\'' && c == '\'' && i+1 < len(s) && s[i+1] == '\'':
			i++
		case quote != 0 && c == quote:
			quote = 0
		case quote == 0 && (c == '"' || c == '\'') && (i == 0 || strings.IndexByte(" ,[{:", s[i-1]) >= 0):
			quote = c
		case quote == 0 && c == '#' && (i == 0 || s[i-1] == ' ' || s[i-1] == '\t'):
			return s[:i]
		}
	}
	return s
}

type yamlParser struct {
	lines []yamlLine
	pos   int
}

func (p *yamlParser) eof() bool     { return p.pos >= len(p.lines) }
func (p *yamlParser) cur() yamlLine { return p.lines[p.pos] }

func (p *yamlParser) skipBlank() {
	for !p.eof() && p.cur().blank() {
		p.pos++
	}
}

func (p *yamlParser) errf(format string, args ...any) error {
	no := len(p.lines)
	if !p.eof() {
		no = p.cur().no
	}
	return fmt.Errorf("yaml line %d: %s", no, fmt.Sprintf(format, args...))
}

// block parses the block starting at the current line, whose indent is indent.
func (p *yamlParser) block(indent int) (any, error) {
	p.skipBlank()
	if p.eof() || p.cur().indent < indent {
		return nil, nil
	}
	l := p.cur()
	if l.seqItem() {
		return p.sequence(l.indent)
	}
	if _, _, ok := splitKey(l.text); ok {
		return p.mapping(l.indent)
	}
	p.pos++
	return p.inline(l.text, l.indent)
}

func (p *yamlParser) sequence(indent int) (any, error) {
	out := []any{}
	for {
		p.skipBlank()
		if p.eof() || p.cur().indent < indent || !p.cur().seqItem() {
			if !p.eof() && p.cur().indent > indent {
				return nil, p.errf("unexpected indentation")
			}
			return out, nil
		}
		if p.cur().indent > indent {
			return nil, p.errf("unexpected indentation")
		}
		l := p.cur()
		after := l.text[1:]
		item := strings.TrimLeft(after, " ")
		var v any
		var err error
		switch {
		case item == "":
			p.pos++
			v, err = p.block(indent + 1)
		case item[0] == '|' || item[0] == '>':
			p.pos++
			v, err = p.blockScalar(item, indent)
		default:
			// Re-read the rest of the line as a block nested at the item's column.
			col := indent + 1 + (len(after) - len(item))
			p.lines[p.pos] = yamlLine{no: l.no, indent: col, text: item, raw: l.raw}
			v, err = p.block(col)
		}
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
}

func (p *yamlParser) mapping(indent int) (any, error) {
	out := Map{}
	for {
		p.skipBlank()
		if p.eof() || p.cur().indent < indent {
			return out, nil
		}
		l := p.cur()
		if l.indent > indent {
			return nil, p.errf("unexpected indentation")
		}
		if l.seqItem() {
			return out, nil
		}
		key, rest, ok := splitKey(l.text)
		if !ok {
			return nil, p.errf("expected \"key: value\", multi-line plain scalars are not supported")
		}
		if _, dup := out[key]; dup {
			return nil, p.errf("duplicate key %q", key)
		}
		p.pos++
		var v any
		var err error
		if rest == "" {
			p.skipBlank()
			switch {
			case p.eof():
			case p.cur().indent > indent:
				v, err = p.block(p.cur().indent)
			case p.cur().indent == indent && p.cur().seqItem():
				v, err = p.sequence(indent)
			}
		} else {
			v, err = p.inline(rest, indent)
		}
		if err != nil {
			return nil, err
		}
		out[key] = v
	}
}

// inline parses a value written on the same line as its key or dash.
func (p *yamlParser) inline(s string, parentIndent int) (any, error) {
	switch s[0] {
	case '&', '*', '!':
		return nil, p.errf("anchors, aliases and tags are not supported")
	case '|', '>':
		return p.blockScalar(s, parentIndent)
	case '[', '{':
		f := &flowParser{s: s}
		v, err := f.value()
		if err == nil {
			f.skipSpace()
			if f.i != len(f.s) {
				err = fmt.Errorf("unexpected text after flow collection")
			}
		}
		if err != nil {
			return nil, p.errf("%v (flow collections must fit on one line)", err)
		}
		return v, nil
	}
	v, err := scalar(s)
	if err != nil {
		return nil, p.errf("%v", err)
	}
	return v, nil
}
