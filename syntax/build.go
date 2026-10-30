package syntax

import (
	"strconv"
	"strings"

	"github.com/CarlOpenLab/livemark/theme"
	"github.com/egoist/mygo/ui/markdown/parser"
)

// node is a node of a tree, as the tree builds it.
type node struct {
	kind Kind
	el   theme.Element
	// start and end bound the node's bytes in the source, end excluded.
	start, end int
	// syntax are the ranges of the node's syntax.
	syntax []Range
	// revStart and revEnd bound the offsets at which the construct shows
	// its syntax, both included, or are -1 for one that never does.
	revStart, revEnd int
	parent           *node
	kids             []*node

	// What the node is, by kind.
	level       int
	setext      bool
	fenced      bool
	lang, info  string
	dest, title string
	label       string
	literal     string
	tag         string
	ordered     bool
	startNum    int
	number      int
	depth       int
	task        Task
	align       string
	auto        bool
}

// builder builds a tree from a parsed document.
type builder struct {
	src string
	doc *parser.Document
}

// build builds the tree of a document parsed from src, whose front matter
// ends at fm.
func (b *builder) build(fm int) *node {
	root := b.new(nil, Document, element("document"), 0, len(b.src))
	if fm > 0 {
		b.frontMatter(root, fm)
	}
	prev := fm
	for c := range b.doc.Children(0) {
		blk := b.doc.Block(c)
		if blk.Start > prev {
			b.raw(root, prev, blk.Start)
		}
		b.block(root, c, 0, 0)
		prev = max(prev, b.spanEnd(c))
	}
	if len(b.src) > prev {
		b.raw(root, prev, len(b.src))
	}
	finish(root)
	return root
}

// new makes a node, as a child of parent when it is not nil.
func (b *builder) new(parent *node, k Kind, el theme.Element, start, end int) *node {
	n := &node{kind: k, el: el, start: start, end: end, parent: parent, revStart: -1}
	if parent != nil {
		parent.kids = append(parent.kids, n)
	}
	return n
}

// element returns an element of type typ.
func element(typ string) theme.Element { return theme.Element{Type: typ} }

// elementClass returns an element of type typ with classes.
func elementClass(typ string, classes ...string) theme.Element {
	return theme.Element{Type: typ, Classes: classes}
}

// withAttr returns the element with an attribute added.
func withAttr(el theme.Element, name, value string) theme.Element {
	el.Attrs = append(el.Attrs, theme.Attr{Name: name, Value: value})
	return el
}

// syntaxInside adds a syntax node within a block, whose own syntax it is,
// and returns it.
func (b *builder) syntaxInside(owner *node, class string, start, end int) *node {
	if end <= start {
		return nil
	}
	owner.syntax = append(owner.syntax, Range{start, end})
	return b.new(owner, Syntax, elementClass("syntax", class), start, end)
}

// syntaxSibling adds a syntax node beside an inline construct, whose
// syntax it is, and returns it.
func (b *builder) syntaxSibling(parent *node, class string, start, end int) *node {
	if end <= start {
		return nil
	}
	return b.new(parent, Syntax, elementClass("syntax", class), start, end)
}

// frontMatter adds the YAML front matter at the top of the source.
func (b *builder) frontMatter(root *node, fm int) {
	n := b.new(root, FrontMatter, element("front-matter"), 0, fm)
	// Its two rules: the first line, and the closing one.
	if e := lineEndAt(b.src, 0); e > 0 {
		n.syntax = append(n.syntax, Range{0, min(3, e)})
	}
	cs := lineStartAt(b.src, max(fm-1, 0))
	n.syntax = append(n.syntax, Range{cs, fm})
}

// raw adds a run of source no block holds: link reference definitions and
// the blank lines around them, shown as plain lines.
func (b *builder) raw(parent *node, from, to int) {
	to = min(to, len(b.src))
	// The first to the last line that is not blank, as the editor's raw
	// leaf sets them.
	start, end := -1, -1
	for off := from; off < to; {
		e := min(lineEndAt(b.src, off), to)
		if strings.TrimSpace(b.src[off:e]) != "" {
			if start < 0 {
				start = off
			}
			end = e
			for end > off && b.src[end-1] == '\r' {
				end--
			}
		}
		off = e + 1
	}
	if start < 0 {
		return
	}
	b.new(parent, LinkDef, element("link-def"), start, end)
}

