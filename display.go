package livemark

import (
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"

	"github.com/CarlOpenLab/livemark/theme"
)

// A leaf shows as display text: the characters drawn, each piece of which
// stands for a range of the source. Text pieces are the source itself,
// character for character; atoms stand for their range as a whole, as a
// line break for its newline and the prefixes after it, or a formula for
// its TeX; decorations stand for nothing, as a fence's backticks drawn
// beside an info string. Syntax that is hidden is simply not a piece: the
// caret cannot go there, and the display skips it.

type pkind uint8

const (
	pText pkind = iota
	pAtom
	pDeco
)

// piece is a run of display text and the source range it stands for.
type piece struct {
	kind pkind
	// s0 and s1 are the source range, relative to the leaf's start; d0 and
	// d1 the display range, in UTF-16 units.
	s0, s1 int
	d0, d1 int
	text   string
	span   ui.Span
	// obj is the inline object an atom draws: an image or a formula.
	obj *object
	// marker tells syntax shown because the caret is at it.
	marker bool
}

// object is an inline object of a layout, drawn in the box the layout
// gives its object replacement character.
type object struct {
	bmp *ui.Bitmap
	// src is what the image was loaded from; alt is drawn in a
	// placeholder while bmp is nil, in style, or its file's name.
	src   string
	alt   string
	style *theme.Computed
	// w, h and descent are the box's size, in DIPs.
	w, h, descent float32
	// block makes the object a line of its own, centered: a paragraph of
	// just an image.
	block bool
}

// child returns the node of the element el inside n, nil for nil: the
// theme node a run of text is in, and the inline construct inside it.
func child(n *theme.Node, el theme.Element) *theme.Node {
	if n == nil {
		return nil
	}
	return n.Child(el)
}

// syntaxElement returns the element of a marker of syntax, of a class.
func syntaxElement(class string) theme.Element {
	el := theme.Element{Type: "syntax"}
	if class != "" {
		el.Classes = []string{class}
	}
	return el
}

// builder makes the pieces of a leaf's display.
type builder struct {
	e   *Editor
	src string
	// base is the offset pieces are relative to.
	base int
	// reveal is the caret's offset, which shows the syntax of what it is
	// in; -1 for none.
	reveal int
	pos    int
	pieces []piece
	text   strings.Builder
	u16    int
	// The composition of an input method, inserted at composeAt.
	compose   string
	composeAt int
	composed  bool
	lastSpan  ui.Span
	// hasObjects tells a display with images or formulas.
	hasObjects bool
	// htmlTags are the formatting tags open, as <u>, and the style each
	// had before it.
	htmlTags []htmlTag
	// width is the room of the leaf, which images shrink to.
	width float32
	// alone tells a paragraph of just an image, drawn as a block; and
	// revealedAny that its source shows.
	alone, revealedAny bool
	// fontSize is the size of the leaf's text.
	fontSize float32
}

type htmlTag struct {
	name string
	prev *theme.Node
}

func (b *builder) revealed(s, e int) bool { return b.reveal >= s && b.reveal <= e }

// add appends a piece, inserting the composition where it falls.
func (b *builder) add(p piece) {
	if b.compose != "" && !b.composed && p.kind == pText && b.composeAt > p.s0+b.base && b.composeAt < p.s1+b.base {
		cut := b.composeAt - b.base - p.s0
		left, right := p, p
		left.text, left.s1 = p.text[:cut], p.s0+cut
		right.text, right.s0 = p.text[cut:], p.s0+cut
		b.add(left)
		b.addCompose(p.span)
		b.add(right)
		return
	}
	if b.compose != "" && !b.composed && b.composeAt <= p.s0+b.base && (p.kind != pDeco || b.composeAt < p.s0+b.base) {
		b.addCompose(b.lastSpan)
	}
	p.d0 = b.u16
	b.u16 += ui.UTF16Len(p.text)
	p.d1 = b.u16
	b.text.WriteString(p.text)
	b.pieces = append(b.pieces, p)
	if p.kind == pText {
		b.lastSpan = p.span
	}
}

