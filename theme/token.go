package theme

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// tokKind is what a token of CSS is.
type tokKind uint8

const (
	tEOF tokKind = iota
	tIdent
	// tFunction is a name followed by "(": the name is its val.
	tFunction
	// tAtKeyword is "@" and a name: the name is its val.
	tAtKeyword
	// tHash is "#" and a name: the name is its val.
	tHash
	// tString is a quoted string: its value, unescaped, is its val.
	tString
	// tBadString is a string a newline ended.
	tBadString
	// tURL is url(…) with an unquoted address, its val.
	tURL
	tBadURL
	// tDelim is a character no other token is: its val.
	tDelim
	tNumber
	tPercentage
	// tDimension is a number with a unit, its val.
	tDimension
	tWhitespace
	tColon
	tSemicolon
	tComma
	tLBracket
	tRBracket
	tLParen
	tRParen
	tLBrace
	tRBrace
	// tCDO and tCDC are <!-- and -->, which a sheet may hold at its top.
	tCDO
	tCDC
)

// token is a token of CSS: what it is, its name or text, its number, and
// where it is in the source, from pos to end.
type token struct {
	kind tokKind
	val  string
	num  float64
	pos  int
	end  int
}

// is reports whether the token is the delimiter c.
func (t token) is(c byte) bool { return t.kind == tDelim && len(t.val) == 1 && t.val[0] == c }

// isIdent reports whether the token is the identifier name, of any case.
func (t token) isIdent(name string) bool {
	return t.kind == tIdent && strings.EqualFold(t.val, name)
}

// text returns the token as CSS writes it.
func (t token) text() string {
	switch t.kind {
	case tIdent:
		return t.val
	case tFunction:
		return t.val + "("
	case tAtKeyword:
		return "@" + t.val
	case tHash:
		return "#" + t.val
	case tString, tBadString:
		return strconv.Quote(t.val)
	case tURL:
		return "url(" + t.val + ")"
	case tDelim:
		return t.val
	case tNumber:
		return formatNum(t.num)
	case tPercentage:
		return formatNum(t.num) + "%"
	case tDimension:
		return formatNum(t.num) + t.val
	case tWhitespace:
		return " "
	case tColon:
		return ":"
	case tSemicolon:
		return ";"
	case tComma:
		return ","
	case tLBracket:
		return "["
	case tRBracket:
		return "]"
	case tLParen:
		return "("
	case tRParen:
		return ")"
	case tLBrace:
		return "{"
	case tRBrace:
		return "}"
	case tCDO:
		return "<!--"
	case tCDC:
		return "-->"
	}
	return ""
}

func formatNum(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

// tokensText returns tokens as CSS writes them.
func tokensText(ts []token) string {
	var sb strings.Builder
	for _, t := range ts {
		sb.WriteString(t.text())
	}
	return strings.TrimSpace(sb.String())
}

// lexer reads the tokens of a source.
type lexer struct {
	src  string
	p    int
	diag *diagnostics
}

// tokenize returns the tokens of src, without its comments, and reports
// what is malformed in it to d.
func tokenize(src string, d *diagnostics) []token {
	l := &lexer{src: src, diag: d}
	var out []token
	for {
		t := l.next()
		if t.kind == tEOF {
			return out
		}
		out = append(out, t)
	}
}

// at returns the byte at p+k, or 0 past the end.
func (l *lexer) at(k int) byte {
	if l.p+k < len(l.src) {
		return l.src[l.p+k]
	}
	return 0
}

func isNameStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || c >= 0x80
}

