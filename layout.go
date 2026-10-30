package livemark

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"

	"github.com/CarlOpenLab/livemark/internal/mdoc"
	"github.com/CarlOpenLab/livemark/theme"
)

// baseSize returns the size of the body text, the font-size of the
// document's root style.
func (e *Editor) baseSize() float32 { return e.rootStyle().FontSize }

// baseLine returns the body's line height as a multiple of its size, or
// the font's own when the theme sets none.
func (e *Editor) baseLine() float32 {
	if lh := e.rootStyle().LineHeight; lh > 0 {
		return lh
	}
	return 1.75
}

// leafView is a leaf as the editor shows it: its display, laid out, and
// the boxes around it, relative to the leaf's box.
type leafView struct {
	disp   display
	layout *ui.TextLayout
	// empty tells a layout of nothing: shaped from a space, for its height.
	empty bool
	// tx and ty place the layout in the leaf's box.
	tx, ty float32
	h      float32
	// panel fills the box behind the text: code, math being edited, front
	// matter.
	panel bool
	// block is an object drawn below the text, or alone: a formula, an
	// image of HTML; bx and by place it.
	block  *object
	bx, by float32
	// label is a code block's language, drawn in its corner.
	label string
	// reveal tells a block showing its source because the caret is in
	// it: a rule, a formula, a fence's info string.
	reveal bool
	// natural is the width of a table cell's text on one line.
	natural float32
	// font is the text's, which the caret is as tall as.
	font ui.Font
	// diagram is a Mermaid diagram drawn instead of its code.
	diagram *mdoc.Diagram
	// style is the theme's computed style of the leaf's block, and info
	// that of a code block's info string.
	style *theme.Computed
	info  *theme.Computed
	used  uint64
}

// viewKey is what a leaf's view is made of: views of the same key are the
// same, and the editor keeps them across frames.
type viewKey struct {
	kind      leafKind
	level     int
	flags     uint8
	width     float32
	reveal    int
	compose   string
	composeAt int
	text      string
	styleGen  uint64
	styleID   uint64
	assets    uint64
	align     parser.Align
}

const (
	fPending uint8 = 1 << iota
	fSetext
	fHeader
	fFront
	fQuote
	fNatural
	fFenced
)

// leafFlags returns the flags of a leaf's key.
func leafFlags(l *leaf, header bool) uint8 {
	var f uint8
	if l.pending {
		f |= fPending
	}
	if l.setext {
		f |= fSetext
	}
	if header {
		f |= fHeader
	}
	if l.frontMatter {
		f |= fFront
	}
	for _, c := range l.conts {
		if c.kind == cQuote {
			f |= fQuote
			break
		}
	}
	return f
}

// revealAt returns the caret's offset relative to the leaf's first line
// when it shows the syntax of the leaf, or -1.
func (e *Editor) revealAt(l *leaf) int {
	return e.revealFor(l, e.revealCaret())
}

// view returns the leaf's view at width, as it shows with the caret where
// it is.
func (e *Editor) view(l *leaf, width float32, header bool, align parser.Align) *leafView {
	return e.viewWith(l, width, header, align, e.revealAt(l))
}

// viewWith returns the leaf's view at width as it shows with the caret at
// reveal (relative to the leaf's first line; -1 for elsewhere), from the
// cache when it has it.
func (e *Editor) viewWith(l *leaf, width float32, header bool, align parser.Align, reveal int) *leafView {
	k := viewKey{
		kind:     l.kind,
		level:    l.level,
		flags:    leafFlags(l, header),
		width:    width,
		reveal:   reveal,
		styleGen: e.styleGen,
		styleID:  e.leafStyleID(l),
		assets:   e.assetGen.Load(),
		align:    align,
	}
	at := -1
	if reveal >= 0 {
		at = reveal + l.lineStart
	}
	withCompose := e.compose != "" && at == e.caret
	if withCompose {
		k.compose, k.composeAt = e.compose, e.caret-l.lineStart
	}
	end := l.end
	if l.kind == kRule {
		end = max(end, l.lineStart)
	}
	k.text = e.src[l.lineStart:max(end, l.lineStart)]
	if l.kind == kCode || l.kind == kMath {
		k.text = e.src[l.lineStart:max(l.end, lineEndAt(e.src, l.lineStart))]
	}
	if v, ok := e.views[k]; ok {
		v.used = e.frame
		return v
	}
	var v *leafView
	switch l.kind {
	case kCode:
		v = e.codeView(l, width, at, withCompose)
	case kMath:
		v = e.mathView(l, width, at, withCompose)
	case kHTML:
		v = e.htmlView(l, width, at, withCompose)
	case kRaw:
		v = e.linesView(l, width, at, withCompose, l.frontMatter)
	case kRule:
		v = e.ruleView(l, width, at, withCompose)
	default:
		v = e.textView(l, width, at, withCompose, header, align)
	}
	v.used = e.frame
	e.views[k] = v
	return v
}