// block adds the block parsed at i, a child of parent. depth is how deep
// it is in lists, quotes and footnotes.
func (b *builder) block(parent *node, i, depth, quoteDepth int) {
	blk := b.doc.Block(i)
	switch blk.Kind {
	case parser.Paragraph:
		n := b.new(parent, Paragraph, element("paragraph"), blk.Start, b.paraEnd(blk.End))
		b.inlines(n, i)
	case parser.Heading:
		b.heading(parent, i)
	case parser.ThematicBreak:
		n := b.new(parent, Rule, element("rule"), blk.Start, blk.End)
		n.revStart, n.revEnd = lineStartAt(b.src, blk.Start), max(blk.End, lineEndAt(b.src, blk.Start))
		b.syntaxInside(n, "rule", blk.Start, blk.End)
	case parser.CodeBlock:
		b.codeBlock(parent, i)
	case parser.MathBlock:
		b.mathBlock(parent, i)
	case parser.HTMLBlock:
		b.htmlBlock(parent, i)
	case parser.Quote:
		n := b.new(parent, Quote, element("quote"), blk.Start, b.spanEnd(i))
		b.quoteSyntax(n, blk.Start, n.end, quoteDepth)
		for c := range b.doc.Children(i) {
			b.block(n, c, depth, quoteDepth+1)
		}
	case parser.List:
		b.list(parent, i, depth)
	case parser.FootnoteDefinition:
		n := b.new(parent, Footnote, withAttr(element("footnote"), "label", blk.Label()), blk.Start, b.spanEnd(i))
		n.label = blk.Label()
		if first := b.firstChildStart(i); first > blk.Start {
			n.syntax = append(n.syntax, Range{blk.Start, first})
		}
		for c := range b.doc.Children(i) {
			b.block(n, c, depth, quoteDepth)
		}
	case parser.Table:
		b.table(parent, i, depth)
	}
}

// list adds a list and its items.
func (b *builder) list(parent *node, i, depth int) {
	blk := b.doc.Block(i)
	el := elementClass("list")
	if blk.Ordered {
		el.Classes = append(el.Classes, "ordered")
	} else {
		el.Classes = append(el.Classes, "bullet")
	}
	if blk.Tight {
		el.Classes = append(el.Classes, "tight")
	} else {
		el.Classes = append(el.Classes, "loose")
	}
	el = withAttr(el, "depth", strconv.Itoa(depth))
	if blk.Ordered {
		el = withAttr(el, "start", strconv.Itoa(blk.Number))
	}
	n := b.new(parent, List, el, blk.Start, b.spanEnd(i))
	n.ordered, n.startNum, n.depth = blk.Ordered, blk.Number, depth
	num := blk.Number
	for c := range b.doc.Children(i) {
		ib := b.doc.Block(c)
		iel := elementClass("item")
		task := taskOf(ib.Task)
		switch task {
		case Unchecked:
			iel.Classes = append(iel.Classes, "task")
		case Checked:
			iel.Classes = append(iel.Classes, "task", "checked")
		}
		if blk.Ordered {
			iel = withAttr(iel, "number", strconv.Itoa(num))
		}
		iel = withAttr(iel, "depth", strconv.Itoa(depth))
		in := b.new(n, Item, iel, ib.Start, b.spanEnd(c))
		in.number, in.depth, in.task = num, depth, task
		if first := b.firstChildStart(c); first > ib.Start {
			in.syntax = append(in.syntax, Range{ib.Start, first})
		}
		for cc := range b.doc.Children(c) {
			b.block(in, cc, depth+1, 0)
		}
		num++
	}
}

// heading adds a heading: an ATX heading's text after its #s, or a setext
// heading's text above its underline.
func (b *builder) heading(parent *node, i int) {
	blk := b.doc.Block(i)
	src := b.src
	p := blk.Start
	for p < len(src) && (src[p] == ' ' || src[p] == '\t') {
		p++
	}
	level := min(max(blk.Level, 1), 6)
	el := elementClass("heading", "h"+strconv.Itoa(level))
	el = withAttr(el, "level", strconv.Itoa(blk.Level))
	n := b.new(parent, Heading, el, blk.Start, b.paraEnd(blk.End))
	n.level = blk.Level
	if p >= len(src) || src[p] != '#' {
		// Setext: the underline is the line after the text.
		n.setext = true
		n.el.Classes = append(n.el.Classes, "setext")
		if cs := b.paraEnd(blk.End); cs < len(src) {
			ls := cs
			if src[ls] == '\n' {
				ls++
			} else if src[ls] == '\r' && ls+1 < len(src) && src[ls+1] == '\n' {
				ls += 2
			}
			if e := lineEndAt(src, ls); e > ls {
				n.syntax = append(n.syntax, Range{ls, e})
				n.end = max(n.end, e)
			}
		}
	} else {
		hs := p
		for p < len(src) && src[p] == '#' {
			p++
		}
		n.syntax = append(n.syntax, Range{hs, p})
		for p < len(src) && (src[p] == ' ' || src[p] == '\t') {
			p++
		}
		n.syntax = append(n.syntax, b.atxClose(blk.Start)...)
	}
	b.inlines(n, i)
}

