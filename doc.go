// Package livemark is a native WYSIWYG Markdown editor for MyGo: one editing
// surface for the whole document, drawn by MyGo, with the Markdown source
// as its only state. The source is parsed after every edit and rendered as
// Typora renders it: block syntax (#, list markers, >, fences, pipes) is
// never shown, inline syntax (**, `, [](), $, ![]()) shows only around the
// caret, and everything else is drawn as the document reads.
//
// Every caret, selection and edit is a byte offset into the source, so
// nothing is ever serialized: what the editor saves is exactly what the
// user wrote, and an outside change (an AI tool, a file reload) is just
// another edit of the same string.
package livemark

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui/markdown/parser"
)

// leafKind is what a leaf of the document is: the units the editor lays
// out one after another.
type leafKind uint8

const (
	kPara leafKind = iota
	kHeading
	kCode
	kMath
	kHTML
	kRule
	kTable
	// kRaw is source no block holds, as link reference definitions and
	// front matter: shown and edited as plain lines.
	kRaw
	// kPhantom is the empty paragraph the caret sits in before it holds
	// any text (see phantom).
	kPhantom
)

// contKind is what a container around a leaf is.
type contKind uint8

const (
	cQuote contKind = iota
	cItem
	cFootnote
)

// cont is a container a leaf is inside: a quotation, an item of a list or
// a footnote's definition, outermost first in leaf.conts.
type cont struct {
	kind contKind
	// id is the container's block in the parse, which identifies it while
	// the source does not change.
	id int
	// start is the offset of its marker: >, a list item's bullet or
	// number, a footnote's [^.
	start int
	// An item's list: numbered or not, the number this item shows, its
	// bullet (-, *, +) or the delimiter after its number (. or )), whether
	// the list is tight, and how deep the list is nested in other lists.
	ordered bool
	num     int
	marker  byte
	tight   bool
	depth   int
	// task is the item's check box, taskAt the offset of its [.
	task   parser.Task
	taskAt int
	// label is a footnote's.
	label string
}

// line is a line of a code, math, HTML or raw leaf: its text without the
// prefixes of the containers around it, and the offset of that text.
type line struct {
	off  int
	text string
}

// leaf is a block the editor lays out: a paragraph, a heading, a block of
// code or math, a table, a rule, raw lines or the phantom paragraph.
type leaf struct {
	kind leafKind
	// id is the leaf's block in the parse; -1 for raw lines and the
	// phantom.
	id int
	// start and end bound the leaf's editable content in the source: a
	// paragraph's text, a heading's without its #, a code block's lines
	// without the fences, a table from its first cell to its last.
	start, end int
	// lineStart is where the leaf's first line starts, prefixes included,
	// and syntaxStart where the leaf's own syntax starts on it, after the
	// prefixes of its containers: a heading's #, a fence's backticks.
	lineStart   int
	syntaxStart int
	// level is a heading's.
	level int
	// conts are the containers around the leaf, outermost first, and
	// firstOf the indices of those whose first leaf this is, which draw
	// their markers beside it.
	conts   []cont
	firstOf []int
	// lastOf the indices of those whose last leaf this is.
	lastOf []int
	// lines are a code, math, HTML or raw leaf's.
	lines []line
	// A fenced code block's info string, from infoStart to infoEnd on its
	// opening line, whose fence ends at fenceEnd; the language is the
	// info string's first word.
	fenced             bool
	fenceEnd           int
	infoStart, infoEnd int
	lang               string
	// tab is a table's rows and cells.
	tab *table
	// pending marks a heading, item or quotation still being typed: only
	// its marker, without the space after it, which shows as text until
	// the space comes, so that typing # does not flash a heading.
	pending bool
	// setext marks a heading underlined with = or -.
	setext bool
	// frontMatter marks the YAML front matter at the top of a document,
	frontMatter bool

	// Where the last layout put the leaf, in the editor's content.
	x, y, w, h float32
	// view is the leaf as displayed and laid out (see leafView), geom a
	// table's grid, and table the table a cell is in.
	view  *leafView
	geom  *tableGeom
	table *leaf
	// styled is the leaf's theme path and style, of the resolver styleGen
	// made, and styleRev whether its block shows its syntax.
	styled   *leafStyled
	styleGen uint64
	styleRev bool
	sigID    uint32
	sigRev   bool
}

// isText reports whether the leaf is edited as text with inline syntax.
func (l *leaf) isText() bool { return l.kind == kPara || l.kind == kHeading || l.kind == kPhantom }

