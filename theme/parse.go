package theme

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Sheet is a style sheet, parsed: its rules and the fonts it brings.
type Sheet struct {
	name  string
	rules []*rule
	faces []FontFace
}

// FontFace is a font a sheet brings with @font-face.
type FontFace struct {
	// Family is the name font-family takes it by.
	Family string
	// Path is its file: as the sheet writes it, or absolute when ParseFile
	// read the sheet.
	Path   string
	Weight int
	Italic bool
}

// Name returns the name the sheet was parsed with.
func (s *Sheet) Name() string { return s.name }

// FontFaces returns the fonts the sheet brings.
func (s *Sheet) FontFaces() []FontFace { return s.faces }

// rule is a rule of a sheet: its selectors, its declarations, and the
// media it applies in.
type rule struct {
	sels  []*selector
	decls []*decl
	media media
}

// decl is a declaration of a rule.
type decl struct {
	prop      string
	val       []token
	important bool
	// vars tells a value holding var(), which reads where it is computed,
	// as the variables are known there; sets is what the others set.
	vars bool
	sets []setting
}

// custom reports whether the declaration is of a variable, --name.
func (d *decl) custom() bool { return strings.HasPrefix(d.prop, "--") }

// Parse reads a style sheet named name. It never fails: what it cannot
// read it skips, as browsers do, and reports in the diagnostics.
func Parse(name, css string) (*Sheet, []Diagnostic) {
	d := &diagnostics{src: css}
	s := &Sheet{name: name}
	p := &sheetParser{ts: tokenize(css, d), d: d, s: s, end: len(css)}
	p.rules(mediaAll, false)
	return s, d.list
}

// ParseFile reads the style sheet of a file. The fonts it brings are
// relative to its directory.
func ParseFile(path string) (*Sheet, []Diagnostic, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	s, diags := Parse(filepath.Base(path), string(b))
	dir := filepath.Dir(path)
	for i := range s.faces {
		if f := &s.faces[i]; !filepath.IsAbs(f.Path) && !strings.Contains(f.Path, "://") {
			f.Path = filepath.Join(dir, filepath.FromSlash(f.Path))
		}
	}
	return s, diags, nil
}

// sheetParser reads the rules of a sheet.
type sheetParser struct {
	ts  []token
	i   int
	d   *diagnostics
	s   *Sheet
	end int
}

func (p *sheetParser) peek() token {
	if p.i < len(p.ts) {
		return p.ts[p.i]
	}
	return token{kind: tEOF, pos: p.end, end: p.end}
}

// rules reads rules, up to the end of the sheet, or up to the "}" closing
// a block when nested.
func (p *sheetParser) rules(m media, nested bool) {
	for {
		switch t := p.peek(); t.kind {
		case tEOF:
			return
		case tWhitespace, tCDO, tCDC:
			p.i++
		case tRBrace:
			if nested {
				return
			}
			p.d.add(Error, t.pos, `unexpected "}"`)
			p.i++
		case tAtKeyword:
			p.atRule(m)
		default:
			p.qualifiedRule(m)
		}
	}
}

// qualifiedRule reads a rule: its selectors and its block of declarations.
func (p *sheetParser) qualifiedRule(m media) {
	start := p.i
	depth := 0
	for {
		t := p.peek()
		switch t.kind {
		case tEOF:
			p.d.add(Error, p.ts[start].pos, `expected "{" after the selector`)
			return
		case tFunction, tLParen, tLBracket:
			depth++
		case tRParen, tRBracket:
			depth = max(depth-1, 0)
		case tSemicolon:
			if depth == 0 {
				p.d.add(Error, t.pos, `expected "{" after the selector`)
				p.i++
				return
			}
		case tRBrace:
			if depth == 0 {
				p.d.add(Error, t.pos, `expected "{" after the selector`)
				return
			}
		case tLBrace:
			if depth == 0 {
				prelude := p.ts[start:p.i]
				body := p.block()
				sels := parseSelectors(prelude, p.d)
				decls := p.decls(body, sels)
				if len(sels) > 0 && len(decls) > 0 {
					p.s.rules = append(p.s.rules, &rule{sels: sels, decls: decls, media: m})
				}
				return
			}
		}
		p.i++
	}
}