// atxClose returns the range of a heading's closing #s, if it has them.
func (b *builder) atxClose(start int) []Range {
	e := lineEndAt(b.src, start)
	q := e
	for q > start && (b.src[q-1] == ' ' || b.src[q-1] == '\t' || b.src[q-1] == '\r') {
		q--
	}
	h := q
	for q > start && b.src[q-1] == '#' {
		q--
	}
	if q == h || q == start || (b.src[q-1] != ' ' && b.src[q-1] != '\t') {
		return nil
	}
	return []Range{{q, h}}
}

// codeBlock adds a fenced or indented block of code, with its fence and
// info string as the syntax the caret shows.
func (b *builder) codeBlock(parent *node, i int) {
	blk := b.doc.Block(i)
	end := b.spanEnd(i)
	el := elementClass("code", "block")
	if blk.Fenced {
		el.Classes = append(el.Classes, "fenced")
	} else {
		el.Classes = append(el.Classes, "indented")
	}
	if lang := blk.Language(); lang != "" {
		el = withAttr(el, "lang", lang)
	}
	if info := blk.Info(); info != "" {
		el = withAttr(el, "info", info)
	}
	n := b.new(parent, Code, el, blk.Start, end)
	n.fenced, n.lang, n.info = blk.Fenced, blk.Language(), blk.Info()
	n.revStart, n.revEnd = lineStartAt(b.src, blk.Start), b.contentEnd(i)
	if blk.Fenced {
		// The opening fence and its info string.
		fs := blk.Start
		fc := byte('`')
		if fs < len(b.src) {
			fc = b.src[fs]
		}
		fe := fs
		for fe < len(b.src) && b.src[fe] == fc {
			fe++
		}
		b.syntaxInside(n, "fence", fs, fe)
		is, ie := trimSpaceRange(b.src, fe, lineEndAt(b.src, fe))
		b.syntaxInside(n, "info", is, ie)
		// The closing fence.
		if ce := b.closingFence(end, fc); ce != nil {
			n.syntax = append(n.syntax, *ce)
		}
	} else {
		// The indentation of each line.
		for k := 0; k < b.doc.LineCount(i); k++ {
			_, off := b.doc.Line(i, k)
			if ls := lineStartAt(b.src, off); ls < off {
				n.syntax = append(n.syntax, Range{ls, off})
			}
		}
	}
	// The reveal range includes the info line, as the editor shows it.
	if blk.Fenced {
		if is, ie := b.codeInfo(i); ie > is {
			n.revStart, n.revEnd = min(n.revStart, lineStartAt(b.src, is)), max(n.revEnd, lineEndAt(b.src, ie))
		}
	}
}

// codeInfo returns the range of a fenced code block's info string.
func (b *builder) codeInfo(i int) (int, int) {
	blk := b.doc.Block(i)
	fs := blk.Start
	fe := fs
	if fe < len(b.src) {
		fc := b.src[fs]
		for fe < len(b.src) && b.src[fe] == fc {
			fe++
		}
	}
	return trimSpaceRange(b.src, fe, lineEndAt(b.src, fe))
}

// closingFence returns the range of a fenced code block's closing fence,
// or nil when it has none.
func (b *builder) closingFence(end int, fc byte) *Range {
	ls := lineStartAt(b.src, max(end-1, 0))
	for ls < end && (b.src[ls] == ' ' || b.src[ls] == '\t') {
		ls++
	}
	if ls < len(b.src) && b.src[ls] == fc {
		e := ls
		for e < len(b.src) && b.src[e] == fc {
			e++
		}
		return &Range{ls, e}
	}
	return nil
}