// innerItem returns the innermost list item around the leaf, if any.
func (l *leaf) innerItem() (cont, bool) {
	for i := len(l.conts) - 1; i >= 0; i-- {
		if l.conts[i].kind == cItem {
			return l.conts[i], true
		}
	}
	return cont{}, false
}

// rowCol returns where a table cell leaf is in its table.
func (l *leaf) rowCol() (row, col int) {
	for r, cells := range l.table.tab.rows {
		for c, cell := range cells {
			if cell == l {
				return r, c
			}
		}
	}
	return -1, -1
}

// alignName returns the name of a table column's alignment.
func alignName(a parser.Align) string {
	switch a {
	case parser.AlignLeft:
		return "left"
	case parser.AlignCenter:
		return "center"
	case parser.AlignRight:
		return "right"
	}
	return ""
}

// opensItem reports whether the leaf is the first of its innermost item.
func (l *leaf) opensItem() bool {
	n := len(l.conts)
	return n > 0 && l.conts[n-1].kind == cItem && slices.Contains(l.firstOf, n-1)
}

// table is a table leaf's model: its rows of cells, each cell a text leaf
// of its own, and how its columns align.
type table struct {
	rows  [][]*leaf
	align []parser.Align
	// delim is the source range of the delimiter row, below the header.
	delim [2]int
}

// cells returns the table's cells in reading order.
func (t *table) cells() []*leaf {
	var out []*leaf
	for _, r := range t.rows {
		out = append(out, r...)
	}
	return out
}