// block reads the block at "{" up to its "}", and returns what is in it.
func (p *sheetParser) block() []token {
	open := p.peek()
	p.i++
	start := p.i
	depth := 0
	for ; p.i < len(p.ts); p.i++ {
		switch p.ts[p.i].kind {
		case tLBrace:
			depth++
		case tRBrace:
			if depth == 0 {
				body := p.ts[start:p.i]
				p.i++
				return body
			}
			depth--
		}
	}
	p.d.add(Error, open.pos, `unclosed "{"`)
	return p.ts[start:]
}

// atRule reads an at-rule: @media and @font-face, or another, which it
// skips.
func (p *sheetParser) atRule(m media) {
	at := p.peek()
	p.i++
	name := strings.ToLower(at.val)
	start := p.i
	for p.i < len(p.ts) {
		if k := p.ts[p.i].kind; k == tSemicolon || k == tLBrace || k == tRBrace {
			break
		}
		p.i++
	}
	prelude := p.ts[start:p.i]
	hasBlock := p.peek().kind == tLBrace
	skip := func() {
		switch p.peek().kind {
		case tLBrace:
			p.block()
		case tSemicolon:
			p.i++
		}
	}
	switch name {
	case "media":
		if !hasBlock {
			p.d.add(Error, at.pos, `expected "{" after @media`)
			skip()
			return
		}
		cond := parseMedia(prelude, p.d)
		p.i++
		p.rules(m.and(cond), true)
		if p.peek().kind == tRBrace {
			p.i++
		} else {
			p.d.add(Error, at.pos, `unclosed "{" of @media`)
		}
	case "font-face":
		if !hasBlock {
			p.d.add(Error, at.pos, `expected "{" after @font-face`)
			skip()
			return
		}
		p.fontFace(p.block(), at.pos)
	case "charset":
		skip()
	case "import":
		p.d.add(Warning, at.pos, "@import is not supported: put the rules in this sheet")
		skip()
	default:
		p.d.addf(Warning, at.pos, "@%s is not supported", name)
		skip()
	}
}

// decls reads the declarations of a rule's block, warning of those that
// apply to none of the elements its selectors match.
func (p *sheetParser) decls(ts []token, sels []*selector) []*decl {
	var out []*decl
	for _, part := range splitTop(ts, tSemicolon) {
		part = trimWS(part)
		if len(part) == 0 {
			continue
		}
		name := part[0]
		switch {
		case name.kind == tAtKeyword:
			p.d.add(Error, name.pos, "at-rules cannot go inside a rule")
			continue
		case name.kind != tIdent:
			p.d.addf(Error, name.pos, "expected a property, not %q", name.text())
			continue
		}
		rest := trimWS(part[1:])
		if len(rest) == 0 || rest[0].kind != tColon {
			if hasKind(part, tLBrace) {
				p.d.add(Error, name.pos, "nested rules are not supported")
			} else {
				p.d.addf(Error, name.pos, "expected \":\" after %q", name.val)
			}
			continue
		}
		val := trimWS(rest[1:])
		dl := &decl{prop: name.val, val: val}
		if n := len(val); n >= 2 && val[n-1].isIdent("important") {
			if k := len(trimWS(val[:n-1])); k > 0 && val[k-1].is('!') {
				dl.important = true
				dl.val = trimWS(val[:k-1])
			}
		}
		if dl.custom() {
			out = append(out, dl)
			continue
		}
		dl.prop = strings.ToLower(dl.prop)
		_, short := shorthands[dl.prop]
		id, long := propIDs[dl.prop]
		if !short && !long {
			p.d.addf(Warning, name.pos, "unknown property %q%s", name.val, didYouMean(dl.prop, propNames))
			continue
		}
		if len(dl.val) == 0 {
			p.d.addf(Warning, name.pos, "%s has no value", dl.prop)
			continue
		}
		if hasVar(dl.val) {
			dl.vars = true
		} else {
			sets, msg, ok := parseDecl(dl.prop, dl.val)
			if !ok {
				if msg != "" {
					msg = ": " + msg
				}
				p.d.addf(Warning, dl.val[0].pos, "invalid value for %s: %q%s", dl.prop, tokensText(dl.val), msg)
				continue
			}
			if msg != "" {
				p.d.add(Warning, dl.val[0].pos, msg)
			}
			dl.sets = sets
		}
		if long {
			p.checkApplies(name, []propID{id}, sels)
		} else {
			p.checkApplies(name, shorthands[dl.prop].subs, sels)
		}
		out = append(out, dl)
	}
	return out
}