// newBuilder returns a builder of the display of leaf l, with the caret
// at at (-1 for elsewhere), and the composition there when withCompose.
func (e *Editor) newBuilder(l *leaf, at int, withCompose bool, size, width float32) *builder {
	b := &builder{e: e, src: e.src, base: l.lineStart, reveal: at, fontSize: size, width: width}
	if withCompose {
		b.compose, b.composeAt = e.compose, e.caret
	}
	return b
}

// textFont returns the font of a text leaf, from its theme style.
func (e *Editor) textFont(l *leaf) ui.Font {
	if l.kind == kHeading && l.pending {
		// A heading being typed is still text.
		return e.styleFont(e.rootStyle())
	}
	return e.styleFont(e.leafStyle(l))
}

// textView lays out a paragraph, a heading, a table cell or the phantom.
func (e *Editor) textView(l *leaf, width float32, at int, withCompose, header bool, align parser.Align) *leafView {
	f := e.textFont(l)
	b := e.newBuilder(l, at, withCompose, f.Size, width)
	st := e.textNode(l)
	switch {
	case l.pending || l.kind == kPhantom || l.id < 0:
		b.textRun(l.start, l.end, st)
		b.finish(l.end)
	default:
		ins := e.doc.Inlines(l.id)
		b.alone = aloneImage(ins, e.src)
		b.inlines(ins, l.start, l.end, st)
	}
	b.ensure(l.start)
	v := &leafView{disp: b.display()}
	shape := ui.TextShape{Width: width}
	switch align {
	case parser.AlignCenter:
		shape.Align = ui.Center
	case parser.AlignRight:
		shape.Align = ui.End
	}
	if b.alone && !b.revealedAny {
		shape.Align = ui.Center
	}
	v.layout, v.empty = shapeDisplay(&v.disp, f, shape)
	v.font = f
	_, v.h = v.layout.Size()
	if width == 0 {
		v.natural, _ = v.layout.Size()
	}
	return v
}

// shapeDisplay lays out a display, or a space for an empty one.
func shapeDisplay(d *display, f ui.Font, shape ui.TextShape) (*ui.TextLayout, bool) {
	if d.text == "" {
		return ui.ShapeRichTextWith([]ui.Span{{Text: " "}}, f, shape), true
	}
	spans := make([]ui.Span, 0, len(d.pieces))
	for _, p := range d.pieces {
		if p.text == "" {
			continue
		}
		sp := p.span
		sp.Text = p.text
		spans = append(spans, sp)
	}
	return ui.ShapeRichTextWith(spans, f, shape), false
}

// aloneImage reports whether a paragraph's inlines are one image and
// nothing else but spaces.
func aloneImage(ins []parser.Inline, src string) bool {
	img := false
	for i := 0; i < len(ins); i = max(ins[i].Next, i+1) {
		switch ins[i].Kind {
		case parser.Image:
			if img {
				return false
			}
			img = true
		case parser.Text:
			if strings.TrimSpace(src[ins[i].Start:ins[i].End]) != "" {
				return false
			}
		default:
			return false
		}
	}
	return img
}