// addCompose inserts the composition, underlined in the style around it.
func (b *builder) addCompose(sp ui.Span) {
	b.composed = true
	sp.Text = ""
	sp.Underline = true
	sp.ObjectWidth, sp.ObjectHeight, sp.ObjectDescent = 0, 0, 0
	at := b.composeAt - b.base
	p := piece{kind: pDeco, s0: at, s1: at, text: b.compose, span: sp}
	p.d0 = b.u16
	b.u16 += ui.UTF16Len(p.text)
	p.d1 = b.u16
	b.text.WriteString(p.text)
	b.pieces = append(b.pieces, p)
}

// finish inserts the composition at the end, when nothing came after it.
func (b *builder) finish(end int) {
	if b.compose != "" && !b.composed && b.composeAt >= b.base && b.composeAt <= end {
		b.addCompose(b.lastSpan)
	}
}

// span returns how text in style st looks in the leaf.
func (b *builder) span(st *theme.Node) ui.Span {
	if st == nil {
		return b.e.styleSpan(b.e.rootStyle())
	}
	return b.e.styleSpan(st.Style())
}

// spanRun adds source text from s to e as it is, in sp.
func (b *builder) spanRun(s, e int, sp ui.Span) {
	if e <= s {
		return
	}
	b.add(piece{kind: pText, s0: s - b.base, s1: e - b.base, text: b.src[s:e], span: sp})
}

// codeRun adds code from s to e in sp, a tab drawn as four spaces.
func (b *builder) codeRun(s, e int, sp ui.Span) {
	for s < e {
		t := strings.IndexByte(b.src[s:e], '\t')
		if t < 0 {
			b.spanRun(s, e, sp)
			return
		}
		b.spanRun(s, s+t, sp)
		b.spanAtom(s+t, s+t+1, "    ", sp)
		s += t + 1
	}
}

// spanAtom adds display text standing for the source from s to e, in sp.
func (b *builder) spanAtom(s, e int, text string, sp ui.Span) {
	b.add(piece{kind: pAtom, s0: s - b.base, s1: e - b.base, text: text, span: sp})
}

// textRun adds source text from s to e as it is.
func (b *builder) textRun(s, e int, st *theme.Node) {
	if e <= s {
		return
	}
	b.add(piece{kind: pText, s0: s - b.base, s1: e - b.base, text: b.src[s:e], span: b.span(st)})
}

// marker adds syntax shown because the caret is at it.
func (b *builder) marker(s, e int, st *theme.Node, class string) {
	if e <= s {
		return
	}
	b.marked(s, e, st, syntaxElement(class))
}

// marked adds source text shown as it is written, in the element el.
func (b *builder) marked(s, e int, st *theme.Node, el theme.Element) {
	if e <= s {
		return
	}
	b.add(piece{kind: pText, s0: s - b.base, s1: e - b.base, text: b.src[s:e], span: b.span(child(st, el)), marker: true})
}

// atom adds display text standing for the source from s to e as a whole.
func (b *builder) atom(s, e int, text string, st *theme.Node, obj *object) {
	sp := b.span(st)
	if obj != nil {
		if obj.bmp == nil && st != nil {
			// A placeholder shows in the style of what it stands for.
			obj.style = st.Style()
		}
		b.hasObjects = true
		// An object is not text: the span's background would show around
		// the box it draws.
		sp.Background, sp.Radius = ui.Color{}, 0
		sp.ObjectWidth, sp.ObjectHeight, sp.ObjectDescent = obj.w, obj.h, obj.descent
		text = "￼"
	}
	b.add(piece{kind: pAtom, s0: s - b.base, s1: e - b.base, text: text, span: sp, obj: obj})
}

// deco adds display text standing for no source, at s.
func (b *builder) deco(s int, text string, sp ui.Span, obj *object) {
	if obj != nil {
		b.hasObjects = true
		sp.Background, sp.Radius = ui.Color{}, 0
		sp.ObjectWidth, sp.ObjectHeight, sp.ObjectDescent = obj.w, obj.h, obj.descent
		text = "￼"
	}
	b.add(piece{kind: pDeco, s0: s - b.base, s1: s - b.base, text: text, span: sp, obj: obj})
}

// gap shows the spaces between the last piece and to, which no inline
// holds: the parser trims them, the editor shows them, so that a space
// typed at the end of a line shows with the caret after it.
func (b *builder) gap(to int, st *theme.Node) {
	if b.pos >= to {
		return
	}
	s := b.pos
	b.pos = to
	g := b.src[s:to]
	if strings.Trim(g, " \t") != "" {
		return
	}
	b.textRun(s, to, st)
}