// checkApplies warns when none of the longhands a declaration sets applies
// to the elements of any of the selectors.
func (p *sheetParser) checkApplies(name token, ids []propID, sels []*selector) {
	for _, s := range sels {
		cat, ok := s.category()
		if !ok {
			return
		}
		for _, id := range ids {
			if props[id].applies(cat) {
				return
			}
		}
	}
	if len(sels) > 0 {
		c := &sels[0].parts[len(sels[0].parts)-1]
		what := c.typ
		if c.pseudo != "" {
			what += "::" + c.pseudo
		}
		p.d.addf(Warning, name.pos, "%s does not apply to %s", strings.ToLower(name.val), what)
	}
}

// category returns the category of the elements the selector styles,
// when it can tell.
func (s *selector) category() (elemCat, bool) {
	c := &s.parts[len(s.parts)-1]
	if c.pseudo != "" {
		return catPseudo, true
	}
	t, ok := elementTypes[c.typ]
	if !ok {
		return 0, false
	}
	if t.cat&catBlock != 0 && t.cat&catInline != 0 {
		switch {
		case slices.Contains(c.classes, "block"):
			return catBlock, true
		case slices.Contains(c.classes, "inline"):
			return catInline, true
		}
	}
	return t.cat, true
}

func hasKind(ts []token, k tokKind) bool {
	for _, t := range ts {
		if t.kind == k {
			return true
		}
	}
	return false
}

// fontFace reads the descriptors of @font-face.
func (p *sheetParser) fontFace(ts []token, pos int) {
	f := FontFace{Weight: 400}
	for _, part := range splitTop(ts, tSemicolon) {
		part = trimWS(part)
		if len(part) == 0 {
			continue
		}
		rest := trimWS(part[1:])
		if part[0].kind != tIdent || len(rest) == 0 || rest[0].kind != tColon {
			p.d.add(Error, part[0].pos, "expected a descriptor of @font-face")
			continue
		}
		val := trimWS(rest[1:])
		q := &vparser{ts: val}
		switch name := strings.ToLower(part[0].val); name {
		case "font-family":
			v, _ := parseFamilies(q)
			fs, _ := v.(families)
			if len(fs) != 1 {
				p.d.add(Warning, part[0].pos, "font-family of @font-face takes one name")
				continue
			}
			f.Family = fs[0]
		case "src":
			f.Path = p.fontSrc(val, part[0].pos)
		case "font-weight":
			v, _ := parseWeight(q)
			w, ok := v.(weight)
			if !ok || !q.done() {
				p.d.add(Warning, part[0].pos, "invalid font-weight of @font-face")
				continue
			}
			f.Weight = int(w)
		case "font-style":
			v, _ := props[pFontStyle].parse(q)
			it, ok := v.(italic)
			if !ok || !q.done() {
				p.d.add(Warning, part[0].pos, "invalid font-style of @font-face")
				continue
			}
			f.Italic = bool(it)
		case "font-display", "unicode-range", "font-stretch":
		default:
			p.d.addf(Warning, part[0].pos, "unknown descriptor %q of @font-face", name)
		}
	}
	if f.Family == "" || f.Path == "" {
		p.d.add(Warning, pos, "@font-face needs a font-family and a src of a TrueType or OpenType font")
		return
	}
	p.s.faces = append(p.s.faces, f)
}

