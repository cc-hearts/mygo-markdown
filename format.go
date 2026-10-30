package livemark

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/egoist/mygo/ui/richtext"
)

// Inline formats: a command on a selection wraps it in the format's
// markers, or takes them away when the selection is in that format; on a
// caret, it formats the word at it, or types the markers with the caret
// between them.

// inlineAround returns the innermost inline of kind around [a, z] in the
// text unit holding the caret.
func (e *Editor) inlineAround(kind parser.InlineKind, a, z int) (parser.Inline, bool) {
	u := e.unitAt(e.caret)
	if u == nil || u.id < 0 || !(u.kind == kPara || u.kind == kHeading) {
		return parser.Inline{}, false
	}
	var best parser.Inline
	found := false
	for _, in := range e.doc.Inlines(u.id) {
		if in.Kind == kind && in.Start <= a && z <= in.End {
			if !found || in.End-in.Start < best.End-best.Start {
				best, found = in, true
			}
		}
	}
	return best, found
}

// ToggleInline formats the selection with marker (as "**"), or takes the
// format of kind away from it.
func (e *Editor) ToggleInline(kind parser.InlineKind, open, close string) {
	e.layoutIfNeeded()
	if e.phantom != nil {
		e.typeText(open + close)
		e.setCaret(e.caret-len(close), false)
		return
	}
	a, z := e.selection()
	if in, ok := e.inlineAround(kind, a, z); ok {
		// Unwrap: take the inline's markers away.
		ol, cl := markerLens(e.src, in, open, close)
		e.begin(stepEdit)
		e.replace(in.End-cl, in.End, "")
		e.replace(in.Start, in.Start+ol, "")
		if a == z {
			e.commit(e.caret, e.caret)
		} else {
			e.commit(max(a-ol, in.Start), max(z-ol, in.Start))
		}
		return
	}
	if a == z {
		wa, wz := e.wordRangeAt(a)
		if wa < wz && wa <= a && a <= wz && strings.TrimSpace(e.src[wa:wz]) != "" && isWordRune(firstRune(e.src[wa:wz])) {
			a, z = wa, wz
		} else {
			e.begin(stepEdit)
			e.replace(a, a, open+close)
			e.commit(a+len(open), a+len(open))
			return
		}
	}
	// Spaces at the edges stay outside: "** bold**" is no emphasis.
	for a < z && e.src[a] == ' ' {
		a++
	}
	for z > a && e.src[z-1] == ' ' {
		z--
	}
	e.begin(stepEdit)
	e.replace(z, z, close)
	e.replace(a, a, open)
	e.commit(a+len(open), z+len(open))
}

// markerLens returns the lengths of an inline's opening and closing
// markers in the source.
func markerLens(src string, in parser.Inline, open, close string) (int, int) {
	s := src[in.Start:in.End]
	ol, cl := len(open), len(close)
	if in.Kind == parser.Code {
		ol = 0
		for ol < len(s) && s[ol] == '`' {
			ol++
		}
		cl = ol
	}
	if in.Kind == parser.Emphasis || in.Kind == parser.Strong || in.Kind == parser.Strikethrough {
		c := s[0]
		ol = 0
		for ol < len(s) && s[ol] == c {
			ol++
		}
		cl = 0
		for cl < len(s) && s[len(s)-1-cl] == c {
			cl++
		}
		want := 1
		switch in.Kind {
		case parser.Strong:
			want = 2
		case parser.Strikethrough:
			want = min(ol, 2)
		}
		ol, cl = min(ol, want), min(cl, want)
	}
	return min(ol, len(s)), min(cl, len(s)-ol)
}

func firstRune(s string) rune {
	for _, r := range s {
		return r
	}
	return 0
}

// ToggleBold, ToggleItalic, ToggleStrike, ToggleCode and ToggleUnderline
// format the selection.
func (e *Editor) ToggleBold()   { e.ToggleInline(parser.Strong, "**", "**") }
func (e *Editor) ToggleItalic() { e.ToggleInline(parser.Emphasis, "*", "*") }
func (e *Editor) ToggleStrike() { e.ToggleInline(parser.Strikethrough, "~~", "~~") }
func (e *Editor) ToggleCode()   { e.ToggleInline(parser.Code, "`", "`") }

