// Package addons discovers, validates, and flattens Odoo addon directories.
package addons

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
)

// ParseManifest reads the Python-literal manifest file at path (typically
// __manifest__.py or __openerp__.py) and returns its top-level dict as a
// map[string]interface{}, mirroring encoding/json's value conventions
// (map[string]interface{}, []interface{}, string, float64, bool, nil).
//
// Only a restricted literal subset of Python is supported: dicts, lists,
// tuples (returned as []interface{}, same as lists), strings (including
// adjacent-literal concatenation and triple-quoted multi-line strings),
// numbers, booleans, and None. Expressions (list `+`, function calls,
// variables) are not evaluated and are reported as parse errors.
func ParseManifest(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("addons: %s: invalid manifest: %w", path, err)
	}

	p := &manifestParser{src: []rune(string(data)), path: path, line: 1, col: 1}
	p.skipSpace()
	value, err := p.parseValue()
	if err != nil {
		return nil, err
	}
	p.skipSpace()
	if !p.atEOF() {
		return nil, p.errorf("trailing content after top-level value")
	}

	dict, ok := value.(map[string]interface{})
	if !ok {
		return nil, p.errorf("top-level value must be a dict")
	}
	return dict, nil
}

type manifestParser struct {
	src  []rune
	pos  int
	line int
	col  int
	path string
}

func (p *manifestParser) errorf(format string, args ...interface{}) error {
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("addons: %s: invalid manifest: %d:%d: %s", p.path, p.line, p.col, msg)
}

func (p *manifestParser) atEOF() bool {
	return p.pos >= len(p.src)
}

func (p *manifestParser) peek() rune {
	if p.atEOF() {
		return 0
	}
	return p.src[p.pos]
}

func (p *manifestParser) advance() rune {
	r := p.src[p.pos]
	p.pos++
	if r == '\n' {
		p.line++
		p.col = 1
	} else {
		p.col++
	}
	return r
}

func (p *manifestParser) skipSpace() {
	for !p.atEOF() {
		r := p.peek()
		if r == '#' {
			for !p.atEOF() && p.peek() != '\n' {
				p.advance()
			}
			continue
		}
		if unicode.IsSpace(r) {
			p.advance()
			continue
		}
		break
	}
}

func (p *manifestParser) expect(r rune) error {
	if p.atEOF() || p.peek() != r {
		return p.errorf("expected %q", r)
	}
	p.advance()
	return nil
}

func (p *manifestParser) parseValue() (interface{}, error) {
	if p.atEOF() {
		return nil, p.errorf("unexpected end of file")
	}

	switch r := p.peek(); {
	case r == '{':
		return p.parseDict()
	case r == '[':
		return p.parseList()
	case r == '(':
		return p.parseTuple()
	case r == '\'' || r == '"' || p.hasStringPrefix():
		return p.parseStringConcat()
	case r == '-' || unicode.IsDigit(r):
		return p.parseNumber()
	case unicode.IsLetter(r) || r == '_':
		return p.parseKeyword()
	default:
		return nil, p.errorf("unexpected character %q", r)
	}
}

// hasStringPrefix reports whether the parser is positioned at a Python
// string-literal prefix (r, b, u, f, or a two-letter combination such as
// rb/br/fr — case-insensitive) immediately followed by a quote character,
// e.g. the leading r before a raw triple-quoted string. Real-world
// manifests use such r-prefixed strings for `description` fields
// containing backslashes (regex-like text, Windows-style paths) that
// would otherwise need escaping.
func (p *manifestParser) hasStringPrefix() bool {
	n, _ := p.stringPrefix()
	return n > 0
}

// stringPrefix returns the length of a string-literal prefix at the
// parser's current position (0 if there is none) and whether it marks a
// raw string (contains 'r' or 'R'). It does not consume anything.
func (p *manifestParser) stringPrefix() (length int, isRaw bool) {
	i := p.pos
	for i < len(p.src) && length < 2 {
		c := unicode.ToLower(p.src[i])
		if c != 'r' && c != 'b' && c != 'u' && c != 'f' {
			break
		}
		if c == 'r' {
			isRaw = true
		}
		i++
		length++
	}
	if length == 0 || i >= len(p.src) || (p.src[i] != '\'' && p.src[i] != '"') {
		return 0, false
	}
	return length, isRaw
}

func (p *manifestParser) parseDict() (map[string]interface{}, error) {
	if err := p.expect('{'); err != nil {
		return nil, err
	}
	dict := map[string]interface{}{}

	p.skipSpace()
	for !p.atEOF() && p.peek() != '}' {
		key, err := p.parseStringConcat()
		if err != nil {
			return nil, err
		}

		p.skipSpace()
		if err := p.expect(':'); err != nil {
			return nil, err
		}

		p.skipSpace()
		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		dict[key] = value

		p.skipSpace()
		if p.atEOF() {
			return nil, p.errorf("unterminated dict")
		}
		if p.peek() == ',' {
			p.advance()
			p.skipSpace()
			continue
		}
		break
	}

	if err := p.expect('}'); err != nil {
		return nil, p.errorf("unterminated dict")
	}
	return dict, nil
}

func (p *manifestParser) parseList() ([]interface{}, error) {
	if err := p.expect('['); err != nil {
		return nil, err
	}
	list := []interface{}{}

	p.skipSpace()
	for !p.atEOF() && p.peek() != ']' {
		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		list = append(list, value)

		p.skipSpace()
		if p.atEOF() {
			return nil, p.errorf("unterminated list")
		}
		if p.peek() == ',' {
			p.advance()
			p.skipSpace()
			continue
		}
		break
	}

	if err := p.expect(']'); err != nil {
		return nil, p.errorf("unterminated list")
	}
	return list, nil
}

