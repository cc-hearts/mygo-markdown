package theme

import (
	"math"
	"strings"
)

// vparser reads the tokens of a value.
type vparser struct {
	ts []token
	i  int
}

func (p *vparser) skipWS() {
	for p.i < len(p.ts) && p.ts[p.i].kind == tWhitespace {
		p.i++
	}
}

// done reports whether nothing but whitespace is left.
func (p *vparser) done() bool {
	p.skipWS()
	return p.i >= len(p.ts)
}

// peek returns the next token that is not whitespace.
func (p *vparser) peek() token {
	p.skipWS()
	if p.i < len(p.ts) {
		return p.ts[p.i]
	}
	return token{kind: tEOF}
}

// next reads the next token that is not whitespace.
func (p *vparser) next() token {
	t := p.peek()
	if p.i < len(p.ts) {
		p.i++
	}
	return t
}

// args reads the arguments of the function just read, up to its ")",
// split at commas.
func (p *vparser) args() ([][]token, bool) {
	end := closing(p.ts, p.i)
	if end < 0 {
		return nil, false
	}
	in := p.ts[p.i:end]
	p.i = end + 1
	return splitTop(in, tComma), true
}

// hasVar reports whether tokens hold var().
func hasVar(ts []token) bool {
	for _, t := range ts {
		if t.kind == tFunction && t.val == "var" {
			return true
		}
	}
	return false
}

// maxSubstituted is the most tokens a value may come to once its var()s
// are replaced, as browsers cap it: variables that double each other
// would otherwise grow without bound.
const maxSubstituted = 4096

// substitute replaces each var() of ts by the value lookup gives its
// variable, or by its fallback. It fails on a var() of no variable and no
// fallback, which makes the declaration invalid where it is computed, and
// on a value that grows past maxSubstituted.
func substitute(ts []token, lookup func(string) ([]token, bool)) ([]token, bool) {
	if !hasVar(ts) {
		return ts, true
	}
	out := make([]token, 0, len(ts))
	for i := 0; i < len(ts); i++ {
		t := ts[i]
		if t.kind != tFunction || t.val != "var" {
			out = append(out, t)
			continue
		}
		end := closing(ts, i+1)
		if end < 0 {
			return nil, false
		}
		args := ts[i+1 : end]
		i = end
		name, fallback, hasFallback := varArgs(args)
		if name == "" {
			return nil, false
		}
		if v, ok := lookup(name); ok {
			if len(out)+len(v) > maxSubstituted {
				return nil, false
			}
			out = append(out, v...)
			continue
		}
		if !hasFallback {
			return nil, false
		}
		fb, ok := substitute(fallback, lookup)
		if !ok || len(out)+len(fb) > maxSubstituted {
			return nil, false
		}
		out = append(out, fb...)
	}
	return out, true
}

// varArgs reads the arguments of var(--name, fallback).
func varArgs(args []token) (name string, fallback []token, hasFallback bool) {
	parts := trimWS(args)
	if len(parts) == 0 || parts[0].kind != tIdent || !strings.HasPrefix(parts[0].val, "--") {
		return "", nil, false
	}
	name = parts[0].val
	rest := trimWS(parts[1:])
	if len(rest) == 0 {
		return name, nil, false
	}
	if rest[0].kind != tComma {
		return "", nil, false
	}
	return name, trimWS(rest[1:]), true
}

// length is a length or a number as written: a value and its unit, or
// calc(), min(), max() or clamp() of others, evaluated once ems are known.
type length struct {
	// op is 0 for a value; '+', '-', '*' or '/' for arithmetic; 'n' for
	// min(), 'x' for max() and 'c' for clamp().
	op   byte
	num  float64
	unit string // "px", "em", "rem", "%", or "" for a number
	args []length
}

// lenCtx is what lengths are relative to: the em, the rem, and what 100%
// is.
type lenCtx struct{ em, rem, pct float64 }