// ToggleUnderline wraps the selection in <u>, or takes it away.
func (e *Editor) ToggleUnderline() {
	a, z := e.selection()
	if a >= 3 && z+4 <= len(e.src) && e.src[a-3:a] == "<u>" && e.src[z:z+4] == "</u>" {
		e.begin(stepEdit)
		e.replace(z, z+4, "")
		e.replace(a-3, a, "")
		e.commit(a-3, z-3)
		return
	}
	e.ToggleInline(parser.HTML, "<u>", "</u>")
}

// InsertLink makes the selection a link, with the caret in its
// destination, or edits the link at the caret.
func (e *Editor) InsertLink() {
	e.layoutIfNeeded()
	a, z := e.selection()
	if in, ok := e.inlineAround(parser.Link, a, z); ok && !in.Auto {
		// Select the link's destination.
		s := e.src[in.Start:in.End]
		if i := strings.LastIndex(s, "]("); i >= 0 {
			ds := in.Start + i + 2
			de := in.End - 1
			if t := strings.IndexAny(e.src[ds:de], " \""); t >= 0 {
				de = ds + t
			}
			e.Select(ds, de)
			return
		}
	}
	if e.phantom != nil {
		e.typeText("[]()")
		e.setCaret(e.caret-3, false)
		return
	}
	sel := e.src[a:z]
	e.begin(stepEdit)
	if strings.HasPrefix(sel, "http://") || strings.HasPrefix(sel, "https://") {
		e.replace(a, z, "["+sel+"]("+sel+")")
		e.commit(a+1, a+1+len(sel))
		return
	}
	e.replace(z, z, "]()")
	e.replace(a, a, "[")
	if a == z {
		e.commit(a+1, a+1)
		return
	}
	c := z + 3
	e.commit(c, c)
}

// InsertImage types an image's Markdown at the caret: its alt text the
// selection, the caret in its destination.
func (e *Editor) InsertImage(path string) {
	text := "![](" + path + ")"
	e.typeText(text)
	if path == "" {
		e.setCaret(e.caret-1, false)
	}
}

// ---------------------------------------------------------------------------
// Paragraph formats

// lineLeaf returns the text leaf at the caret, which paragraph commands act
// on.
func (e *Editor) lineLeaf() *leaf {
	e.layoutIfNeeded()
	u := e.unitAt(e.caret)
	if u == nil || e.isCell(u) {
		return nil
	}
	return u
}

// SetHeading makes the paragraph at the caret a heading of level, or a
// paragraph for level 0.
func (e *Editor) SetHeading(level int) {
	l := e.lineLeaf()
	if l == nil {
		return
	}
	if l.kind == kPhantom {
		if level > 0 {
			e.typeText(strings.Repeat("#", level) + " ")
		}
		return
	}
	if l.kind != kPara && l.kind != kHeading {
		return
	}
	prefix := ""
	if level > 0 {
		prefix = strings.Repeat("#", level) + " "
	}
	caret := e.caret
	e.begin(stepEdit)
	switch {
	case l.kind == kHeading && l.setext:
		end := blockEnd(e.src, l)
		e.replace(lineEndAt(e.src, l.end), end, "")
		e.replace(l.start, l.start, prefix)
		caret += len(prefix)
	case l.kind == kHeading && !l.pending:
		fs := l.syntaxStart
		e.replace(fs, l.start, prefix)
		caret += len(prefix) - (l.start - fs)
	default:
		// The line holding the caret becomes the heading.
		ls := max(lineStartAt(e.src, e.caret), l.start)
		if ls != l.start {
			ls = lineStartAt(e.src, e.caret) + len(e.contPrefix(l))
		}
		e.replace(ls, ls, prefix)
		caret += len(prefix)
	}
	e.commit(caret, caret)
}

// headingLevel returns the level of the heading at the caret, 0 for a
// paragraph.
func (e *Editor) headingLevel() int {
	if l := e.lineLeaf(); l != nil && l.kind == kHeading && !l.pending {
		return l.level
	}
	return 0
}