// inlines adds the display of a block's inlines, from the source offset
// from to to.
func (b *builder) inlines(ins []parser.Inline, from, to int, st *theme.Node) {
	b.pos = from
	b.walk(ins, 0, len(ins), st)
	b.gap(to, st)
	b.finish(to)
}

func (b *builder) walk(ins []parser.Inline, i, end int, st *theme.Node) {
	for i < end {
		in := &ins[i]
		b.gap(in.Start, st)
		next := max(in.Next, i+1)
		switch in.Kind {
		case parser.Text:
			b.text_(in.Start, in.End, st)
		case parser.SoftBreak, parser.LineBreak:
			b.atom(in.Start, in.End, "\n", st, nil)
		case parser.Code:
			b.codeSpan(in, st)
		case parser.Emphasis, parser.Strong, parser.Strikethrough:
			b.wrapped(ins, i, st)
		case parser.Link:
			b.link(ins, i, st)
		case parser.Image:
			b.image(ins, i, st)
		case parser.HTML:
			st = b.htmlInline(in, st)
		case parser.Math:
			b.math(in, st)
		case parser.FootnoteRef:
			b.footnoteRef(in, st)
		default:
			b.textRun(in.Start, in.End, st)
		}
		b.pos = max(b.pos, in.End)
		i = next
	}
}

// children returns where the children of inline i start and end in the
// source: the first one's start and the last one's end, or from and from
// when it has none.
func children(ins []parser.Inline, i int, from int) (int, int) {
	in := &ins[i]
	if i+1 >= in.Next || i+1 >= len(ins) {
		return from, from
	}
	s := ins[i+1].Start
	e := s
	for j := i + 1; j < in.Next && j < len(ins); j = max(ins[j].Next, j+1) {
		e = max(e, ins[j].End)
	}
	return s, e
}

// text_ adds a Text inline's source, with its escapes and character
// references as the characters they stand for, unless the caret is at
// them.
func (b *builder) text_(s, e int, st *theme.Node) {
	src := b.src
	run := s
	for p := s; p < e; {
		c := src[p]
		if c == '\\' && p+1 < e && isASCIIPunct(src[p+1]) {
			b.textRun(run, p, st)
			if b.revealed(p, p+2) {
				b.marker(p, p+1, st, "escape")
				b.textRun(p+1, p+2, st)
			} else {
				b.atom(p, p+2, src[p+1:p+2], st, nil)
			}
			p += 2
			run = p
			continue
		}
		if c == '&' {
			if n, txt := parser.Entity(src[p:e]); n > 0 && txt != "" {
				b.textRun(run, p, st)
				if b.revealed(p, p+n) {
					b.marker(p, p+n, st, "entity")
				} else {
					b.atom(p, p+n, txt, st, nil)
				}
				p += n
				run = p
				continue
			}
		}
		p++
	}
	b.textRun(run, e, st)
}

// wrapped adds emphasis, strong emphasis or strikethrough: its markers
// while the caret is at it, and its text in its style.
func (b *builder) wrapped(ins []parser.Inline, i int, st *theme.Node) {
	in := &ins[i]
	cs, ce := children(ins, i, in.Start)
	if cs == in.Start {
		// Nothing inside: show it as written.
		b.textRun(in.Start, in.End, st)
		return
	}
	rev := b.revealed(in.Start, in.End)
	inner := st
	class := ""
	switch in.Kind {
	case parser.Emphasis:
		inner, class = child(st, theme.Element{Type: "emphasis"}), "emphasis"
	case parser.Strong:
		inner, class = child(st, theme.Element{Type: "strong"}), "strong"
	case parser.Strikethrough:
		inner, class = child(st, theme.Element{Type: "strike"}), "strike"
	}
	if rev {
		b.marker(in.Start, cs, st, class)
	}
	b.pos = cs
	b.walk(ins, i+1, in.Next, inner)
	b.gap(ce, inner)
	if rev {
		b.marker(ce, in.End, st, class)
	}
	b.pos = in.End
}