// codeView lays out a block of code: its lines, highlighted, in a panel,
// with its fence's info string above them while the caret is there.
func (e *Editor) codeView(l *leaf, width float32, at int, withCompose bool) *leafView {
	st := e.styledLeaf(l)
	cs := st.style
	f := e.styleFont(cs)
	padL, padR, padT, padB := cs.Padding.Left, cs.Padding.Right, cs.Padding.Top, cs.Padding.Bottom
	inner := max(width-padL-padR, 40)
	b := e.newBuilder(l, at, withCompose, f.Size, inner)
	caretIn := at >= 0
	v := &leafView{style: cs, panel: true, tx: padL, ty: padT, label: l.lang}
	if l.fenced {
		v.info = e.styleNode(l).Pseudo("info", 0)
	}
	plain := e.styleSpan(cs)
	infoLine := l.fenced && caretIn && b.reveal >= l.infoStart && b.reveal <= l.infoEnd
	if infoLine {
		node := e.styleNode(l)
		fenceStyle := e.styleSpan(node.Child(syntaxElement("")).Style())
		infoStyle := e.styleSpan(node.Child(syntaxElement("info")).Style())
		fence := strings.TrimLeft(e.src[l.lineStart:l.fenceEnd], " \t>")
		b.deco(l.infoStart, fence, fenceStyle, nil)
		b.spanRun(l.infoStart, l.infoEnd, infoStyle)
		if len(l.lines) > 0 {
			b.spanAtom(l.infoEnd, l.lines[0].off, "\n", plain)
		}
		v.reveal = true
	}
	lang := editor.LanguageNamed(l.lang)
	var state uint64
	var toks []editor.Token
	for k, ln := range l.lines {
		if k > 0 {
			prev := l.lines[k-1]
			b.spanAtom(prev.off+len(prev.text), ln.off, "\n", plain)
		}
		if lang == nil || lang.Tokenizer == nil {
			b.codeRun(ln.off, ln.off+len(ln.text), plain)
			continue
		}
		toks, state = lang.Tokenizer.Tokenize(ln.text, state, toks[:0])
		at := 0
		for _, tk := range toks {
			end := min(tk.End, len(ln.text))
			if end <= at {
				continue
			}
			sp := e.styleSpan(tokenStyle(st, tk.Kind))
			b.codeRun(ln.off+at, ln.off+end, sp)
			at = end
		}
		b.codeRun(ln.off+at, ln.off+len(ln.text), plain)
	}
	b.finish(l.end)
	b.ensure(l.start)
	v.disp = b.display()
	v.layout, v.empty = shapeDisplay(&v.disp, f, ui.TextShape{Width: inner})
	v.font = f
	_, th := v.layout.Size()
	v.h = th + padT + padB
	if !caretIn && strings.EqualFold(l.lang, "mermaid") {
		if d, ok := mdoc.ParseDiagram(e.codeText(l)); ok {
			v.diagram = &d
			v.h = d.Height(width) + padT + padB
		}
	}
	return v
}

// codeText returns the text of a code or math leaf's lines.
func (e *Editor) codeText(l *leaf) string {
	var sb strings.Builder
	for k, ln := range l.lines {
		if k > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(ln.text)
	}
	return sb.String()
}

// mathView lays out a block of math: typeset, or its TeX above the
// typeset formula while the caret is in it.
func (e *Editor) mathView(l *leaf, width float32, at int, withCompose bool) *leafView {
	cs := e.leafStyle(l)
	caretIn := at >= 0
	tex := e.codeText(l)
	obj := e.mathBlockObject(tex, width)
	if !caretIn {
		if obj == nil {
			// Nothing to typeset, or TeX the typesetter cannot read: its
			// source shows, in the panel of code.
			return e.linesViewState(l, width, -1, false, true, theme.Broken)
		}
		v := &leafView{style: cs, block: obj}
		v.bx, v.by = (width-obj.w)/2, cs.Padding.Top
		v.h = obj.h + cs.Padding.Top + cs.Padding.Bottom
		return v
	}
	v := e.linesView(l, width, at, withCompose, true)
	v.reveal = true
	if obj != nil {
		gap := e.baseSize() * 0.6
		v.block = obj
		v.bx, v.by = (width-obj.w)/2, v.h+gap
		v.h += obj.h + 2*gap
	}
	return v
}