// ShiftHeading raises (dir > 0) or lowers the heading level at the caret.
func (e *Editor) ShiftHeading(dir int) {
	lv := e.headingLevel()
	switch {
	case lv == 0 && dir > 0:
		e.SetHeading(6)
	case lv == 0:
		return
	case dir > 0 && lv > 1:
		e.SetHeading(lv - 1)
	case dir < 0 && lv < 6:
		e.SetHeading(lv + 1)
	case dir < 0:
		e.SetHeading(0)
	}
}

// ToggleQuote puts the block at the caret in a quotation, or takes it out
// of one.
func (e *Editor) ToggleQuote() {
	l := e.lineLeaf()
	if l == nil {
		return
	}
	if n := len(l.conts); n > 0 && l.conts[n-1].kind == cQuote {
		e.unquote(l)
		return
	}
	if l.kind == kPhantom {
		e.typeText("> ")
		return
	}
	e.prefixLines(l, "> ")
}

// prefixLines puts p at the content start of every line of a leaf.
func (e *Editor) prefixLines(l *leaf, p string) {
	cont := len(e.contPrefix(l))
	start, end := l.lineStart, blockEnd(e.src, l)
	e.indentLines(start, end, p, cont)
}

// ToggleList makes the paragraph at the caret an item of a list of kind
// ("-", "1." or "- [ ]"), or takes it out of its list.
func (e *Editor) ToggleList(marker string) {
	l := e.lineLeaf()
	if l == nil {
		return
	}
	if l.kind == kPhantom {
		e.typeText(marker + " ")
		return
	}
	if l.opensItem() {
		it, _ := l.innerItem()
		task := it.task != parser.NoTask
		same := it.ordered == strings.HasSuffix(marker, ".") && task == strings.Contains(marker, "[")
		if same {
			e.leaveItem(l)
			return
		}
		// Change the kind of list: the item's marker becomes marker.
		caret := e.caret
		e.begin(stepEdit)
		e.replace(it.start, l.start, marker+" ")
		caret += len(marker) + 1 - (l.start - it.start)
		e.commit(caret, caret)
		return
	}
	if l.kind != kPara && l.kind != kHeading {
		return
	}
	at := l.syntaxStart
	caret := e.caret
	e.begin(stepEdit)
	e.replace(at, at, marker+" ")
	e.commit(caret+len(marker)+1, caret+len(marker)+1)
}

// InsertCodeBlock puts the selected lines in a fenced block of code, or
// opens an empty one below the block at the caret.
func (e *Editor) InsertCodeBlock() { e.insertFenced("```", "```") }

// InsertMathBlock opens a block of math.
func (e *Editor) InsertMathBlock() { e.insertFenced("$$", "$$") }

func (e *Editor) insertFenced(open, close string) {
	l := e.lineLeaf()
	if l == nil {
		return
	}
	if l.kind == kPhantom {
		p := e.phantom
		e.begin(stepEdit)
		e.phantom = nil
		cont := phantomPrefix(p.conts)
		text := open + "\n" + cont + "\n" + cont + close
		e.replace(p.at, p.at, p.pre+text+p.post)
		c := p.at + len(p.pre) + len(open)
		if open == "$$" {
			c += 1 + len(cont)
		}
		e.commit(c, c)
		return
	}
	cont := e.contPrefix(l)
	if l.kind == kPara && strings.TrimSpace(e.src[l.start:l.end]) != "" {
		// The paragraph's text goes in the block.
		body := e.src[l.start:l.end]
		e.begin(stepEdit)
		text := open + "\n" + cont + body + "\n" + cont + close
		e.replace(l.start, l.end, text)
		c := l.start + len(open)
		if open == "$$" {
			c += 1 + len(cont)
		}
		e.commit(c, c)
		return
	}
	end := blockEnd(e.src, l)
	blank := blankPrefix(cont)
	text := "\n" + blank + "\n" + cont + open + "\n" + cont + "\n" + cont + close
	e.begin(stepEdit)
	e.replace(end, end, text)
	c := end + len("\n"+blank+"\n"+cont+open)
	if open == "$$" {
		c += 1 + len(cont)
	}
	e.commit(c, c)
}

