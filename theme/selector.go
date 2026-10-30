package theme

import (
	"fmt"
	"math/bits"
	"slices"
	"strings"
)

// selector is a complex selector: compound selectors joined by
// combinators, the rightmost last, which matches the element styled.
type selector struct {
	parts []compound
	// combs[i] joins parts[i-1] to parts[i]: ' ' for a descendant, '>'
	// for a child.
	combs []byte
	spec  specificity
}

// compound is what one element must be.
type compound struct {
	// typ is the element's type, "" for any.
	typ     string
	classes []string
	attrs   []attrSel
	states  States
	root    bool
	nots    []compound
	// pseudo is the pseudo-element of the rightmost compound, and pstates
	// the pseudo-classes after it.
	pseudo  string
	pstates States
}

// attrSel is an attribute selector.
type attrSel struct {
	name string
	// op is 0 for the attribute's presence, else the first character of
	// its operator: '=', '~', '^', '$' or '*'.
	op   byte
	val  string
	fold bool
}

// specificity counts a selector's classes, attributes and pseudo-classes,
// then its types and pseudo-elements: the more, the stronger.
type specificity [2]int

func (a specificity) less(b specificity) bool {
	if a[0] != b[0] {
		return a[0] < b[0]
	}
	return a[1] < b[1]
}

// selErr is why a selector is dropped.
type selErr struct {
	pos int
	sev Severity
	msg string
}

// parseSelectors reads a list of selectors. It drops those it cannot use
// and reports them, and warns of those that can match no node.
func parseSelectors(ts []token, d *diagnostics) []*selector {
	var out []*selector
	for _, part := range splitTop(ts, tComma) {
		part = trimWS(part)
		if len(part) == 0 {
			pos := 0
			if len(ts) > 0 {
				pos = ts[0].pos
			}
			d.add(Error, pos, "empty selector")
			continue
		}
		p := &selParser{ts: part, end: part[len(part)-1].end, warn: func(pos int, msg string) { d.add(Warning, pos, msg) }}
		s, err := p.selector()
		if err != nil {
			d.add(err.sev, err.pos, err.msg+": the selector is dropped")
			continue
		}
		out = append(out, s)
	}
	return out
}

// selParser reads a selector.
type selParser struct {
	ts   []token
	i    int
	end  int
	warn func(pos int, msg string)
}

func (p *selParser) peek() token {
	if p.i < len(p.ts) {
		return p.ts[p.i]
	}
	return token{kind: tEOF, pos: p.end, end: p.end}
}

func (p *selParser) skipWS() bool {
	had := false
	for p.i < len(p.ts) && p.ts[p.i].kind == tWhitespace {
		p.i++
		had = true
	}
	return had
}

func (p *selParser) selector() (*selector, *selErr) {
	s := &selector{}
	comb := byte(' ')
	for {
		c, err := p.compound(true)
		if err != nil {
			return nil, err
		}
		if n := len(s.parts); n > 0 && s.parts[n-1].pseudo != "" {
			return nil, &selErr{p.peek().pos, Error, "a pseudo-element must come last"}
		}
		s.parts = append(s.parts, c)
		s.combs = append(s.combs, comb)
		ws := p.skipWS()
		t := p.peek()
		switch {
		case t.kind == tEOF:
			for i := range s.parts {
				sp := s.parts[i].specificity()
				s.spec[0] += sp[0]
				s.spec[1] += sp[1]
			}
			return s, nil
		case t.is('>'):
			p.i++
			p.skipWS()
			comb = '>'
		case t.is('+') || t.is('~'):
			return nil, &selErr{t.pos, Warning, "the sibling combinators + and ~ are not supported"}
		case ws:
			comb = ' '
		default:
			return nil, &selErr{t.pos, Error, fmt.Sprintf("unexpected %q in a selector", t.text())}
		}
	}
}