// cols returns the number of columns.
func (t *table) cols() int {
	n := len(t.align)
	for _, r := range t.rows {
		n = max(n, len(r))
	}
	return n
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

// buildLeaves turns the parse of src into the leaves the editor lays out,
// in document order.
func buildLeaves(src string, d *parser.Document, fm int) []*leaf {
	var out []*leaf
	if fm > 0 {
		out = append(out, rawLeaf(src, 0, fm, true))
	}
	var walk func(i int, conts []cont, listDepth int)
	walk = func(i int, conts []cont, listDepth int) {
		prevEnd := fm
		for c := range d.Children(i) {
			b := d.Block(c)
			if i == 0 {
				// Source between top-level blocks that no block holds, as a
				// link reference definition, shows as raw lines.
				out = appendRaw(out, src, prevEnd, b.Start)
				prevEnd = max(prevEnd, spanEnd(src, d, c))
			}
			switch b.Kind {
			case parser.Quote:
				first := len(out)
				walk(c, appendCont(conts, cont{kind: cQuote, id: c, start: b.Start}), listDepth)
				if len(out) == first {
					out = append(out, emptyLeaf(src, b, conts, cont{kind: cQuote, id: c, start: b.Start}))
				}
				markFirst(out[first:], len(conts))
				markLast(out[first:], len(conts))
			case parser.List:
				n := 0
				for it := range d.Children(c) {
					ib := d.Block(it)
					ct := cont{
						kind: cItem, id: it, start: ib.Start,
						ordered: b.Ordered, num: b.Number + n, marker: b.Marker, tight: b.Tight,
						depth: listDepth, task: ib.Task, taskAt: -1,
					}
					if ib.Task != parser.NoTask {
						ct.taskAt = taskBoxAt(src, ib.Start)
					}
					first := len(out)
					walk(it, appendCont(conts, ct), listDepth+1)
					if len(out) == first {
						out = append(out, emptyItemLeaf(src, ib, conts, ct))
					}
					markFirst(out[first:], len(conts))
					markLast(out[first:], len(conts))
					n++
				}
			case parser.FootnoteDefinition:
				first := len(out)
				ct := cont{kind: cFootnote, id: c, start: b.Start, label: b.Label()}
				walk(c, appendCont(conts, ct), listDepth)
				if len(out) == first {
					out = append(out, emptyLeaf(src, b, conts, ct))
				}
				markFirst(out[first:], len(conts))
				markLast(out[first:], len(conts))
			case parser.Paragraph:
				out = append(out, &leaf{kind: kPara, id: c, start: b.Start, end: paraEnd(src, b.End),
					lineStart: lineStartAt(src, b.Start), syntaxStart: b.Start, conts: slices.Clone(conts)})
			case parser.Heading:
				out = append(out, headingLeaf(src, d, c, conts))
			case parser.ThematicBreak:
				out = append(out, &leaf{kind: kRule, id: c, start: b.Start, end: b.End,
					lineStart: lineStartAt(src, b.Start), syntaxStart: b.Start, conts: slices.Clone(conts)})
			case parser.CodeBlock:
				out = append(out, codeLeaf(src, d, c, conts))
			case parser.MathBlock:
				out = append(out, linesLeaf(src, d, c, kMath, conts))
			case parser.HTMLBlock:
				out = append(out, linesLeaf(src, d, c, kHTML, conts))
			case parser.Table:
				out = append(out, tableLeaf(src, d, c, conts))
			}
		}
		if i == 0 {
			out = appendRaw(out, src, prevEnd, len(src))
		}
	}
	walk(0, nil, 0)
	return out
}

// appendCont returns conts with c after them, in a slice of its own.
func appendCont(conts []cont, c cont) []cont {
	out := make([]cont, len(conts)+1)
	copy(out, conts)
	out[len(conts)] = c
	return out
}

// markFirst records that the first of leaves opens the container at depth.
func markFirst(leaves []*leaf, depth int) {
	if len(leaves) > 0 {
		leaves[0].firstOf = append(leaves[0].firstOf, depth)
	}
}

// markLast records that the last of leaves closes the container at depth.
func markLast(leaves []*leaf, depth int) {
	if len(leaves) > 0 {
		leaves[len(leaves)-1].lastOf = append(leaves[len(leaves)-1].lastOf, depth)
	}
}

// paraEnd extends a paragraph's end over the spaces after its last
// character on its line, which the parser leaves out: a space just typed
// at the end of a paragraph shows, and the caret after it.
func paraEnd(src string, end int) int {
	for end < len(src) && (src[end] == ' ' || src[end] == '\t') {
		end++
	}
	return end
}

// headingLeaf makes the leaf of a heading: an ATX heading's text after its
// #s, or a setext heading's text above its underline.
func headingLeaf(src string, d *parser.Document, c int, conts []cont) *leaf {
	b := d.Block(c)
	l := &leaf{kind: kHeading, id: c, level: b.Level, lineStart: lineStartAt(src, b.Start), syntaxStart: b.Start, conts: slices.Clone(conts)}
	// An ATX heading's line starts with its #s, after the indentation.
	p := b.Start
	for p < len(src) && src[p] == ' ' {
		p++
	}
	if p < len(src) && src[p] == '#' {
		marks := p
		for p < len(src) && src[p] == '#' {
			p++
		}
		hashes := p - marks
		if p >= len(src) || src[p] == '\n' || src[p] == '\r' {
			// # alone: still being typed.
			l.pending = true
			l.start, l.end = marks, p
			_ = hashes
			return l
		}
		for p < len(src) && (src[p] == ' ' || src[p] == '\t') {
			p++
		}
		l.start = p
		if ins := d.Inlines(c); len(ins) > 0 {
			l.end = paraEnd(src, max(b.End, p))
		} else {
			l.end = max(p, lineEndAt(src, p))
		}
		if l.end < l.start {
			l.end = l.start
		}
		return l
	}
	// A setext heading: its text is its paragraph's.
	l.setext = true
	l.start, l.end = b.Start, b.End
	return l
}

// codeLeaf makes the leaf of a fenced or indented block of code: its
// lines, and its fence's info string.
func codeLeaf(src string, d *parser.Document, c int, conts []cont) *leaf {
	b := d.Block(c)
	l := linesLeaf(src, d, c, kCode, conts)
	l.fenced = b.Fenced
	if b.Fenced {
		// The opening fence: ``` or ~~~ and the info string after it.
		p := b.Start
		for p < len(src) && src[p] == ' ' {
			p++
		}
		fc := byte('`')
		if p < len(src) {
			fc = src[p]
		}
		for p < len(src) && src[p] == fc {
			p++
		}
		l.fenceEnd = p
		e := lineEndAt(src, p)
		l.infoStart, l.infoEnd = p, e
		for l.infoStart < e && (src[l.infoStart] == ' ' || src[l.infoStart] == '\t') {
			l.infoStart++
		}
		for l.infoEnd > l.infoStart && (src[l.infoEnd-1] == ' ' || src[l.infoEnd-1] == '\t' || src[l.infoEnd-1] == '\r') {
			l.infoEnd--
		}
		l.lang = b.Language()
	}
	return l
}

// linesLeaf makes the leaf of a block of lines: code, math or HTML.
func linesLeaf(src string, d *parser.Document, c int, kind leafKind, conts []cont) *leaf {
	b := d.Block(c)
	l := &leaf{kind: kind, id: c, lineStart: lineStartAt(src, b.Start), syntaxStart: b.Start, conts: slices.Clone(conts)}
	n := d.LineCount(c)
	for k := 0; k < n; k++ {
		text, off := d.Line(c, k)
		// The parser's line is of the source it was given, which may have
		// characters swapped (see parse.CJKFriendly): the line is the source's.
		text = src[off : off+len(sourceLine(d.Source(), text, off))]
		l.lines = append(l.lines, line{off: off, text: strings.TrimSuffix(text, "\r")})
	}
	if kind == kMath || (kind == kCode && b.Fenced) {
		// A fenced block's content lines end before its closing fence; a
		// block opened without content gets the line after its fence, so
		// the caret has somewhere to be.
		if len(l.lines) == 0 {
			open := lineEndAt(src, b.Start)
			l.start, l.end = open, open
			l.lines = nil
			return l
		}
	}
	if len(l.lines) == 0 {
		l.start, l.end = b.Start, b.Start
		return l
	}
	l.start = l.lines[0].off
	last := l.lines[len(l.lines)-1]
	l.end = last.off + len(last.text)
	return l
}

// sourceLine returns the source of a line the parser gives as text from
// off. A tab that the indentation of a container took part of leaves the
// rest of its width as spaces before the text, which are not in the
// source: they are left out, so that the line is the source's bytes.
func sourceLine(src, text string, off int) string {
	for p := 0; p < tabStop && p <= len(text); p++ {
		if p > 0 && text[p-1] != ' ' {
			break
		}
		if e := off + len(text) - p; e <= len(src) && src[off:e] == text[p:] {
			return text[p:]
		}
	}
	return src[off:min(off+len(text), lineEndAt(src, off))]
}

// tabStop is the width of a tab, in columns.
const tabStop = 4

// spanEnd returns where the source of block c ends: the end of its last
// line, past what the parser's End leaves out (a table's last pipe, a
// heading's closing #s or its underline, the closing $$ of math), and of
// the last line of its descendants.
func spanEnd(src string, d *parser.Document, c int) int {
	b := d.Block(c)
	end := lineEndAt(src, b.End)
	switch b.Kind {
	case parser.Heading:
		p := b.Start
		for p < len(src) && (src[p] == ' ' || src[p] == '\t') {
			p++
		}
		if (p >= len(src) || src[p] != '#') && end < len(src) {
			// A setext heading's underline.
			end = lineEndAt(src, end+1)
		}
	case parser.MathBlock:
		if !b.Open {
			// The closing $$ (or \]) ends the content's line, or the next.
			if rest := src[b.End:end]; !strings.Contains(rest, "$$") && !strings.Contains(rest, `\]`) && end < len(src) {
				next := lineEndAt(src, end+1)
				if t := trimPrefixChars(src[end+1 : next]); strings.HasPrefix(t, "$$") || strings.HasPrefix(t, `\]`) {
					end = next
				}
			}
		}
	}
	for ch := range d.Children(c) {
		end = max(end, spanEnd(src, d, ch))
	}
	return end
}

// tableLeaf makes the leaf of a table: its rows of cells.
func tableLeaf(src string, d *parser.Document, c int, conts []cont) *leaf {
	b := d.Block(c)
	t := &table{align: b.Align}
	l := &leaf{kind: kTable, id: c, start: b.Start, end: b.End, lineStart: lineStartAt(src, b.Start), syntaxStart: b.Start, conts: slices.Clone(conts), tab: t}
	for r := range d.Children(c) {
		var row []*leaf
		for cell := range d.Children(r) {
			cb := d.Block(cell)
			start, end := cb.Start, cb.End
			if start == end {
				// An empty cell's text goes after the space that follows its
				// pipe, not against the pipe that closes it.
				start, end = emptyCellAt(src, start), emptyCellAt(src, start)
			}
			row = append(row, &leaf{kind: kPara, id: cell, start: start, end: end, lineStart: lineStartAt(src, cb.Start), syntaxStart: start, table: l})
		}
		t.rows = append(t.rows, row)
	}
	// The delimiter row is the line after the header's.
	h := lineEndAt(src, b.Start)
	if h < len(src) {
		t.delim = [2]int{h + 1, lineEndAt(src, h+1)}
	}
	return l
}

// emptyCellAt returns where the text of an empty cell goes, for the parser's
// position of it at: one space after the pipe before it.
func emptyCellAt(src string, at int) int {
	ls := lineStartAt(src, at)
	q := at - 1
	for q >= ls && src[q] != '|' {
		if src[q] != ' ' && src[q] != '\t' {
			return at
		}
		q--
	}
	if q < ls {
		return at
	}
	if q+1 < at && src[q+1] == ' ' {
		return q + 2
	}
	return q + 1
}

// emptyItemLeaf makes the leaf of a list item without content, "- " or
// "1. ", for the caret to type in after its marker.
func emptyItemLeaf(src string, ib *parser.Block, conts []cont, ct cont) *leaf {
	p := ib.Start
	for p < len(src) && src[p] != ' ' && src[p] != '\t' && src[p] != '\n' && src[p] != '\r' {
		p++
	}
	l := &leaf{kind: kPara, id: -1, lineStart: lineStartAt(src, ib.Start), conts: appendCont(conts, ct)}
	if p >= len(src) || src[p] == '\n' || src[p] == '\r' {
		// "-" alone: a marker still being typed.
		l.pending = true
		l.start, l.end = ib.Start, p
		l.syntaxStart = ib.Start
		return l
	}
	p++ // the space after the marker
	if ct.taskAt >= 0 {
		p = max(p, ct.taskAt+4)
	}
	p = min(p, lineEndAt(src, ib.Start))
	l.start, l.end = p, max(p, lineEndAt(src, p))
	l.syntaxStart = p
	return l
}

// emptyLeaf makes the leaf of a quotation or footnote without content.
func emptyLeaf(src string, b *parser.Block, conts []cont, ct cont) *leaf {
	e := lineEndAt(src, b.Start)
	p := b.Start
	if ct.kind == cQuote {
		p++ // past >
		if p < e && src[p] == ' ' {
			p++
		}
	} else if i := strings.Index(src[b.Start:e], "]:"); i >= 0 {
		p = b.Start + i + 2
		for p < e && src[p] == ' ' {
			p++
		}
	}
	l := &leaf{kind: kPara, id: -1, lineStart: lineStartAt(src, b.Start), conts: appendCont(conts, ct)}
	if ct.kind == cQuote && p == b.Start+1 {
		// ">" alone: still being typed.
		l.pending = true
		l.start, l.end = b.Start, p
		l.syntaxStart = b.Start
		return l
	}
	l.start, l.end = min(p, e), e
	l.syntaxStart = l.start
	return l
}

// rawLeaf makes a leaf of the lines of src from start to end.
func rawLeaf(src string, start, end int, front bool) *leaf {
	l := &leaf{kind: kRaw, id: -1, lineStart: start, syntaxStart: start, frontMatter: front}
	for off := start; off <= end && off < len(src); {
		e := min(lineEndAt(src, off), end)
		l.lines = append(l.lines, line{off: off, text: strings.TrimSuffix(src[off:e], "\r")})
		off = e + 1
	}
	if len(l.lines) == 0 {
		l.start, l.end = start, start
		return l
	}
	l.start = l.lines[0].off
	last := l.lines[len(l.lines)-1]
	l.end = last.off + len(last.text)
	return l
}

// appendRaw adds the non-blank lines of src between from and to, which no
// block holds, as a raw leaf.
func appendRaw(out []*leaf, src string, from, to int) []*leaf {
	if from < 0 || to <= from || from >= len(src) {
		return out
	}
	to = min(to, len(src))
	// Trim blank lines on both sides.
	for from < to {
		e := lineEndAt(src, from)
		if strings.TrimSpace(src[from:min(e, to)]) != "" {
			break
		}
		from = e + 1
	}
	if from >= to {
		return out
	}
	for to > from {
		s := lineStartAt(src, to-1)
		if strings.TrimSpace(src[s:to]) != "" {
			break
		}
		to = s - 1
	}
	if to <= from {
		return out
	}
	from = lineStartAt(src, from)
	return append(out, rawLeaf(src, from, to, false))
}

// taskBoxAt returns the offset of the [ of a task item's check box, on the
// item's first line after its marker.
func taskBoxAt(src string, itemStart int) int {
	e := lineEndAt(src, itemStart)
	if i := strings.IndexByte(src[itemStart:e], '['); i >= 0 {
		return itemStart + i
	}
	return -1
}