// link adds a link: its text, and its brackets and destination while the
// caret is in it.
func (b *builder) link(ins []parser.Inline, i int, st *theme.Node) {
	in := &ins[i]
	cs, ce := children(ins, i, in.Start)
	el := theme.Element{Type: "link"}
	if in.Auto {
		el.Classes = append(el.Classes, "auto")
	}
	if in.Dest != "" {
		el.Attrs = append(el.Attrs, theme.Attr{Name: "href", Value: in.Dest})
	}
	if in.Title != "" {
		el.Attrs = append(el.Attrs, theme.Attr{Name: "title", Value: in.Title})
	}
	inner := child(st, el)
	if cs == in.Start {
		// A link without text shows its source, as there is nothing else
		// to click or to see.
		b.marker(in.Start, in.End, st, "link")
		return
	}
	if in.Auto && cs == in.Start && ce == in.End {
		b.walk(ins, i+1, in.Next, inner)
		return
	}
	rev := b.revealed(in.Start, in.End)
	if rev {
		b.marker(in.Start, cs, st, "link")
	}
	b.pos = cs
	b.walk(ins, i+1, in.Next, inner)
	b.gap(ce, inner)
	if rev {
		b.marker(ce, in.End, st, "link")
	}
	b.pos = in.End
}

// codeSpan adds code within text: its backticks while the caret is at it.
func (b *builder) codeSpan(in *parser.Inline, st *theme.Node) {
	n := 0
	for in.Start+n < in.End && b.src[in.Start+n] == '`' {
		n++
	}
	cs, ce := in.Start+n, in.End-n
	if ce < cs {
		ce = cs
	}
	inner := child(st, theme.Element{Type: "code", Classes: []string{"inline"}})
	if b.revealed(in.Start, in.End) {
		b.marker(in.Start, cs, st, "code")
		b.textRun(cs, ce, inner)
		b.marker(ce, in.End, st, "code")
		return
	}
	if ce == cs {
		b.marker(in.Start, in.End, st, "code")
		return
	}
	b.textRun(cs, ce, inner)
}

// math adds a formula within text: typeset, or its TeX while the caret is
// at it.
func (b *builder) math(in *parser.Inline, st *theme.Node) {
	n := 1
	if strings.HasPrefix(b.src[in.Start:in.End], "$$") || strings.HasPrefix(b.src[in.Start:in.End], `\(`) || strings.HasPrefix(b.src[in.Start:in.End], `\[`) {
		n = 2
	}
	cs, ce := min(in.Start+n, in.End), max(in.End-n, in.Start+n)
	el := theme.Element{Type: "math", Classes: []string{"inline"}}
	inner := child(st, el)
	// The formula draws in math's own ink; :revealed colors its source.
	color := uiColor(inner.Style().Color)
	if b.revealed(in.Start, in.End) {
		el.States |= theme.Revealed
		inner = child(st, el)
		b.marker(in.Start, cs, st, "math")
		b.textRun(cs, min(ce, in.End), inner)
		b.marker(min(ce, in.End), in.End, st, "math")
		return
	}
	if obj := b.e.mathObject(in.Text, b.fontSize, in.Display, color); obj != nil {
		b.atom(in.Start, in.End, "", inner, obj)
		return
	}
	el.States |= theme.Broken
	b.atom(in.Start, in.End, texFallback(in.Text), child(st, el), nil)
}

// eimage returns the theme element of an image inline.
func eimage(in *parser.Inline, obj *object) theme.Element {
	el := theme.Element{Type: "image"}
	if in.Dest != "" {
		el.Attrs = append(el.Attrs, theme.Attr{Name: "src", Value: in.Dest})
	}
	if in.Title != "" {
		el.Attrs = append(el.Attrs, theme.Attr{Name: "title", Value: in.Title})
	}
	if obj != nil && obj.alt != "" {
		el.Attrs = append(el.Attrs, theme.Attr{Name: "alt", Value: obj.alt})
	}
	if obj == nil || obj.bmp == nil {
		el.States |= theme.Broken
	}
	return el
}