// compound reads a compound selector; outer tells one outside :not(),
// which may end with a pseudo-element.
func (p *selParser) compound(outer bool) (compound, *selErr) {
	var c compound
	start := p.peek()
	any := false
	switch t := p.peek(); {
	case t.kind == tIdent:
		c.typ = strings.ToLower(t.val)
		p.i++
		any = true
		if _, ok := elementTypes[c.typ]; !ok {
			p.warn(t.pos, fmt.Sprintf("unknown element %q%s", t.val, didYouMean(c.typ, typeNames)))
		}
	case t.is('*'):
		p.i++
		any = true
	}
	for {
		t := p.peek()
		if c.pseudo != "" && (t.is('.') || t.kind == tLBracket || t.kind == tIdent) {
			return c, &selErr{t.pos, Error, "nothing but pseudo-classes can follow a pseudo-element"}
		}
		switch {
		case t.is('.'):
			p.i++
			n := p.peek()
			if n.kind != tIdent {
				return c, &selErr{t.pos, Error, `expected a class name after "."`}
			}
			p.i++
			c.classes = append(c.classes, n.val)
			p.checkName(n.pos, c.typ, "class", n.val, func(t elemType) []string { return t.classes }, allClasses)
			any = true
		case t.kind == tNumber || t.kind == tDimension:
			return c, &selErr{t.pos, Error, "a class name cannot start with a digit (heading levels are .h1 to .h6)"}
		case t.kind == tHash:
			return c, &selErr{t.pos, Warning, "ids are not supported: nodes have none"}
		case t.kind == tLBracket:
			a, err := p.attr(c.typ)
			if err != nil {
				return c, err
			}
			c.attrs = append(c.attrs, a)
			any = true
		case t.kind == tColon:
			p.i++
			if p.peek().kind == tColon {
				p.i++
				if !outer {
					return c, &selErr{t.pos, Error, "a pseudo-element cannot go inside :not()"}
				}
				if err := p.pseudoElement(&c); err != nil {
					return c, err
				}
			} else if err := p.pseudoClass(&c); err != nil {
				return c, err
			}
			any = true
		default:
			if !any {
				return c, &selErr{start.pos, Error, fmt.Sprintf("expected a selector, not %q", start.text())}
			}
			return c, nil
		}
	}
}

// checkName warns of a class or an attribute no element of the type has.
func (p *selParser) checkName(pos int, typ, what, name string, of func(elemType) []string, all []string) {
	if t, ok := elementTypes[typ]; ok {
		if !slices.Contains(of(t), name) {
			p.warn(pos, fmt.Sprintf("%s has no %s %q%s", typ, what, name, didYouMean(name, of(t))))
		}
		return
	}
	if typ == "" && !slices.Contains(all, name) {
		p.warn(pos, fmt.Sprintf("no element has the %s %q%s", what, name, didYouMean(name, all)))
	}
}

// attr reads an attribute selector, from its "[".
func (p *selParser) attr(typ string) (attrSel, *selErr) {
	open := p.peek()
	p.i++
	p.skipWS()
	n := p.peek()
	if n.kind != tIdent {
		return attrSel{}, &selErr{n.pos, Error, "expected an attribute name after \"[\""}
	}
	p.i++
	a := attrSel{name: strings.ToLower(n.val)}
	p.checkName(n.pos, typ, "attribute", a.name, func(t elemType) []string { return t.attrs }, allAttrs)
	p.skipWS()
	t := p.peek()
	switch {
	case t.kind == tRBracket:
		p.i++
		return a, nil
	case t.is('='):
		a.op = '='
		p.i++
	case t.kind == tDelim && strings.Contains("~^$*|", t.val) && p.i+1 < len(p.ts) && p.ts[p.i+1].is('='):
		if t.val == "|" {
			return a, &selErr{t.pos, Warning, "the attribute operator |= is not supported"}
		}
		a.op = t.val[0]
		p.i += 2
	default:
		return a, &selErr{t.pos, Error, "expected \"]\" or an operator in the attribute selector"}
	}
	p.skipWS()
	v := p.peek()
	if v.kind != tIdent && v.kind != tString {
		return a, &selErr{v.pos, Error, "expected a value in the attribute selector"}
	}
	a.val = v.val
	p.i++
	p.skipWS()
	if f := p.peek(); f.isIdent("i") || f.isIdent("s") {
		a.fold = f.isIdent("i")
		p.i++
		p.skipWS()
	}
	if p.peek().kind != tRBracket {
		return a, &selErr{open.pos, Error, "unclosed \"[\""}
	}
	p.i++
	return a, nil
}