// InsertTable puts a table of rows and cols below the block at the caret,
// the caret in its first header cell.
func (e *Editor) InsertTable(rows, cols int) {
	l := e.lineLeaf()
	cont := ""
	at := len(e.src)
	pre := "\n\n"
	if l != nil {
		cont = e.contPrefix(l)
		at = blockEnd(e.src, l)
		pre = "\n" + blankPrefix(cont) + "\n" + cont
	}
	if p := e.phantom; p != nil {
		at, pre = p.at, p.pre
		cont = phantomPrefix(p.conts)
	}
	var b strings.Builder
	b.WriteString("|")
	for c := 0; c < cols; c++ {
		b.WriteString("   |")
	}
	b.WriteString("\n" + cont + "|")
	for c := 0; c < cols; c++ {
		b.WriteString(" --- |")
	}
	for r := 1; r < rows; r++ {
		b.WriteString("\n" + cont + "|")
		for c := 0; c < cols; c++ {
			b.WriteString("   |")
		}
	}
	post := ""
	if p := e.phantom; p != nil {
		post = p.post
		e.phantom = nil
	}
	e.begin(stepEdit)
	e.replace(at, at, pre+b.String()+post)
	c := at + len(pre) + 2
	e.commit(c, c)
}

// InsertRule puts a thematic break below the block at the caret.
func (e *Editor) InsertRule() {
	l := e.lineLeaf()
	if l == nil {
		return
	}
	if p := e.phantom; p != nil {
		e.begin(stepEdit)
		e.phantom = nil
		e.replace(p.at, p.at, p.pre+"---"+p.post)
		e.commit(p.at+len(p.pre)+3, p.at+len(p.pre)+3)
		e.setPhantom(&phantom{at: e.caret, pre: "\n\n", conts: p.conts})
		return
	}
	cont := e.contPrefix(l)
	end := blockEnd(e.src, l)
	text := "\n" + blankPrefix(cont) + "\n" + cont + "---"
	e.begin(stepEdit)
	e.replace(end, end, text)
	e.commit(end+len(text), end+len(text))
	e.setPhantom(&phantom{at: e.caret, pre: "\n" + blankPrefix(cont) + "\n" + cont, conts: l.conts})
}

// ToggleTask checks or unchecks the task item the caret is in.
func (e *Editor) ToggleTask() {
	l := e.lineLeaf()
	if l == nil {
		return
	}
	if it, ok := l.innerItem(); ok && it.taskAt >= 0 {
		e.toggleTaskAt(it)
	}
}

// toggleTaskAt checks or unchecks an item's box.
func (e *Editor) toggleTaskAt(it cont) {
	at := it.taskAt
	if at < 0 || at+3 > len(e.src) {
		return
	}
	mark := "x"
	if it.task == parser.Checked {
		mark = " "
	}
	anchor, caret := e.anchor, e.caret
	e.begin(stepEdit)
	e.replace(at+1, at+2, mark)
	e.commit(anchor, caret)
}

// ---------------------------------------------------------------------------
// Tables

// TableCommand runs a command on the table at the caret: "rowAbove",
// "rowBelow", "deleteRow", "colLeft", "colRight", "deleteCol",
// "alignLeft", "alignCenter", "alignRight", "deleteTable".
func (e *Editor) TableCommand(cmd string) {
	e.layoutIfNeeded()
	t, r, c := e.cellPos(e.caret)
	if t == nil {
		return
	}
	lines := e.tableLines(t)
	if len(lines) < 2 {
		return
	}
	switch cmd {
	case "rowAbove":
		if r == 0 {
			r = 1
		}
		e.insertRow(t, r-1, c)
	case "rowBelow":
		e.insertRow(t, r, c)
	case "deleteRow":
		if r == 0 || len(t.tab.rows) <= 1 {
			return
		}
		ln := lines[r+1]
		end := ln[1]
		if end < len(e.src) {
			end++
		}
		e.begin(stepEdit)
		e.replace(ln[0], end, "")
		e.commit(ln[0], ln[0])
	case "colLeft", "colRight":
		at := c
		if cmd == "colRight" {
			at = c + 1
		}
		e.insertColumn(t, lines, at)
	case "deleteCol":
		if t.tab.cols() <= 1 {
			return
		}
		e.deleteColumn(t, lines, c)
	case "alignLeft", "alignCenter", "alignRight":
		e.alignColumn(t, lines, c, cmd)
	case "deleteTable":
		e.removeBlock(t)
	}
}