// image adds an image: drawn, and its source too while the caret is at it.
func (b *builder) image(ins []parser.Inline, i int, st *theme.Node) {
	in := &ins[i]
	obj := b.e.imageObject(in.Dest, b.e.plainText(ins[i+1:max(in.Next, i+1)]), b.width, b.alone)
	el := eimage(in, obj)
	if obj != nil && obj.bmp == nil && st != nil {
		// The placeholder shows in the style of the broken image, whether
		// or not its source shows too.
		obj.style = child(st, el).Style()
	}
	if !b.revealed(in.Start, in.End) {
		b.atom(in.Start, in.End, "", child(st, el), obj)
		return
	}
	b.revealedAny = true
	el.States = theme.Revealed
	cs, ce := children(ins, i, in.Start+2)
	alt := child(st, el)
	b.marker(in.Start, cs, st, "image")
	b.pos = cs
	b.walk(ins, i+1, in.Next, alt)
	b.gap(ce, alt)
	b.marker(ce, in.End, st, "image")
	b.pos = in.End
	if obj != nil && obj.block {
		b.deco(in.End, "\n", b.span(st), nil)
	}
	b.deco(in.End, "", b.span(alt), obj)
}

// footnoteRef adds a reference to a footnote: its label raised, or its
// source while the caret is at it.
func (b *builder) footnoteRef(in *parser.Inline, st *theme.Node) {
	if b.revealed(in.Start, in.End) {
		b.marker(in.Start, in.End, st, "footnote-ref")
		return
	}
	el := theme.Element{Type: "footnote-ref"}
	if in.Text != "" {
		el.Attrs = append(el.Attrs, theme.Attr{Name: "label", Value: in.Text})
	}
	b.add(piece{kind: pAtom, s0: in.Start - b.base, s1: in.End - b.base, text: "[" + in.Text + "]", span: b.span(child(st, el))})
}

// htmlInline adds a tag within text. The tags of formatting, as <u> and
// <b>, are hidden, unless the caret is at them, and style the text up to
// their closing tags; others show as written.
func (b *builder) htmlInline(in *parser.Inline, st *theme.Node) *theme.Node {
	name, closing, ok := htmlTagName(in.Text)
	if !ok {
		b.marked(in.Start, in.End, st, htmlInlineElement(""))
		return st
	}
	rev := b.revealed(in.Start, in.End)
	el := htmlInlineElement(name)
	if name == "br" {
		if rev {
			b.marked(in.Start, in.End, st, el)
		} else {
			b.atom(in.Start, in.End, "\n", child(st, el), nil)
		}
		return st
	}
	if rev {
		b.marked(in.Start, in.End, st, el)
	} else {
		b.pos = in.End
	}
	if closing {
		for k := len(b.htmlTags) - 1; k >= 0; k-- {
			if b.htmlTags[k].name == name {
				prev := b.htmlTags[k].prev
				b.htmlTags = b.htmlTags[:k]
				return prev
			}
		}
		return st
	}
	b.htmlTags = append(b.htmlTags, htmlTag{name: name, prev: st})
	switch name {
	case "b", "strong":
		st = child(st, theme.Element{Type: "strong"})
	case "i", "em":
		st = child(st, theme.Element{Type: "emphasis"})
	case "u", "ins":
		st = child(st, theme.Element{Type: "underline"})
	case "s", "del", "strike":
		st = child(st, theme.Element{Type: "strike"})
	case "mark":
		st = child(st, theme.Element{Type: "mark"})
	case "sup":
		st = child(st, theme.Element{Type: "sup"})
	case "sub":
		st = child(st, theme.Element{Type: "sub"})
	case "kbd":
		st = child(st, theme.Element{Type: "kbd"})
	case "code":
		st = child(st, theme.Element{Type: "code", Classes: []string{"inline"}})
	}
	return st
}

// htmlInlineElement returns the element of an HTML tag shown within text.
func htmlInlineElement(tag string) theme.Element {
	el := theme.Element{Type: "html", Classes: []string{"inline"}}
	if tag != "" {
		el.Attrs = append(el.Attrs, theme.Attr{Name: "tag", Value: tag})
	}
	return el
}

// htmlTagName reads the name of a tag of formatting, and whether it
// closes, from the text of an HTML inline.
func htmlTagName(tag string) (name string, closing, ok bool) {
	if !strings.HasPrefix(tag, "<") || !strings.HasSuffix(tag, ">") {
		return "", false, false
	}
	t := strings.TrimSpace(tag[1 : len(tag)-1])
	t = strings.TrimSuffix(t, "/")
	if strings.HasPrefix(t, "/") {
		closing = true
		t = t[1:]
	}
	t = strings.ToLower(strings.TrimSpace(t))
	if i := strings.IndexAny(t, " \t\n"); i >= 0 {
		t = t[:i]
	}
	switch t {
	case "b", "strong", "i", "em", "u", "ins", "s", "del", "strike", "mark", "sup", "sub", "kbd", "code", "br":
		return t, closing, true
	}
	return "", false, false
}