// pseudoElement reads a pseudo-element after its "::", and the
// pseudo-classes after it.
func (p *selParser) pseudoElement(c *compound) *selErr {
	n := p.peek()
	if n.kind != tIdent {
		return &selErr{n.pos, Error, `expected a pseudo-element name after "::"`}
	}
	p.i++
	name := strings.ToLower(n.val)
	if why, ok := unsupportedPseudos[name]; ok {
		return &selErr{n.pos, Warning, why}
	}
	if !slices.Contains(allPseudos, name) {
		return &selErr{n.pos, Warning, fmt.Sprintf("unknown pseudo-element ::%s%s", name, didYouMean(name, allPseudos))}
	}
	if t, ok := elementTypes[c.typ]; ok && !slices.Contains(t.pseudos, name) {
		if slices.Contains(globalPseudos, name) && c.typ != "document" {
			p.warn(n.pos, fmt.Sprintf("::%s belongs to the document: write it alone or on :root", name))
		} else if !slices.Contains(globalPseudos, name) {
			p.warn(n.pos, fmt.Sprintf("%s has no ::%s", c.typ, name))
		}
	}
	c.pseudo = name
	for p.peek().kind == tColon && !(p.i+1 < len(p.ts) && p.ts[p.i+1].kind == tColon) {
		colon := p.peek()
		p.i++
		s := p.peek()
		if name == "search-text" && s.isIdent("current") {
			p.i++
			c.pstates |= Current
			continue
		}
		return &selErr{colon.pos, Error, fmt.Sprintf("::%s takes no pseudo-class %q", name, ":"+s.text())}
	}
	return nil
}

// pseudoClass reads a pseudo-class after its ":".
func (p *selParser) pseudoClass(c *compound) *selErr {
	n := p.peek()
	switch n.kind {
	case tIdent:
		p.i++
		name := strings.ToLower(n.val)
		switch name {
		case "first-child":
			c.states |= FirstChild
		case "last-child":
			c.states |= LastChild
		case "revealed":
			c.states |= Revealed
		case "broken":
			c.states |= Broken
		case "root":
			c.root = true
		case "current":
			return &selErr{n.pos, Error, ":current goes after ::search-text alone"}
		default:
			if why, ok := unsupportedPseudos[name]; ok {
				return &selErr{n.pos, Warning, why}
			}
			return &selErr{n.pos, Warning, fmt.Sprintf("unknown pseudo-class :%s%s", name, didYouMean(name, pseudoClassNames))}
		}
		return nil
	case tFunction:
		p.i++
		end := closing(p.ts, p.i)
		if end < 0 {
			return &selErr{n.pos, Error, fmt.Sprintf("unclosed :%s(", n.val)}
		}
		args := p.ts[p.i:end]
		p.i = end + 1
		if n.val != "not" {
			if why, ok := unsupportedPseudos[n.val]; ok {
				return &selErr{n.pos, Warning, why}
			}
			return &selErr{n.pos, Warning, fmt.Sprintf("unknown pseudo-class :%s()", n.val)}
		}
		for _, arg := range splitTop(args, tComma) {
			arg = trimWS(arg)
			if len(arg) == 0 {
				return &selErr{n.pos, Error, "empty :not()"}
			}
			q := &selParser{ts: arg, end: arg[len(arg)-1].end, warn: p.warn}
			inner, err := q.compound(false)
			if err != nil {
				return err
			}
			if q.skipWS(); q.i < len(q.ts) {
				return &selErr{q.peek().pos, Warning, ":not() takes compound selectors alone, without combinators"}
			}
			c.nots = append(c.nots, inner)
		}
		return nil
	}
	return &selErr{n.pos, Error, `expected a pseudo-class after ":"`}
}