// tableLines returns the source ranges of a table's lines: the header, the
// delimiter row and the body rows.
func (e *Editor) tableLines(t *leaf) [][2]int {
	var out [][2]int
	end := blockEnd(e.src, t)
	for off := t.lineStart; off <= end; {
		le := lineEndAt(e.src, off)
		out = append(out, [2]int{off, le})
		if le >= end || le >= len(e.src) {
			break
		}
		off = le + 1
	}
	return out
}

// splitRow splits a table line into its prefix and the offsets of its
// pipes.
func splitRow(src string, ln [2]int) (pipes []int) {
	for i := ln[0]; i < ln[1]; i++ {
		switch src[i] {
		case '\\':
			i++
		case '|':
			pipes = append(pipes, i)
		}
	}
	return pipes
}

// insertColumn inserts an empty column before column at.
func (e *Editor) insertColumn(t *leaf, lines [][2]int, at int) {
	e.begin(stepEdit)
	caret := e.caret
	for k := len(lines) - 1; k >= 0; k-- {
		pipes := splitRow(e.src, lines[k])
		if len(pipes) == 0 {
			continue
		}
		cell := "   |"
		if k == 1 {
			cell = " --- |"
		}
		// Before the at-th cell: after its pipe, or at the end.
		pos := -1
		if at < len(pipes) {
			pos = pipes[at] + 1
		}
		if pos < 0 {
			pos = pipes[len(pipes)-1] + 1
		}
		e.replace(pos, pos, cell)
		if caret >= pos {
			caret += len(cell)
		}
	}
	e.commit(e.caret, e.caret)
}

// deleteColumn deletes column at.
func (e *Editor) deleteColumn(t *leaf, lines [][2]int, at int) {
	e.begin(stepEdit)
	for k := len(lines) - 1; k >= 0; k-- {
		pipes := splitRow(e.src, lines[k])
		if at+1 < len(pipes) {
			e.replace(pipes[at]+1, pipes[at+1]+1, "")
		} else if at < len(pipes) && at > 0 {
			e.replace(pipes[at-1]+1, pipes[at]+1, "")
		}
	}
	e.commit(e.caret, e.caret)
	e.setCaret(e.normalize(e.caret, -1), false)
}

// alignColumn sets the alignment of column at in the delimiter row.
func (e *Editor) alignColumn(t *leaf, lines [][2]int, at int, cmd string) {
	pipes := splitRow(e.src, lines[1])
	if at+1 >= len(pipes) {
		return
	}
	spec := " --- "
	switch cmd {
	case "alignLeft":
		spec = " :-- "
	case "alignCenter":
		spec = " :-: "
	case "alignRight":
		spec = " --: "
	}
	e.begin(stepEdit)
	e.replace(pipes[at]+1, pipes[at+1], spec)
	e.commit(e.caret, e.caret)
}

// ---------------------------------------------------------------------------
// The clipboard

// Copy puts the selection on the clipboard: its Markdown as text and as
// Markdown, and as HTML, which pastes with its formatting elsewhere.
func (e *Editor) Copy() bool {
	md := e.SelectedText()
	if md == "" {
		return false
	}
	html := parser.Parse(md).HTML()
	data := transfer.New(transfer.NewItem(
		transfer.Bytes(transfer.Text, []byte(md)),
		transfer.Bytes(transfer.Markdown, []byte(md)),
		transfer.Bytes(transfer.HTML, []byte(html)),
	))
	if e.opts.WriteClipboard != nil {
		e.opts.WriteClipboard(data)
	} else {
		e.services.WriteClipboardData(data)
	}
	return true
}

// Cut copies the selection and deletes it.
func (e *Editor) Cut() bool {
	if !e.Copy() {
		return false
	}
	e.deleteSelection()
	return true
}