func isASCIIPunct(c byte) bool {
	return c >= '!' && c <= '/' || c >= ':' && c <= '@' || c >= '[' && c <= '`' || c >= '{' && c <= '~'
}

// texFallback shows TeX that could not be typeset as readable text.
func texFallback(tex string) string {
	return strings.TrimSpace(tex)
}

// display is a leaf's display: its text, pieces and spans.
type display struct {
	text       string
	pieces     []piece
	hasObjects bool
}

func (b *builder) display() display {
	return display{text: b.text.String(), pieces: b.pieces, hasObjects: b.hasObjects}
}

// ensure gives an empty display a piece at the content's start, the one
// place the caret can be.
func (b *builder) ensure(at int) {
	if len(b.pieces) == 0 {
		b.add(piece{kind: pText, s0: at - b.base, s1: at - b.base})
	}
}

// ---------------------------------------------------------------------------
// Mapping between the display and the source

// srcToDisp returns the display offset of the source offset rel (relative
// to the leaf), and whether the caret can be there: in a text piece, or at
// the edge of an atom.
func (d *display) srcToDisp(rel int) (int, bool) {
	for i := range d.pieces {
		p := &d.pieces[i]
		if rel < p.s0 || rel > p.s1 {
			continue
		}
		switch p.kind {
		case pText:
			return p.d0 + ui.UTF16Len(p.text[:rel-p.s0]), true
		case pAtom:
			if rel == p.s0 {
				return p.d0, true
			}
			if rel == p.s1 {
				return p.d1, true
			}
		case pDeco:
			// The caret is beside a decoration, at the piece after it or
			// before it.
		}
	}
	if len(d.pieces) == 0 && rel == 0 {
		return 0, true
	}
	return 0, false
}

// dispToSrc returns the source offset (relative to the leaf) of display
// offset u: of the smallest candidate when several pieces meet there with
// hidden syntax between them and up is set, else of the largest.
func (d *display) dispToSrc(u int, up bool) int {
	best, found := 0, false
	take := func(s int) {
		if !found || (up && s < best) || (!up && s > best) {
			best, found = s, true
		}
	}
	for i := range d.pieces {
		p := &d.pieces[i]
		if u < p.d0 || u > p.d1 {
			continue
		}
		switch p.kind {
		case pText:
			take(p.s0 + utf16ByteOffset(p.text, u-p.d0))
		case pAtom:
			if u == p.d0 {
				take(p.s0)
			} else if u == p.d1 {
				take(p.s1)
			} else if u-p.d0 <= p.d1-u {
				take(p.s0)
			} else {
				take(p.s1)
			}
		case pDeco:
			if u == p.d0 || u == p.d1 || p.d0 == p.d1 {
				take(p.s0)
			} else {
				take(p.s0)
			}
		}
	}
	if !found && len(d.pieces) > 0 {
		last := d.pieces[len(d.pieces)-1]
		if u > last.d1 {
			return last.s1
		}
		return d.pieces[0].s0
	}
	return best
}

// stops returns the source offsets (relative) the caret can take in the
// display, in order.
func (d *display) stops() []int {
	var out []int
	add := func(s int) {
		if len(out) == 0 || out[len(out)-1] != s {
			out = append(out, s)
		}
	}
	for i := range d.pieces {
		p := &d.pieces[i]
		switch p.kind {
		case pText:
			for k := 0; k <= len(p.text); {
				add(p.s0 + k)
				if k == len(p.text) {
					break
				}
				_, n := utf8.DecodeRuneInString(p.text[k:])
				k += n
			}
		case pAtom:
			add(p.s0)
			add(p.s1)
		case pDeco:
			add(p.s0)
		}
	}
	if len(out) == 0 {
		out = append(out, 0)
	}
	return out
}

// utf16ByteOffset returns the byte offset in s of UTF-16 unit u, keeping
// surrogate pairs whole.
func utf16ByteOffset(s string, u int) int {
	if u <= 0 {
		return 0
	}
	n := 0
	for i, r := range s {
		if n >= u {
			return i
		}
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return len(s)
}