func (c *compound) specificity() specificity {
	var s specificity
	s[0] = len(c.classes) + len(c.attrs) + bits.OnesCount8(uint8(c.states)) + bits.OnesCount8(uint8(c.pstates))
	if c.root {
		s[0]++
	}
	var most specificity
	for i := range c.nots {
		if n := c.nots[i].specificity(); most.less(n) {
			most = n
		}
	}
	s[0] += most[0]
	s[1] += most[1]
	if c.typ != "" {
		s[1]++
	}
	if c.pseudo != "" {
		s[1]++
	}
	return s
}

// matches reports whether the selector matches node n, or its
// pseudo-element pseudo in states ps when pseudo is not "".
func (s *selector) matches(n *Node, pseudo string, ps States) bool {
	last := len(s.parts) - 1
	c := &s.parts[last]
	if c.pseudo != pseudo || c.pstates&^ps != 0 || !c.matches(n) {
		return false
	}
	return s.matchLeft(last, n)
}

// matchLeft reports whether the compounds left of parts[i] match the
// ancestors of n, which parts[i] matched.
func (s *selector) matchLeft(i int, n *Node) bool {
	if i == 0 {
		return true
	}
	c := &s.parts[i-1]
	if s.combs[i] == '>' {
		p := n.parent
		return p != nil && c.matches(p) && s.matchLeft(i-1, p)
	}
	for a := n.parent; a != nil; a = a.parent {
		if c.matches(a) && s.matchLeft(i-1, a) {
			return true
		}
	}
	return false
}

// matches reports whether the compound matches node n.
func (c *compound) matches(n *Node) bool {
	e := &n.el
	if c.typ != "" && c.typ != e.Type || c.root && n.parent != nil || c.states&^e.States != 0 {
		return false
	}
	for _, cl := range c.classes {
		if !e.HasClass(cl) {
			return false
		}
	}
	for i := range c.attrs {
		if !c.attrs[i].matches(e) {
			return false
		}
	}
	for i := range c.nots {
		if c.nots[i].matches(n) {
			return false
		}
	}
	return true
}

func (a *attrSel) matches(e *Element) bool {
	v, ok := e.Attr(a.name)
	if !ok {
		return false
	}
	x := a.val
	if a.fold {
		v, x = strings.ToLower(v), strings.ToLower(x)
	}
	switch a.op {
	case 0:
		return true
	case '=':
		return v == x
	case '~':
		return slices.Contains(strings.Fields(v), x)
	case '^':
		return x != "" && strings.HasPrefix(v, x)
	case '$':
		return x != "" && strings.HasSuffix(v, x)
	case '*':
		return x != "" && strings.Contains(v, x)
	}
	return false
}

// splitTop splits tokens at the tokens of kind sep outside parentheses,
// brackets and braces.
func splitTop(ts []token, sep tokKind) [][]token {
	var out [][]token
	depth, start := 0, 0
	for i, t := range ts {
		switch t.kind {
		case tFunction, tLParen, tLBracket, tLBrace:
			depth++
		case tRParen, tRBracket, tRBrace:
			depth = max(depth-1, 0)
		case sep:
			if depth == 0 {
				out = append(out, ts[start:i])
				start = i + 1
			}
		}
	}
	return append(out, ts[start:])
}

// closing returns the index of the ")" closing the function or the
// parenthesis whose contents start at i, or -1.
func closing(ts []token, i int) int {
	depth := 0
	for ; i < len(ts); i++ {
		switch ts[i].kind {
		case tFunction, tLParen:
			depth++
		case tRParen:
			if depth == 0 {
				return i
			}
			depth--
		}
	}
	return -1
}

// trimWS returns tokens without the whitespace at their ends.
func trimWS(ts []token) []token {
	for len(ts) > 0 && ts[0].kind == tWhitespace {
		ts = ts[1:]
	}
	for len(ts) > 0 && ts[len(ts)-1].kind == tWhitespace {
		ts = ts[:len(ts)-1]
	}
	return ts
}