// Paste inserts the clipboard at the selection: Markdown as it is, an
// image saved next to the document, HTML turned into Markdown, or text;
// plain pastes text only.
func (e *Editor) Paste(plain bool) {
	var data transfer.Data
	if e.opts.ReadClipboard != nil {
		data = e.opts.ReadClipboard()
	} else {
		data = e.services.ReadClipboardData(transfer.Markdown, transfer.PNG, transfer.FileList, transfer.HTML, transfer.Text)
	}
	if !plain {
		if b, err := data.Read(transfer.Markdown); err == nil && len(b) > 0 {
			e.insertText(string(b))
			return
		}
		if e.opts.SaveImage != nil {
			if b, err := data.Read(transfer.PNG); err == nil && len(b) > 0 {
				if path, err := e.opts.SaveImage(b, ".png"); err == nil {
					e.typeText("![](" + linkDest(path) + ")")
					return
				}
			}
			if files, err := data.Files(); err == nil && len(files) > 0 {
				var parts []string
				for _, f := range files {
					if !isImagePath(f) {
						continue
					}
					b, err := os.ReadFile(f)
					if err != nil {
						continue
					}
					if path, err := e.opts.SaveImage(b, strings.ToLower(filepath.Ext(f))); err == nil {
						parts = append(parts, "!["+strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))+"]("+linkDest(path)+")")
					}
				}
				if len(parts) > 0 {
					e.insertText(strings.Join(parts, "\n\n"))
					return
				}
			}
		}
		if b, err := data.Read(transfer.HTML); err == nil && richHTML(string(b)) {
			if md := strings.TrimSpace(richtext.ParseHTML(string(b)).Markdown()); md != "" {
				e.insertText(md)
				return
			}
		}
	}
	if b, err := data.Read(transfer.Text); err == nil && len(b) > 0 {
		e.insertText(strings.ReplaceAll(string(b), "\r\n", "\n"))
	}
}

// richHTML reports whether HTML has formatting worth keeping, rather than
// the colored spans an editor of code copies.
func richHTML(html string) bool {
	low := strings.ToLower(html)
	for _, tag := range []string{"<h1", "<h2", "<h3", "<h4", "<ul", "<ol", "<table", "<a ", "<strong", "<b>", "<em>", "<i>", "<blockquote", "<img"} {
		if strings.Contains(low, tag) {
			return true
		}
	}
	return false
}

// dropFiles takes files dropped on the editor: images go in at the caret,
// saved next to the document, and the others go to the app.
func (e *Editor) dropFiles(paths []string) {
	var images, others []string
	for _, p := range paths {
		if isImagePath(p) {
			images = append(images, p)
		} else {
			others = append(others, p)
		}
	}
	var parts []string
	for _, f := range images {
		path := filepath.ToSlash(f)
		if e.opts.SaveImage != nil {
			if b, err := os.ReadFile(f); err == nil {
				if p, err := e.opts.SaveImage(b, strings.ToLower(filepath.Ext(f))); err == nil {
					path = p
				}
			}
		}
		parts = append(parts, "!["+strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))+"]("+linkDest(path)+")")
	}
	if len(parts) > 0 {
		e.insertText(strings.Join(parts, "\n\n"))
	}
	if len(others) > 0 && e.opts.OnDropFiles != nil {
		e.opts.OnDropFiles(others)
	}
}

// linkDest writes a path as a link's destination: in angle brackets when
// it has spaces.
func linkDest(p string) string {
	if strings.ContainsAny(p, " ()") {
		return "<" + p + ">"
	}
	return p
}

func isImagePath(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg":
		return true
	}
	return false
}

// insertText inserts pasted text at the selection, its lines continuing
// the containers the caret is in, as one step of undo.
func (e *Editor) insertText(text string) {
	if text == "" {
		return
	}
	e.layoutIfNeeded()
	if p := e.phantom; p != nil {
		cont := phantomPrefix(p.conts)
		text = strings.ReplaceAll(text, "\n", "\n"+cont)
		e.begin(stepEdit)
		e.phantom = nil
		e.replace(p.at, p.at, p.pre+text+p.post)
		c := p.at + len(p.pre) + len(text)
		e.commit(c, c)
		return
	}
	a, z := e.selection()
	if u := e.unitAt(e.caret); u != nil {
		if e.isCell(u) {
			text = strings.ReplaceAll(strings.ReplaceAll(text, "|", `\|`), "\n", " ")
		} else if cont := e.contPrefix(u); cont != "" {
			text = strings.ReplaceAll(text, "\n", "\n"+cont)
		}
	}
	e.begin(stepEdit)
	e.replace(a, z, text)
	e.commit(a+len(text), a+len(text))
}