// mathBlock adds a block of math, its $$ or \[ \] its syntax.
func (b *builder) mathBlock(parent *node, i int) {
	blk := b.doc.Block(i)
	end := b.spanEnd(i)
	n := b.new(parent, Math, elementClass("math", "block"), blk.Start, end)
	n.revStart, n.revEnd = lineStartAt(b.src, blk.Start), b.contentEnd(i)
	open := b.mathOpen(blk.Start)
	if len(open) > 0 {
		n.syntax = append(n.syntax, open...)
	}
	if close := b.mathClose(end); close != nil {
		n.syntax = append(n.syntax, *close)
	}
}

// mathOpen returns the range of a math block's opening delimiter.
func (b *builder) mathOpen(start int) []Range {
	for _, d := range []string{"$$", `\[`} {
		if strings.HasPrefix(b.src[start:], d) {
			return []Range{{start, start + len(d)}}
		}
	}
	return nil
}

// mathClose returns the range of a math block's closing delimiter, which
// is on its last line, or nil.
func (b *builder) mathClose(end int) *Range {
	ls := lineStartAt(b.src, max(end-1, 0))
	for ls < end && (b.src[ls] == ' ' || b.src[ls] == '\t') {
		ls++
	}
	for _, d := range []string{"$$", `\]`} {
		if strings.HasPrefix(b.src[ls:], d) {
			return &Range{ls, ls + len(d)}
		}
	}
	return nil
}

// htmlBlock adds a block of raw HTML.
func (b *builder) htmlBlock(parent *node, i int) {
	blk := b.doc.Block(i)
	n := b.new(parent, HTML, elementClass("html", "block"), blk.Start, b.spanEnd(i))
	n.revStart, n.revEnd = lineStartAt(b.src, blk.Start), b.contentEnd(i)
}

// quoteSyntax adds the > markers of a quote's lines to its syntax.
func (b *builder) quoteSyntax(n *node, start, end, depth int) {
	for off := lineStartAt(b.src, start); off < end; {
		le := lineEndAt(b.src, off)
		p := off
		for p < le && (b.src[p] == ' ' || b.src[p] == '\t') {
			p++
		}
		for k := 0; k <= depth && p < le && b.src[p] == '>'; k++ {
			if k == depth {
				n.syntax = append(n.syntax, Range{p, p + 1})
			}
			p++
			if p < le && b.src[p] == ' ' {
				p++
			}
			for p < le && (b.src[p] == ' ' || b.src[p] == '\t') {
				p++
			}
		}
		off = le + 1
	}
}

// table adds a table, its rows and their cells.
func (b *builder) table(parent *node, i, depth int) {
	blk := b.doc.Block(i)
	n := b.new(parent, Table, element("table"), blk.Start, b.spanEnd(i))
	// The delimiter row is the line after the header's.
	if h := lineEndAt(b.src, blk.Start); h < len(b.src) {
		ds := h + 1
		n.syntax = append(n.syntax, Range{ds, lineEndAt(b.src, ds)})
	}
	row := 0
	for r := range b.doc.Children(i) {
		rb := b.doc.Block(r)
		rel := element("row")
		if rb.Header {
			rel.Classes = append(rel.Classes, "header")
		} else {
			if (row % 2) == 0 {
				rel.Classes = append(rel.Classes, "odd")
			} else {
				rel.Classes = append(rel.Classes, "even")
			}
			row++
		}
		rn := b.new(n, Row, rel, rb.Start, rb.End)
		col := 0
		for c := range b.doc.Children(r) {
			cb := b.doc.Block(c)
			cel := element("cell")
			if rb.Header {
				cel.Classes = append(cel.Classes, "header")
			}
			align := ""
			if col < len(blk.Align) {
				switch blk.Align[col] {
				case parser.AlignLeft:
					align = "left"
				case parser.AlignCenter:
					align = "center"
				case parser.AlignRight:
					align = "right"
				}
			}
			if align != "" {
				cel = withAttr(cel, "align", align)
			}
			cn := b.new(rn, Cell, cel, cb.Start, cb.End)
			cn.align = align
			b.inlines(cn, c)
			col++
		}
	}
}

// taskOf returns a parser task state as a Task.
func taskOf(t parser.Task) Task {
	switch t {
	case parser.Unchecked:
		return Unchecked
	case parser.Checked:
		return Checked
	}
	return NoTask
}

// firstChildStart returns where the first child block of i starts, or 0.
func (b *builder) firstChildStart(i int) int {
	for c := range b.doc.Children(i) {
		return b.doc.Block(c).Start
	}
	return 0
}