func (l *length) eval(c lenCtx) float64 {
	switch l.op {
	case 0:
		switch l.unit {
		case "em":
			return l.num * c.em
		case "rem":
			return l.num * c.rem
		case "%":
			return l.num / 100 * c.pct
		}
		return l.num
	case '+':
		return l.args[0].eval(c) + l.args[1].eval(c)
	case '-':
		return l.args[0].eval(c) - l.args[1].eval(c)
	case '*':
		return l.args[0].eval(c) * l.args[1].eval(c)
	case '/':
		d := l.args[1].eval(c)
		if d == 0 {
			return 0
		}
		return l.args[0].eval(c) / d
	case 'n', 'x':
		v := l.args[0].eval(c)
		for i := 1; i < len(l.args); i++ {
			if l.op == 'n' {
				v = math.Min(v, l.args[i].eval(c))
			} else {
				v = math.Max(v, l.args[i].eval(c))
			}
		}
		return v
	case 'c':
		return math.Max(l.args[0].eval(c), math.Min(l.args[1].eval(c), l.args[2].eval(c)))
	}
	return 0
}

// numeric reports whether the length is a number, without a unit.
func (l *length) numeric() bool {
	switch l.op {
	case 0:
		return l.unit == ""
	case '*':
		return l.args[0].numeric() && l.args[1].numeric()
	case '/':
		return l.args[0].numeric()
	}
	return l.args[0].numeric()
}

// lenUnits are the units of lengths.
var lenUnits = map[string]bool{"px": true, "em": true, "rem": true}

// parseLength reads a length, a percentage when pct, or a number when num:
// a number of no unit, as line-height takes.
func parseLength(p *vparser, pct, num bool) (length, bool) {
	l, ok := lengthTerm(p, pct)
	if !ok {
		return l, false
	}
	if l.numeric() && !num && !(l.op == 0 && l.num == 0) {
		return l, false
	}
	return l, true
}

// lengthTerm reads a value, or a function of lengths.
func lengthTerm(p *vparser, pct bool) (length, bool) {
	t := p.next()
	switch t.kind {
	case tNumber:
		return length{num: t.num}, true
	case tDimension:
		if !lenUnits[t.val] {
			return length{}, false
		}
		return length{num: t.num, unit: t.val}, true
	case tPercentage:
		return length{num: t.num, unit: "%"}, pct
	case tFunction:
		switch t.val {
		case "calc":
			end := closing(p.ts, p.i)
			if end < 0 {
				return length{}, false
			}
			q := &vparser{ts: p.ts[p.i:end]}
			p.i = end + 1
			l, ok := calcSum(q, pct)
			return l, ok && q.done()
		case "min", "max", "clamp":
			args, ok := p.args()
			if !ok || t.val == "clamp" && len(args) != 3 {
				return length{}, false
			}
			op := map[string]byte{"min": 'n', "max": 'x', "clamp": 'c'}[t.val]
			out := length{op: op}
			for _, a := range args {
				q := &vparser{ts: a}
				l, ok := calcSum(q, pct)
				if !ok || !q.done() {
					return length{}, false
				}
				out.args = append(out.args, l)
			}
			return out, true
		}
	case tLParen:
		end := closing(p.ts, p.i)
		if end < 0 {
			return length{}, false
		}
		q := &vparser{ts: p.ts[p.i:end]}
		p.i = end + 1
		l, ok := calcSum(q, pct)
		return l, ok && q.done()
	}
	return length{}, false
}

// calcSum reads a sum of calc(): products joined by + and -, which CSS
// wants spaces around.
func calcSum(p *vparser, pct bool) (length, bool) {
	l, ok := calcProduct(p, pct)
	for ok {
		t := p.peek()
		if !t.is('+') && !t.is('-') {
			break
		}
		p.next()
		r, rok := calcProduct(p, pct)
		if !rok || l.numeric() != r.numeric() {
			return length{}, false
		}
		l = length{op: t.val[0], args: []length{l, r}}
	}
	return l, ok
}

// calcProduct reads terms joined by * and /.
func calcProduct(p *vparser, pct bool) (length, bool) {
	l, ok := lengthTerm(p, pct)
	for ok {
		t := p.peek()
		if !t.is('*') && !t.is('/') {
			break
		}
		p.next()
		r, rok := lengthTerm(p, pct)
		if !rok || t.is('*') && !l.numeric() && !r.numeric() || t.is('/') && !r.numeric() {
			return length{}, false
		}
		l = length{op: t.val[0], args: []length{l, r}}
	}
	return l, ok
}