func isName(c byte) bool { return isNameStart(c) || isDigit(c) || c == '-' }

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isHex(c byte) bool { return isDigit(c) || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' }

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' }

// validEscape reports whether the bytes at p+k start an escape.
func (l *lexer) validEscape(k int) bool {
	return l.at(k) == '\\' && l.at(k+1) != '\n' && l.at(k+1) != '\r' && l.at(k+1) != '\f' && l.p+k+1 < len(l.src)
}

// startsIdent reports whether the bytes at p+k start an identifier.
func (l *lexer) startsIdent(k int) bool {
	switch c := l.at(k); {
	case c == '-':
		return isNameStart(l.at(k+1)) || l.at(k+1) == '-' || l.validEscape(k+1)
	case isNameStart(c):
		return true
	case c == '\\':
		return l.validEscape(k)
	}
	return false
}

// startsNumber reports whether the bytes at p start a number.
func (l *lexer) startsNumber() bool {
	c := l.at(0)
	switch {
	case c == '+' || c == '-':
		return isDigit(l.at(1)) || l.at(1) == '.' && isDigit(l.at(2))
	case c == '.':
		return isDigit(l.at(1))
	}
	return isDigit(c)
}

// skipComments skips the comments at p, reporting one left open.
func (l *lexer) skipComments() {
	for l.at(0) == '/' && l.at(1) == '*' {
		end := strings.Index(l.src[l.p+2:], "*/")
		if end < 0 {
			l.diag.add(Error, l.p, "unclosed comment")
			l.p = len(l.src)
			return
		}
		l.p += 2 + end + 2
	}
}

func (l *lexer) next() token {
	l.skipComments()
	start := l.p
	tok := func(k tokKind, val string) token { return token{kind: k, val: val, pos: start, end: l.p} }
	if l.p >= len(l.src) {
		return token{kind: tEOF, pos: l.p, end: l.p}
	}
	c := l.src[l.p]
	switch {
	case isSpace(c):
		for l.p < len(l.src) && isSpace(l.src[l.p]) {
			l.p++
		}
		return tok(tWhitespace, "")
	case c == '"' || c == '\'':
		return l.string(c)
	case c == '#':
		if isName(l.at(1)) || l.validEscape(1) {
			l.p++
			return tok(tHash, l.name())
		}
	case c == '(':
		l.p++
		return tok(tLParen, "")
	case c == ')':
		l.p++
		return tok(tRParen, "")
	case c == '[':
		l.p++
		return tok(tLBracket, "")
	case c == ']':
		l.p++
		return tok(tRBracket, "")
	case c == '{':
		l.p++
		return tok(tLBrace, "")
	case c == '}':
		l.p++
		return tok(tRBrace, "")
	case c == ',':
		l.p++
		return tok(tComma, "")
	case c == ':':
		l.p++
		return tok(tColon, "")
	case c == ';':
		l.p++
		return tok(tSemicolon, "")
	case c == '+' || c == '.':
		if l.startsNumber() {
			return l.numeric()
		}
	case c == '-':
		if l.startsNumber() {
			return l.numeric()
		}
		if l.at(1) == '-' && l.at(2) == '>' {
			l.p += 3
			return tok(tCDC, "")
		}
		if l.startsIdent(0) {
			return l.identLike()
		}
	case c == '<':
		if strings.HasPrefix(l.src[l.p:], "<!--") {
			l.p += 4
			return tok(tCDO, "")
		}
	case c == '@':
		if l.startsIdent(1) {
			l.p++
			return tok(tAtKeyword, l.name())
		}
	case c == '\\':
		if l.validEscape(0) {
			return l.identLike()
		}
		l.diag.add(Error, l.p, "invalid escape")
	case isDigit(c):
		return l.numeric()
	case isNameStart(c):
		return l.identLike()
	}
	_, n := utf8.DecodeRuneInString(l.src[l.p:])
	l.p += n
	return tok(tDelim, l.src[start:l.p])
}

// name reads a name at p, its escapes unescaped.
func (l *lexer) name() string {
	var sb strings.Builder
	for l.p < len(l.src) {
		c := l.src[l.p]
		switch {
		case isName(c):
			sb.WriteByte(c)
			l.p++
		case l.validEscape(0):
			l.p++
			sb.WriteRune(l.escape())
		default:
			return sb.String()
		}
	}
	return sb.String()
}

// escape reads an escape after its backslash.
func (l *lexer) escape() rune {
	if l.p >= len(l.src) {
		return utf8.RuneError
	}
	if isHex(l.src[l.p]) {
		n := 0
		for n < 6 && l.p+n < len(l.src) && isHex(l.src[l.p+n]) {
			n++
		}
		v, _ := strconv.ParseUint(l.src[l.p:l.p+n], 16, 32)
		l.p += n
		if l.p < len(l.src) && isSpace(l.src[l.p]) {
			if l.src[l.p] == '\r' && l.at(1) == '\n' {
				l.p++
			}
			l.p++
		}
		if v == 0 || v > utf8.MaxRune || v >= 0xD800 && v <= 0xDFFF {
			return utf8.RuneError
		}
		return rune(v)
	}
	r, n := utf8.DecodeRuneInString(l.src[l.p:])
	l.p += n
	return r
}

// string reads a string quoted by q.
func (l *lexer) string(q byte) token {
	start := l.p
	l.p++
	var sb strings.Builder
	for l.p < len(l.src) {
		c := l.src[l.p]
		switch {
		case c == q:
			l.p++
			return token{kind: tString, val: sb.String(), pos: start, end: l.p}
		case c == '\n' || c == '\r' || c == '\f':
			l.diag.add(Error, start, "unclosed string")
			return token{kind: tBadString, val: sb.String(), pos: start, end: l.p}
		case c == '\\':
			l.p++
			switch {
			case l.p >= len(l.src):
			case l.src[l.p] == '\n' || l.src[l.p] == '\f':
				l.p++
			case l.src[l.p] == '\r':
				l.p++
				if l.at(0) == '\n' {
					l.p++
				}
			default:
				sb.WriteRune(l.escape())
			}
		default:
			sb.WriteByte(c)
			l.p++
		}
	}
	l.diag.add(Error, start, "unclosed string")
	return token{kind: tString, val: sb.String(), pos: start, end: l.p}
}

// numeric reads a number, a percentage or a dimension.
func (l *lexer) numeric() token {
	start := l.p
	if c := l.at(0); c == '+' || c == '-' {
		l.p++
	}
	for isDigit(l.at(0)) {
		l.p++
	}
	if l.at(0) == '.' && isDigit(l.at(1)) {
		l.p++
		for isDigit(l.at(0)) {
			l.p++
		}
	}
	if c := l.at(0); (c == 'e' || c == 'E') && (isDigit(l.at(1)) || (l.at(1) == '+' || l.at(1) == '-') && isDigit(l.at(2))) {
		l.p += 2
		for isDigit(l.at(0)) {
			l.p++
		}
	}
	num, _ := strconv.ParseFloat(l.src[start:l.p], 64)
	switch {
	case l.startsIdent(0):
		unit := l.name()
		return token{kind: tDimension, val: strings.ToLower(unit), num: num, pos: start, end: l.p}
	case l.at(0) == '%':
		l.p++
		return token{kind: tPercentage, num: num, pos: start, end: l.p}
	}
	return token{kind: tNumber, num: num, pos: start, end: l.p}
}

// identLike reads an identifier, a function, or a url(…).
func (l *lexer) identLike() token {
	start := l.p
	name := l.name()
	if l.at(0) != '(' {
		return token{kind: tIdent, val: name, pos: start, end: l.p}
	}
	l.p++
	if strings.EqualFold(name, "url") {
		q := l.p
		for q < len(l.src) && isSpace(l.src[q]) {
			q++
		}
		if q >= len(l.src) || l.src[q] != '"' && l.src[q] != '\'' {
			l.p = q
			return l.url(start)
		}
	}
	return token{kind: tFunction, val: strings.ToLower(name), pos: start, end: l.p}
}

// url reads the unquoted address of a url( and its closing parenthesis.
func (l *lexer) url(start int) token {
	var sb strings.Builder
	for l.p < len(l.src) {
		c := l.src[l.p]
		switch {
		case c == ')':
			l.p++
			return token{kind: tURL, val: sb.String(), pos: start, end: l.p}
		case isSpace(c):
			for l.p < len(l.src) && isSpace(l.src[l.p]) {
				l.p++
			}
			if l.at(0) == ')' {
				l.p++
				return token{kind: tURL, val: sb.String(), pos: start, end: l.p}
			}
			return l.badURL(start)
		case c == '"' || c == '\'' || c == '(':
			return l.badURL(start)
		case c == '\\':
			if !l.validEscape(0) {
				return l.badURL(start)
			}
			l.p++
			sb.WriteRune(l.escape())
		default:
			sb.WriteByte(c)
			l.p++
		}
	}
	l.diag.add(Error, start, "unclosed url(")
	return token{kind: tURL, val: sb.String(), pos: start, end: l.p}
}

// badURL skips the rest of a malformed url(…).
func (l *lexer) badURL(start int) token {
	l.diag.add(Error, start, "malformed url(): quote an address with spaces, quotes or parentheses")
	for l.p < len(l.src) && l.src[l.p] != ')' {
		if l.validEscape(0) {
			l.p++
		}
		l.p++
	}
	if l.p < len(l.src) {
		l.p++
	}
	return token{kind: tBadURL, pos: start, end: l.p}
}