// htmlView lays out a block of HTML: its source, or the image of a block
// of just an image while the caret is elsewhere.
func (e *Editor) htmlView(l *leaf, width float32, at int, withCompose bool) *leafView {
	html := e.codeText(l)
	if obj := e.htmlImageObject(html, width); obj != nil {
		if at < 0 {
			cs := e.leafStyle(l)
			return &leafView{style: cs, block: obj, bx: (width - obj.w) / 2, h: obj.h}
		}
		v := e.linesView(l, width, at, withCompose, false)
		gap := e.baseSize() * 0.5
		v.block, v.bx, v.by = obj, (width-obj.w)/2, v.h+gap
		v.h += obj.h + gap
		return v
	}
	return e.linesView(l, width, at, withCompose, false)
}

// linesView lays out a leaf's lines as they are, in its block's font.
func (e *Editor) linesView(l *leaf, width float32, at int, withCompose, panel bool) *leafView {
	return e.linesViewState(l, width, at, withCompose, panel, 0)
}

// linesViewState is linesView with extra states of the block, as a
// formula that could not be typeset is :broken.
func (e *Editor) linesViewState(l *leaf, width float32, at int, withCompose, panel bool, states theme.States) *leafView {
	cs := e.leafStyleState(l, states)
	f := e.styleFont(cs)
	padL, padR, padT, padB := cs.Padding.Left, cs.Padding.Right, cs.Padding.Top, cs.Padding.Bottom
	inner := max(width-padL-padR, 40)
	v := &leafView{style: cs, panel: panel, tx: padL, ty: padT}
	b := e.newBuilder(l, at, withCompose, f.Size, inner)
	sp := ui.Span{Color: uiColor(cs.Color)}
	for k, ln := range l.lines {
		if k > 0 {
			prev := l.lines[k-1]
			b.spanAtom(prev.off+len(prev.text), ln.off, "\n", sp)
		}
		b.codeRun(ln.off, ln.off+len(ln.text), sp)
	}
	b.finish(l.end)
	b.ensure(l.start)
	v.disp = b.display()
	v.layout, v.empty = shapeDisplay(&v.disp, f, ui.TextShape{Width: inner})
	v.font = f
	_, th := v.layout.Size()
	v.h = th + padT + padB
	return v
}

// ruleView lays out a thematic break: a line, or its source while the
// caret is at it.
func (e *Editor) ruleView(l *leaf, width float32, at int, withCompose bool) *leafView {
	cs := e.leafStyle(l)
	if at < 0 {
		return &leafView{style: cs, h: cs.Height}
	}
	f := e.styleFont(cs)
	b := e.newBuilder(l, at, withCompose, f.Size, width)
	b.spanRun(l.start, l.end, e.styleSpan(e.styleNode(l).Child(syntaxElement("")).Style()))
	b.finish(l.end)
	b.ensure(l.start)
	v := &leafView{style: cs, disp: b.display(), reveal: true}
	v.layout, v.empty = shapeDisplay(&v.disp, f, ui.TextShape{Width: width})
	v.font = f
	_, v.h = v.layout.Size()
	return v
}

// ---------------------------------------------------------------------------
// The document

// indentOf returns how far the containers move a leaf in: their
// padding-left.
func (e *Editor) indentOf(conts []cont) float32 {
	x := float32(0)
	n := e.res.Root()
	for _, c := range conts {
		n = appendContNode(n, c)
		x += n.Style().Padding.Left
	}
	return x
}

// margins returns the room above and below a leaf, from its theme style.
func (e *Editor) margins(l *leaf) (top, bottom float32) {
	if l.kind == kHeading && l.pending {
		// A heading being typed is still text.
		s := e.baseSize()
		return s * 0.7, s * 0.7
	}
	c := e.leafStyle(l)
	return c.Margin.Top, c.Margin.Bottom
}

// tightAfter reports whether a leaf follows the one before it closely: an
// item of a tight list, or the content of an item after its first leaf.
func tightAfter(prev, l *leaf) bool {
	if prev == nil {
		return false
	}
	if it, ok := l.innerItem(); ok && it.tight {
		if _, pin := prev.innerItem(); pin {
			return true
		}
	}
	return false
}

