package document

import (
	"fmt"
	"strings"
)

// flowParser reads one single-line YAML flow collection such as [a, b] or
// {a: 1, b: [x]}.
type flowParser struct {
	s string
	i int
}

func (f *flowParser) skipSpace() {
	for f.i < len(f.s) && f.s[f.i] == ' ' {
		f.i++
	}
}

func (f *flowParser) value() (any, error) {
	f.skipSpace()
	if f.i >= len(f.s) {
		return nil, fmt.Errorf("unterminated flow collection")
	}
	switch f.s[f.i] {
	case '[':
		return f.sequence()
	case '{':
		return f.mapping()
	case '"', '\'':
		end := closingQuote(f.s[f.i:])
		if end < 0 {
			return nil, fmt.Errorf("unterminated quoted string")
		}
		tok := f.s[f.i : f.i+end+1]
		f.i += end + 1
		return scalar(tok)
	}
	return scalar(f.plain(",]}"))
}

// plain consumes a plain token up to any stop byte and trims it.
func (f *flowParser) plain(stops string) string {
	start := f.i
	for f.i < len(f.s) && !strings.ContainsRune(stops, rune(f.s[f.i])) {
		f.i++
	}
	tok := strings.TrimSpace(f.s[start:f.i])
	if tok == "" {
		return "null"
	}
	return tok
}

func (f *flowParser) sequence() (any, error) {
	f.i++ // [
	out := []any{}
	for {
		f.skipSpace()
		if f.i < len(f.s) && f.s[f.i] == ']' {
			f.i++
			return out, nil
		}
		v, err := f.value()
		if err != nil {
			return nil, err
		}
		out = append(out, v)
		if err := f.separator(']'); err != nil {
			return nil, err
		}
		if f.s[f.i-1] == ']' {
			return out, nil
		}
	}
}

func (f *flowParser) mapping() (any, error) {
	f.i++ // {
	out := Map{}
	for {
		f.skipSpace()
		if f.i < len(f.s) && f.s[f.i] == '}' {
			f.i++
			return out, nil
		}
		var key string
		if f.i < len(f.s) && (f.s[f.i] == '"' || f.s[f.i] == '\'') {
			k, err := f.value()
			if err != nil {
				return nil, err
			}
			key = fmt.Sprint(k)
		} else {
			key = f.plain(":,}")
		}
		f.skipSpace()
		var v any
		if f.i < len(f.s) && f.s[f.i] == ':' {
			f.i++
			var err error
			if v, err = f.value(); err != nil {
				return nil, err
			}
		}
		out[key] = v
		if err := f.separator('}'); err != nil {
			return nil, err
		}
		if f.s[f.i-1] == '}' {
			return out, nil
		}
	}
}

// separator consumes a "," or the closing byte.
func (f *flowParser) separator(closer byte) error {
	f.skipSpace()
	if f.i >= len(f.s) {
		return fmt.Errorf("unterminated flow collection")
	}
	if c := f.s[f.i]; c == ',' || c == closer {
		f.i++
		return nil
	}
	return fmt.Errorf("expected ',' or '%c' at column %d", closer, f.i+1)
}