// contentEnd returns where the content of a block of lines ends: the end
// of its last line, as the editor's leaf sets it, or the end of its
// opening line when it has no lines.
func (b *builder) contentEnd(i int) int {
	if n := b.doc.LineCount(i); n > 0 {
		text, off := b.doc.Line(i, n-1)
		return off + len(b.mathLine(text, off))
	}
	return lineEndAt(b.src, b.doc.Block(i).Start)
}

// mathLine returns the source of a line the parser gives as text from off
// (see the editor's sourceLine): the text is the source's, at off.
func (b *builder) mathLine(text string, off int) string {
	if off >= 0 && off+len(text) <= len(b.src) && b.src[off:off+len(text)] == text {
		return text
	}
	return b.src[off:min(off+len(text), lineEndAt(b.src, off))]
}

// spanEnd returns where the source of block i ends: the end of its last
// line, past what the parser's End leaves out, and of its descendants'.
func (b *builder) spanEnd(i int) int {
	blk := b.doc.Block(i)
	end := lineEndAt(b.src, blk.End)
	switch blk.Kind {
	case parser.Heading:
		p := blk.Start
		for p < len(b.src) && (b.src[p] == ' ' || b.src[p] == '\t') {
			p++
		}
		if (p >= len(b.src) || b.src[p] != '#') && end < len(b.src) {
			// A setext heading's underline.
			end = lineEndAt(b.src, end+1)
		}
	case parser.MathBlock:
		if !blk.Open {
			if rest := b.src[blk.End:end]; !strings.Contains(rest, "$$") && !strings.Contains(rest, `\]`) && end < len(b.src) {
				next := lineEndAt(b.src, end+1)
				if t := strings.TrimLeft(b.src[end+1:next], " \t"); strings.HasPrefix(t, "$$") || strings.HasPrefix(t, `\]`) {
					end = next
				}
			}
		}
	}
	for c := range b.doc.Children(i) {
		end = max(end, b.spanEnd(c))
	}
	return end
}

// paraEnd extends a paragraph's end over the spaces after its last
// character on its line, which the parser leaves out.
func (b *builder) paraEnd(end int) int {
	for end < len(b.src) && (b.src[end] == ' ' || b.src[end] == '\t') {
		end++
	}
	return end
}

// trimSpaceRange trims spaces and tabs from both ends of a range.
func trimSpaceRange(src string, start, end int) (int, int) {
	for start < end && (src[start] == ' ' || src[start] == '\t' || src[start] == '\r') {
		start++
	}
	for end > start && (src[end-1] == ' ' || src[end-1] == '\t' || src[end-1] == '\r') {
		end--
	}
	return start, end
}

// finish gives each node its place among its siblings, first or last, and
// finishes its children.
func finish(n *node) {
	var first, last *node
	for _, k := range n.kids {
		if k.kind == Syntax {
			continue
		}
		if first == nil {
			first = k
		}
		last = k
	}
	if first != nil {
		first.el.States |= theme.FirstChild
	}
	if last != nil {
		last.el.States |= theme.LastChild
	}
	for _, k := range n.kids {
		if k.kind != Syntax {
			finish(k)
		}
	}
}

// childAt returns the innermost child of n holding off, or nil: the
// smallest range inside it, a syntax range over the construct it marks,
// and at a node's end the node itself.
func childAt(n *node, off int) *node {
	var inside, edge *node
	for _, k := range n.kids {
		switch {
		case off >= k.start && off < k.end:
			inside = narrower(inside, k)
		case k.kind != Syntax && off == k.end && off >= k.start:
			edge = narrower(edge, k)
		}
	}
	if inside != nil {
		return inside
	}
	return edge
}

// narrower returns the narrower of a and b, a syntax range when both are
// as wide.
func narrower(a, b *node) *node {
	if a == nil {
		return b
	}
	wa, wb := a.end-a.start, b.end-b.start
	if wb < wa || wb == wa && a.kind != Syntax && b.kind == Syntax {
		return b
	}
	return a
}

// rangeString returns a range for a debugger.
func rangeString(r Range) string {
	return "[" + strconv.Itoa(r.Start) + "," + strconv.Itoa(r.End) + ")"
}

// lineStartAt returns the offset of the start of the line holding off.
func lineStartAt(src string, off int) int {
	off = min(max(off, 0), len(src))
	return strings.LastIndexByte(src[:off], '\n') + 1
}

// lineEndAt returns the offset of the end of the line holding off, before
// its newline.
func lineEndAt(src string, off int) int {
	off = min(max(off, 0), len(src))
	if i := strings.IndexByte(src[off:], '\n'); i >= 0 {
		return off + i
	}
	return len(src)
}