// layoutDoc lays the leaves out one below the other, in the content
// column of the element, from x to x+width.
func (e *Editor) layoutDoc(x, width float32) {
	y := e.padTop
	var prev *leaf
	prevBottom := float32(0)
	for _, l := range e.leaves {
		var ind float32
		for _, cs := range e.styledLeaf(l).contStyle {
			ind += cs.Padding.Left
		}
		l.x = x + ind
		l.w = max(width-ind, 60)
		top, bottom := e.margins(l)
		if prev == nil {
			top = 0
		} else if tightAfter(prev, l) {
			// The blocks of a tight list's item follow each other closely:
			// their own top margin, not the room the one before leaves.
			prevBottom = 0
		}
		y += max(top, prevBottom)
		l.y = y
		switch l.kind {
		case kTable:
			e.layoutTable(l)
		default:
			l.view = e.view(l, l.w, false, parser.AlignNone)
			l.h = l.view.h
		}
		y += l.h
		prevBottom = bottom
		prev = l
	}
	e.docH = y + e.padBottom
}

// layoutTable lays a table out: its columns as wide as their widest
// cells, narrowed to the table's room when they would not fit, and its
// rows as tall as their tallest cells.
func (e *Editor) layoutTable(l *leaf) {
	t := l.tab
	cols := t.cols()
	natural := make([]float32, cols)
	for r, row := range t.rows {
		for c, cell := range row {
			cs := e.leafStyle(cell)
			v := e.view(cell, 0, r == 0, alignAt(t.align, c))
			natural[c] = max(natural[c], v.natural+cs.Padding.Left+cs.Padding.Right+2)
		}
	}
	total := float32(0)
	minW := e.baseSize() * 3
	for c := range natural {
		natural[c] = max(natural[c], minW)
		total += natural[c]
	}
	widths := natural
	if total > l.w {
		// Narrow the columns in proportion, none below the narrowest.
		widths = make([]float32, cols)
		room := l.w
		for c := range natural {
			widths[c] = max(natural[c]*room/total, minW)
		}
	}
	g := &tableGeom{colX: make([]float32, cols+1), colW: widths}
	for c := range widths {
		g.colX[c+1] = g.colX[c] + widths[c]
	}
	g.rowY = make([]float32, len(t.rows)+1)
	for r, row := range t.rows {
		h := e.baseSize() * e.baseLine()
		padT, padB := float32(0), float32(0)
		for c, cell := range row {
			if c >= cols {
				break
			}
			cs := e.leafStyle(cell)
			v := e.view(cell, max(widths[c]-cs.Padding.Left-cs.Padding.Right, 10), r == 0, alignAt(t.align, c))
			cell.view = v
			h = max(h, v.h)
			padT, padB = max(padT, cs.Padding.Top), max(padB, cs.Padding.Bottom)
		}
		h += padT + padB
		g.rowY[r+1] = g.rowY[r] + h
		for c, cell := range row {
			if c >= cols {
				break
			}
			cs := e.leafStyle(cell)
			cell.x = l.x + g.colX[c] + cs.Padding.Left
			cell.y = l.y + g.rowY[r] + cs.Padding.Top
			cell.w = max(widths[c]-cs.Padding.Left-cs.Padding.Right, 10)
			cell.h = cell.view.h
		}
	}
	g.w = g.colX[cols]
	l.geom = g
	l.h = g.rowY[len(t.rows)]
}

// tableGeom is where a table's columns and rows are, relative to its box.
type tableGeom struct {
	colX, colW []float32
	rowY       []float32
	w          float32
}

func alignAt(a []parser.Align, c int) parser.Align {
	if c < len(a) {
		return a[c]
	}
	return parser.AlignNone
}

// pruneViews forgets the views no frame has used for a while.
func (e *Editor) pruneViews() {
	if e.frame%120 != 0 {
		return
	}
	for k, v := range e.views {
		if e.frame-v.used > 240 {
			delete(e.views, k)
		}
	}
}