// parseTuple parses a Python tuple literal, e.g. ('remove', 'path') — used
// inside 'assets' entries to mark an asset for removal. Tuples have no
// dedicated Go/JSON representation, so (mirroring parseList) they are
// returned as a plain []interface{}; callers that care about the
// distinction must inspect the manifest's own field conventions.
func (p *manifestParser) parseTuple() ([]interface{}, error) {
	if err := p.expect('('); err != nil {
		return nil, err
	}
	tuple := []interface{}{}

	p.skipSpace()
	for !p.atEOF() && p.peek() != ')' {
		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		tuple = append(tuple, value)

		p.skipSpace()
		if p.atEOF() {
			return nil, p.errorf("unterminated tuple")
		}
		if p.peek() == ',' {
			p.advance()
			p.skipSpace()
			continue
		}
		break
	}

	if err := p.expect(')'); err != nil {
		return nil, p.errorf("unterminated tuple")
	}
	return tuple, nil
}

func (p *manifestParser) parseString() (string, error) {
	prefixLen, isRaw := p.stringPrefix()
	for i := 0; i < prefixLen; i++ {
		p.advance()
	}

	if p.atEOF() || (p.peek() != '\'' && p.peek() != '"') {
		return "", p.errorf("expected string")
	}

	if p.startsTripleQuote() {
		return p.parseTripleQuotedString()
	}

	quote := p.advance()

	var sb strings.Builder
	for {
		if p.atEOF() {
			return "", p.errorf("unterminated string")
		}
		r := p.advance()
		if r == quote {
			return sb.String(), nil
		}
		if r == '\n' {
			return "", p.errorf("unterminated string")
		}
		if r != '\\' {
			sb.WriteRune(r)
			continue
		}
		if isRaw {
			sb.WriteRune(r)
			continue
		}

		if p.atEOF() {
			return "", p.errorf("unterminated string")
		}
		esc := p.advance()
		switch esc {
		case '\\':
			sb.WriteRune('\\')
		case '\'':
			sb.WriteRune('\'')
		case '"':
			sb.WriteRune('"')
		case 'n':
			sb.WriteRune('\n')
		case '\n':
			// backslash-newline is a line continuation: it produces no
			// character, letting the string span multiple physical lines.
		default:
			return "", p.errorf("invalid escape sequence \\%c", esc)
		}
	}
}

// startsTripleQuote reports whether the parser is positioned at a Python
// triple-quote delimiter: a quote character repeated three times.
func (p *manifestParser) startsTripleQuote() bool {
	if p.pos+2 >= len(p.src) {
		return false
	}
	q := p.src[p.pos]
	return (q == '\'' || q == '"') && p.src[p.pos+1] == q && p.src[p.pos+2] == q
}

// parseTripleQuotedString parses a Python triple-quoted string (its quote
// character repeated three times, opening and closing), which — unlike the
// single/double-quoted form above — may contain literal newlines and a
// bare, unescaped instance of its own quote character. Real-world
// manifests use this form throughout for multi-line `description` fields.
// It ends at the next occurrence of the same triple-quote sequence; escape
// sequences are not processed, matching how these fields are used in
// practice (plain prose, never containing an escaped character).
func (p *manifestParser) parseTripleQuotedString() (string, error) {
	quote := p.peek()
	p.advance()
	p.advance()
	p.advance()

	var sb strings.Builder
	for {
		if p.atEOF() {
			return "", p.errorf("unterminated string")
		}
		if p.peek() == quote && p.pos+2 < len(p.src) && p.src[p.pos+1] == quote && p.src[p.pos+2] == quote {
			p.advance()
			p.advance()
			p.advance()
			return sb.String(), nil
		}
		sb.WriteRune(p.advance())
	}
}

// parseStringConcat parses a string literal followed by zero or more
// adjacent string literals, concatenating them — Python implicitly
// concatenates adjacent string literals (e.g. `"a" "b"` == `"ab"`), a
// pattern real-world manifests use to wrap long author/summary fields
// across several lines.
func (p *manifestParser) parseStringConcat() (string, error) {
	s, err := p.parseString()
	if err != nil {
		return "", err
	}

	for {
		savedPos, savedLine, savedCol := p.pos, p.line, p.col
		p.skipSpace()
		if p.peek() != '\'' && p.peek() != '"' {
			p.pos, p.line, p.col = savedPos, savedLine, savedCol
			return s, nil
		}

		next, err := p.parseString()
		if err != nil {
			return "", err
		}
		s += next
	}
}

func (p *manifestParser) parseNumber() (float64, error) {
	start := p.pos
	if p.peek() == '-' {
		p.advance()
	}
	if p.atEOF() || !unicode.IsDigit(p.peek()) {
		return 0, p.errorf("invalid number")
	}
	for !p.atEOF() && unicode.IsDigit(p.peek()) {
		p.advance()
	}
	if !p.atEOF() && p.peek() == '.' {
		p.advance()
		if p.atEOF() || !unicode.IsDigit(p.peek()) {
			return 0, p.errorf("invalid number")
		}
		for !p.atEOF() && unicode.IsDigit(p.peek()) {
			p.advance()
		}
	}

	text := string(p.src[start:p.pos])
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, p.errorf("invalid number %q", text)
	}
	return value, nil
}

func (p *manifestParser) parseKeyword() (interface{}, error) {
	start := p.pos
	for !p.atEOF() && (unicode.IsLetter(p.peek()) || unicode.IsDigit(p.peek()) || p.peek() == '_') {
		p.advance()
	}
	word := string(p.src[start:p.pos])

	switch word {
	case "True":
		return true, nil
	case "False":
		return false, nil
	case "None":
		return nil, nil
	default:
		return nil, p.errorf("unexpected identifier %q", word)
	}
}