// fontSrc returns the first source of src that is a TrueType or an
// OpenType font.
func (p *sheetParser) fontSrc(ts []token, pos int) string {
	for _, part := range splitTop(ts, tComma) {
		q := &vparser{ts: part}
		var path string
		switch t := q.next(); {
		case t.kind == tURL:
			path = t.val
		case t.kind == tFunction && t.val == "url":
			args, ok := q.args()
			if !ok || len(args) != 1 || len(trimWS(args[0])) != 1 || trimWS(args[0])[0].kind != tString {
				continue
			}
			path = trimWS(args[0])[0].val
		default:
			continue
		}
		format := ""
		if t := q.peek(); t.kind == tFunction && t.val == "format" {
			q.next()
			args, ok := q.args()
			if ok && len(args) > 0 && len(trimWS(args[0])) == 1 {
				format = strings.ToLower(trimWS(args[0])[0].val)
			}
		}
		ext := strings.ToLower(filepath.Ext(path))
		switch {
		case format == "truetype", format == "opentype", format == "collection":
			return path
		case format == "" && ext != ".woff" && ext != ".woff2":
			return path
		}
	}
	p.d.add(Warning, pos, "only TrueType and OpenType fonts (.ttf, .otf, .ttc) are supported")
	return ""
}

// media is when rules apply: always, in the dark or the light, or never.
type media uint8

const (
	mediaAll media = iota
	mediaDark
	mediaLight
	mediaNone
)

func (m media) and(o media) media {
	switch {
	case m == mediaAll:
		return o
	case o == mediaAll, m == o:
		return m
	}
	return mediaNone
}

func (m media) or(o media) media {
	switch {
	case m == mediaNone:
		return o
	case o == mediaNone, m == o:
		return m
	}
	return mediaAll
}

func (m media) applies(dark bool) bool {
	switch m {
	case mediaAll:
		return true
	case mediaDark:
		return dark
	case mediaLight:
		return !dark
	}
	return false
}

// parseMedia reads the queries of @media.
func parseMedia(ts []token, d *diagnostics) media {
	ts = trimWS(ts)
	if len(ts) == 0 {
		return mediaAll
	}
	out := mediaNone
	for _, q := range splitTop(ts, tComma) {
		out = out.or(parseQuery(trimWS(q), d))
	}
	return out
}

// parseQuery reads a media query: a media type, and features joined by
// "and", of which prefers-color-scheme alone is known.
func parseQuery(ts []token, d *diagnostics) media {
	m := mediaAll
	for i := 0; i < len(ts); i++ {
		t := ts[i]
		switch {
		case t.kind == tWhitespace, t.isIdent("and"), t.isIdent("only"), t.isIdent("all"), t.isIdent("screen"):
		case t.isIdent("print"), t.isIdent("speech"):
			return mediaNone
		case t.isIdent("not"):
			d.add(Warning, t.pos, "not in media queries is not supported")
			return mediaNone
		case t.kind == tLParen:
			end := closing(ts, i+1)
			if end < 0 {
				d.add(Error, t.pos, `unclosed "(" in the media query`)
				return mediaNone
			}
			m = m.and(parseFeature(trimWS(ts[i+1:end]), d, t.pos))
			i = end
		default:
			d.add(Warning, t.pos, fmt.Sprintf("unknown media %q", t.text()))
			return mediaNone
		}
	}
	return m
}

func parseFeature(ts []token, d *diagnostics, pos int) media {
	var words []token
	for _, t := range ts {
		if t.kind != tWhitespace {
			words = append(words, t)
		}
	}
	if len(words) == 3 && words[0].isIdent("prefers-color-scheme") && words[1].kind == tColon {
		switch {
		case words[2].isIdent("dark"):
			return mediaDark
		case words[2].isIdent("light"):
			return mediaLight
		}
	}
	d.add(Warning, pos, "only the media feature (prefers-color-scheme: dark | light) is supported")
	return mediaNone
}
