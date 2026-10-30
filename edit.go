package livemark

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui/markdown/parser"
)

// Editing works on the source, as Typora's editing reads: typing goes in
// at the caret, and the structure of the document follows from what the
// source becomes, so that "# " makes a heading and "- " a list. Keys that
// change structure (Enter, Backspace at the start of a block, Tab) write
// the Markdown that structure is, in the containers the caret is in.

// unitAt returns the unit the caret at off is in, and whether it is a
// table's cell.
func (e *Editor) unitAt(off int) *leaf {
	us := e.units()
	if i := e.unitIndex(us, off); i >= 0 {
		return us[i]
	}
	return nil
}

// leafOf returns the leaf a unit is, or the table a cell is in.
func (e *Editor) leafOf(u *leaf) *leaf {
	if t := e.tableOf(u); t != nil {
		return t
	}
	return u
}

// contPrefix returns what a continuation line of a leaf starts with, to
// stay in the leaf's containers: its first line's prefix with list markers
// blanked, "> " kept.
func (e *Editor) contPrefix(l *leaf) string {
	if l.kind == kPhantom {
		return phantomPrefix(l.conts)
	}
	end := min(max(l.syntaxStart, l.lineStart), len(e.src))
	return blankMarkers(e.src[l.lineStart:end])
}

// blankMarkers turns a line's prefix into a continuation line's: list
// markers and task boxes become spaces, quotation markers stay.
func blankMarkers(p string) string {
	b := []byte(p)
	for i, c := range b {
		if c != '>' && c != ' ' && c != '\t' {
			b[i] = ' '
		}
	}
	return string(b)
}

// blankPrefix returns what a blank line inside a leaf's containers is:
// its quotation markers.
func blankPrefix(cont string) string {
	if i := strings.LastIndexByte(cont, '>'); i >= 0 {
		return cont[:i+1]
	}
	return ""
}

// phantomPrefix returns the prefix of a line in containers.
func phantomPrefix(conts []cont) string {
	var b strings.Builder
	for _, c := range conts {
		if c.kind == cQuote {
			b.WriteString("> ")
		} else {
			b.WriteString("  ")
		}
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Typing

// typeText inserts text typed at the selection, as the keyboard and input
// methods type it: pairs of brackets close, a selection typed over with a
// bracket is wrapped, a third backtick opens a block of code.
func (e *Editor) typeText(text string) {
	if text == "" {
		return
	}
	e.layoutIfNeeded()
	if p := e.phantom; p != nil {
		e.begin(stepTyping)
		e.phantom = nil
		at := p.at
		ins, c := text, len(text)
		if block, k, ok := blockFor(text, phantomPrefix(p.conts)); ok {
			ins, c = block, k
		} else if r := []rune(text); len(r) == 1 {
			if closeCh, ok := autoPairs[r[0]]; ok {
				ins = text + string(closeCh)
			}
		}
		e.replace(at, at, p.pre+ins+p.post)
		c += at + len(p.pre)
		e.commit(c, c)
		return
	}
	a, z := e.selection()
	u := e.unitAt(e.caret)
	inCode := u != nil && (u.kind == kCode || u.kind == kMath || u.kind == kHTML || u.kind == kRaw)
	cell := u != nil && e.isCell(u)
	if r := []rune(text); len(r) == 1 {
		ch := r[0]
		if cell && ch == '|' {
			text = `\|`
		} else if a != z {
			if closeCh, ok := wrapPairs[ch]; ok && (inCode || ch != '\'') {
				e.begin(stepEdit)
				e.replace(z, z, string(closeCh))
				e.replace(a, a, string(ch))
				e.commit(a+len(string(ch)), z+len(string(ch)))
				return
			}
		} else {
			if e.skipCloser(ch) {
				return
			}
			if e.autoPair(ch, inCode) {
				return
			}
		}
	}
	if a == z && !inCode && !cell && e.blockShortcut(u, text) {
		return
	}
	kind := stepTyping
	if a != z {
		kind = stepEdit
	}
	e.begin(kind)
	e.replace(a, z, text)
	c := a + len(text)
	e.commit(c, c)
}

// wrapPairs are what a selection typed over with an opening character is
// wrapped in.
var wrapPairs = map[rune]rune{
	'(': ')', '[': ']', '{': '}', '"': '"', '\'': '\'', '`': '`', '*': '*', '_': '_', '~': '~',
	'（': '）', '【': '】', '《': '》', '“': '”', '‘': '’', '「': '」',
}

// autoPairs are the brackets typing an opener closes.
var autoPairs = map[rune]rune{'(': ')', '[': ']', '{': '}', '"': '"', '（': '）', '【': '】', '《': '》', '“': '”', '「': '」'}

// autoPair types an opening bracket and its closer, the caret between
// them, where nothing but space or a closer follows. It reports whether
// it did.
func (e *Editor) autoPair(ch rune, inCode bool) bool {
	closeCh, ok := autoPairs[ch]
	if !ok && !(inCode && (ch == '\'' || ch == '`')) {
		return false
	}
	if !ok {
		closeCh = ch
	}
	next := e.runeAfter(e.caret)
	if next != 0 && next != ' ' && next != '\n' && next != '\t' && !strings.ContainsRune(")]}>\"'`）】》”」", next) {
		return false
	}
	if ch == '"' || ch == '\'' || ch == '`' {
		// A quote after a word is an apostrophe or a closing quote.
		if prev := e.runeBefore(e.caret); isWordRune(prev) {
			return false
		}
	}
	e.begin(stepTyping)
	s := string(ch) + string(closeCh)
	e.replace(e.caret, e.caret, s)
	c := e.caret - len(string(closeCh))
	e.commit(c, c)
	return true
}

// skipCloser moves the caret over the closer typed right before it. It
// reports whether it did.
func (e *Editor) skipCloser(ch rune) bool {
	if !strings.ContainsRune(")]}\"'`）】》”」", ch) || e.runeAfter(e.caret) != ch {
		return false
	}
	// Only a closer that a pair opened: the text before has its opener.
	before := e.src[lineStartAt(e.src, e.caret):e.caret]
	open := map[rune]rune{')': '(', ']': '[', '}': '{', '）': '（', '】': '【', '》': '《', '”': '“', '」': '「'}[ch]
	if open != 0 && strings.Count(before, string(open)) <= strings.Count(before, string(ch)) {
		return false
	}
	if open == 0 && strings.Count(before, string(ch))%2 == 0 {
		return false
	}
	e.breakCoalescing()
	e.setCaret(e.caret+len(string(ch)), false)
	return true
}

func (e *Editor) runeAfter(off int) rune {
	if off >= len(e.src) {
		return 0
	}
	r := []rune(e.src[off:min(off+4, len(e.src))])
	if len(r) == 0 {
		return 0
	}
	return r[0]
}

func (e *Editor) runeBefore(off int) rune {
	if off <= 0 {
		return 0
	}
	s := e.src[max(0, off-4):off]
	r := []rune(s)
	if len(r) == 0 {
		return 0
	}
	return r[len(r)-1]
}

// blockShortcut turns what is typed at the start of an empty paragraph
// into a block, as it is typed, so that the rest of the document is never
// a block of code or math for a moment: a third ` or ~ opens a fenced
// block of code, a second $ a block of math, with its closing fence. It
// reports whether it did.
func (e *Editor) blockShortcut(u *leaf, text string) bool {
	if u == nil || !u.isText() || u.kind == kHeading && !u.pending {
		return false
	}
	ls := lineStartAt(e.src, e.caret)
	rest := e.src[e.caret:lineEndAt(e.src, e.caret)]
	if strings.TrimSpace(rest) != "" || u.start > e.caret {
		return false
	}
	content := e.src[max(u.start, ls):e.caret]
	prefix := e.src[ls : e.caret-len(content)]
	if strings.Trim(prefix, " \t>") != "" && !u.opensItem() {
		return false
	}
	cont := e.contPrefix(u)
	block, c, ok := blockFor(content+text, cont)
	if !ok {
		return false
	}
	start := e.caret - len(content)
	e.begin(stepEdit)
	e.replace(start, e.caret+len(rest), block)
	e.commit(start+c, start+c)
	return true
}

// blockFor returns the block that text typed alone on a line opens, with
// its closing fence, and where the caret goes in it: ``` and ~~~ a block
// of code (the caret in its info string), $$ a block of math.
func blockFor(text, cont string) (string, int, bool) {
	switch text {
	case "```", "~~~":
		return text + "\n" + cont + "\n" + cont + text, len(text), true
	case "$$":
		return "$$\n" + cont + "\n" + cont + "$$", 3 + len(cont), true
	}
	return "", 0, false
}

// ---------------------------------------------------------------------------
// Deleting

// deleteSelection deletes the selection, as one step of undo.
func (e *Editor) deleteSelection() {
	a, z := e.selection()
	if a == z {
		return
	}
	e.begin(stepEdit)
	e.replace(a, z, "")
	e.commit(a, a)
	e.tidyEmpty()
}

// tidyEmpty makes the empty paragraph a deletion leaves behind a phantom,
// so the caret stays in a line of its own, and takes its blank lines out
// of the source.
func (e *Editor) tidyEmpty() {
	if e.leafAt(e.caret) != nil || e.phantom != nil {
		return
	}
	// The caret is between blocks: put it in a phantom paragraph there,
	// on the blank lines the text left.
	off := e.caret
	s := lineStartAt(e.src, off)
	if strings.TrimSpace(e.src[s:lineEndAt(e.src, off)]) != "" {
		return
	}
	// The blank lines around the caret.
	a := s
	for a > 0 {
		p := lineStartAt(e.src, a-1)
		if strings.TrimSpace(strings.Trim(e.src[p:a-1], ">")) != "" {
			break
		}
		a = p
	}
	z := lineEndAt(e.src, off)
	for z < len(e.src) {
		n := lineEndAt(e.src, z+1)
		if strings.TrimSpace(strings.Trim(e.src[z+1:n], ">")) != "" {
			break
		}
		z = n
	}
	// Keep a paragraph's break: one blank line between what is above and
	// below.
	switch {
	case a == 0 && z == len(e.src):
		e.begin(stepEdit)
		e.replace(0, len(e.src), "")
		e.mergeLastStep()
		e.setPhantom(&phantom{at: 0})
	case a == 0:
		e.begin(stepEdit)
		e.replace(0, z+1, "")
		e.mergeLastStep()
		e.setPhantom(&phantom{at: 0, post: "\n\n", before: true})
	case z == len(e.src):
		e.begin(stepEdit)
		e.replace(a-1, len(e.src), "\n")
		e.mergeLastStep()
		e.setPhantom(&phantom{at: a - 1, pre: "\n"})
	default:
		e.begin(stepEdit)
		e.replace(a-1, z, "\n")
		e.mergeLastStep()
		e.setPhantom(&phantom{at: a - 1, pre: "\n", post: "\n"})
	}
}

// mergeLastStep commits the open step into the last one, as the tidying
// that belongs to an edit.
func (e *Editor) mergeLastStep() {
	s := e.hist.open
	e.hist.open = nil
	if s == nil || len(s.changes) == 0 {
		return
	}
	if n := len(e.hist.undo); n > 0 {
		last := &e.hist.undo[n-1]
		last.changes = append(last.changes, s.changes...)
		last.closedForCoalescing = true
	}
	e.rev++
	e.parsed = 0
	e.sync()
}

// backspace deletes backward: the selection, the grapheme or word before
// the caret, or at the start of a block, the block's structure.
func (e *Editor) backspace(word bool) {
	e.layoutIfNeeded()
	if e.anchor != e.caret {
		e.deleteSelection()
		return
	}
	if p := e.phantom; p != nil {
		// An empty paragraph goes away: to the end of the leaf before it.
		at := p.at
		e.dropPhantom()
		if p.before {
			e.setCaret(e.normalize(at, 1), false)
		} else {
			e.setCaret(e.normalize(at, -1), false)
		}
		return
	}
	off := e.caret
	u := e.unitAt(off)
	if u == nil {
		return
	}
	if first, ok := e.edgeStop(u, false); ok && off <= first && !(u.kind == kCode && u.fenced && off >= u.infoStart && off <= u.infoEnd && off > u.infoStart) {
		e.backspaceAtStart(u)
		return
	}
	prev := e.step(off, -1, word)
	if prev >= off {
		return
	}
	if !word {
		// One grapheme of what shows, which may be an atom standing for
		// more source, as a line break and the prefix after it.
		e.begin(stepDelete)
	} else {
		e.begin(stepEdit)
	}
	e.replace(prev, off, "")
	e.commit(prev, prev)
	e.tidyEmpty()
}

// backspaceAtStart handles Backspace at the start of a unit: a heading
// becomes a paragraph, an item leaves its list, a quotation's paragraph
// leaves it, a paragraph joins the one before.
func (e *Editor) backspaceAtStart(u *leaf) {
	l := e.leafOf(u)
	if e.isCell(u) {
		// To the end of the cell before.
		e.setCaret(e.step(u.start, -1, false), false)
		return
	}
	switch {
	case l.kind == kCode || l.kind == kMath:
		if strings.TrimSpace(e.codeText(l)) == "" {
			e.removeBlock(l)
			return
		}
		if l.kind == kCode && l.fenced && e.caret != l.infoStart {
			e.setCaret(l.infoEnd, false)
			return
		}
		e.setCaret(e.step(e.caret, -1, false), false)
		return
	case l.kind == kHeading && !l.pending:
		e.begin(stepEdit)
		if l.setext {
			end := blockEnd(e.src, l)
			e.replace(lineEndAt(e.src, l.end), end, "")
		} else {
			e.replace(l.syntaxStart, l.start, "")
		}
		e.commit(e.caret, e.caret)
		return
	case l.opensItem():
		e.leaveItem(l)
		return
	}
	if n := len(l.conts); n > 0 && l.conts[n-1].kind == cQuote && slices_contains(l.firstOf, n-1) {
		e.unquote(l)
		return
	}
	// Join the leaf before.
	i := e.indexOf(l)
	if i <= 0 {
		return
	}
	prev := e.leaves[i-1]
	switch prev.kind {
	case kPara, kHeading:
		if prev.pending {
			break
		}
		e.begin(stepEdit)
		e.replace(prev.end, l.start, "")
		e.commit(prev.end, prev.end)
	case kRule:
		e.begin(stepEdit)
		e.replace(prev.lineStart, l.lineStart, "")
		c := e.caret
		e.commit(c, c)
	case kPhantom:
	default:
		e.setCaret(e.step(e.caret, -1, false), false)
	}
}

// slices_contains reports whether s holds v.
func slices_contains(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// indexOf returns the index of a leaf among the leaves.
func (e *Editor) indexOf(l *leaf) int {
	for i, x := range e.leaves {
		if x == l {
			return i
		}
	}
	return -1
}

// removeBlock deletes a leaf's source, its lines, and puts the caret in a
// phantom paragraph in its place.
func (e *Editor) removeBlock(l *leaf) {
	start, end := l.lineStart, blockEnd(e.src, l)
	if end < len(e.src) {
		end++
	}
	e.begin(stepEdit)
	e.replace(start, end, "")
	e.commit(start, start)
	if e.leafAt(start) == nil || e.leafAt(start).lineStart != start {
		e.tidyEmpty()
		if e.phantom == nil {
			e.setPhantom(&phantom{at: start, post: "\n", before: true, conts: l.conts})
		}
	}
}

// deleteForward deletes forward: the selection, the grapheme or word after
// the caret, or at the end of a block, joins the block after.
func (e *Editor) deleteForward(word bool) {
	e.layoutIfNeeded()
	if e.anchor != e.caret {
		e.deleteSelection()
		return
	}
	if e.phantom != nil {
		at := e.phantom.at
		e.dropPhantom()
		e.setCaret(e.normalize(at, 1), false)
		return
	}
	off := e.caret
	u := e.unitAt(off)
	if u == nil {
		return
	}
	if last, ok := e.edgeStop(u, true); ok && off >= last {
		l := e.leafOf(u)
		if e.isCell(u) || l.kind == kCode || l.kind == kMath || l.kind == kHTML || l.kind == kRaw {
			return
		}
		i := e.indexOf(l)
		if i < 0 || i+1 >= len(e.leaves) {
			return
		}
		next := e.leaves[i+1]
		switch next.kind {
		case kPara, kHeading:
			e.begin(stepEdit)
			e.replace(l.end, next.start, "")
			e.commit(l.end, l.end)
		case kRule:
			e.begin(stepEdit)
			end := blockEnd(e.src, next)
			if end < len(e.src) {
				end++
			}
			e.replace(lineEndAt(e.src, l.end), end-1, "")
			e.commit(off, off)
		}
		return
	}
	next := e.step(off, 1, word)
	if next <= off {
		return
	}
	e.begin(stepDelete)
	e.replace(off, next, "")
	e.commit(off, off)
	e.tidyEmpty()
}

// ---------------------------------------------------------------------------
// Enter

// enter breaks the line at the caret as Typora does: a new paragraph, a
// new item, the next row of a table, a new line of code.
func (e *Editor) enter(shift bool) {
	e.layoutIfNeeded()
	if e.anchor != e.caret {
		e.deleteSelection()
	}
	if p := e.phantom; p != nil {
		// An empty paragraph in a container leaves it.
		if len(p.conts) > 0 {
			np := *p
			np.conts = p.conts[:len(p.conts)-1]
			np.pre = "\n\n" + phantomPrefix(np.conts)
			if np.before {
				np.pre = ""
				np.post = "\n\n"
			}
			e.setPhantom(&np)
		}
		return
	}
	off := e.caret
	u := e.unitAt(off)
	if u == nil {
		return
	}
	if e.isCell(u) {
		e.nextRow()
		return
	}
	l := u
	cont := e.contPrefix(l)
	switch l.kind {
	case kCode, kMath, kHTML, kRaw:
		if l.kind == kCode && l.fenced && off >= l.infoStart && off <= l.infoEnd {
			// From the info string into the code.
			if len(l.lines) == 0 {
				e.begin(stepEdit)
				e.replace(l.infoEnd, l.infoEnd, "\n"+cont)
				c := l.infoEnd + 1 + len(cont)
				e.commit(c, c)
				return
			}
			e.setCaret(l.start, false)
			return
		}
		indent := ""
		if l.kind == kCode {
			ls := e.lineContentStart(l, off)
			for i := ls; i < off && (e.src[i] == ' ' || e.src[i] == '\t'); i++ {
				indent += string(e.src[i])
			}
		}
		e.begin(stepEdit)
		ins := "\n" + cont + indent
		e.replace(off, off, ins)
		e.commit(off+len(ins), off+len(ins))
		return
	case kRule:
		e.setPhantom(&phantom{at: blockEnd(e.src, l), pre: "\n\n" + blankPrefixTo(cont), conts: l.conts})
		return
	}
	// Text: a paragraph, a heading, an item.
	if l.pending {
		e.begin(stepEdit)
		e.replace(off, off, "\n"+cont)
		e.commit(off+1+len(cont), off+1+len(cont))
		return
	}
	if shift && l.kind != kHeading {
		// A line break within the paragraph.
		e.begin(stepEdit)
		ins := "\n" + cont
		e.replace(off, off, ins)
		e.commit(off+len(ins), off+len(ins))
		return
	}
	if l.kind == kPara && e.tableShortcut(l) {
		return
	}
	if l.opensItem() {
		e.enterItem(l, off)
		return
	}
	first, _ := e.edgeStop(l, false)
	last, _ := e.edgeStop(l, true)
	blank := blankPrefixTo(cont)
	switch {
	case off >= last:
		// A new paragraph below, empty until typed in.
		e.setPhantom(&phantom{at: blockEnd(e.src, l), pre: "\n" + blank + "\n" + cont, conts: l.conts})
	case off <= first && strings.TrimSpace(e.src[l.start:l.end]) != "":
		// A new paragraph above.
		at := l.syntaxStart
		e.setPhantom(&phantom{at: at, post: "\n" + blank + "\n" + cont, conts: l.conts, before: true})
	default:
		// Spaces at the break go with it.
		from, to := off, off
		for from > l.start && e.src[from-1] == ' ' {
			from--
		}
		for to < l.end && e.src[to] == ' ' {
			to++
		}
		e.begin(stepEdit)
		ins := "\n" + blank + "\n" + cont
		e.replace(from, to, ins)
		e.commit(from+len(ins), from+len(ins))
	}
}

// blankPrefixTo returns the blank line's prefix of a continuation prefix.
func blankPrefixTo(cont string) string { return blankPrefix(cont) }

// lineContentStart returns where the content of the line holding off
// starts in a leaf of lines.
func (e *Editor) lineContentStart(l *leaf, off int) int {
	for _, ln := range l.lines {
		if off >= ln.off && off <= ln.off+len(ln.text) {
			return ln.off
		}
	}
	return lineStartAt(e.src, off)
}

// enterItem breaks an item at the caret: the text after it goes to a new
// item of the list, or on an empty item, the item leaves the list.
func (e *Editor) enterItem(l *leaf, off int) {
	it, _ := l.innerItem()
	if strings.TrimSpace(e.src[l.start:l.end]) == "" {
		e.leaveItem(l)
		return
	}
	lineStart := lineStartAt(e.src, it.start)
	outer := blankMarkers(e.src[lineStart:it.start])
	marker := e.nextMarker(it)
	sep := "\n"
	if !it.tight {
		sep = "\n" + blankPrefix(outer) + "\n"
	}
	ins := sep + outer + marker
	e.begin(stepEdit)
	// Spaces before the break go with it.
	from := off
	for from > l.start && e.src[from-1] == ' ' {
		from--
	}
	to := off
	for to < l.end && e.src[to] == ' ' {
		to++
	}
	e.replace(from, to, ins)
	e.commit(from+len(ins), from+len(ins))
}

// nextMarker returns the marker of the item after it: the same bullet, the
// next number, an unchecked box for a task.
func (e *Editor) nextMarker(it cont) string {
	m := string(it.marker) + " "
	if it.ordered {
		m = strconv.Itoa(it.num+1) + string(it.marker) + " "
	} else if it.marker == 0 {
		m = "- "
	}
	if it.task != parser.NoTask {
		m += "[ ] "
	}
	return m
}

// leaveItem takes an item out of its list: a nested item goes up a level,
// one at the top becomes a paragraph after the list.
func (e *Editor) leaveItem(l *leaf) {
	it, _ := l.innerItem()
	nested := 0
	for _, c := range l.conts {
		if c.kind == cItem {
			nested++
		}
	}
	if nested > 1 {
		e.outdentItem(l)
		return
	}
	lineStart := lineStartAt(e.src, it.start)
	outer := e.src[lineStart:it.start]
	empty := strings.TrimSpace(e.src[l.start:l.end]) == ""
	if empty {
		// An empty item leaves its list: its line goes, and an empty
		// paragraph takes its place, below the items above it.
		prefix := blankMarkers(outer)
		blank := blankPrefix(prefix)
		var conts []cont
		for _, c := range l.conts {
			if c.kind != cItem {
				conts = append(conts, c)
			}
		}
		le := lineEndAt(e.src, lineStart)
		e.begin(stepEdit)
		if le >= len(e.src) {
			from := max(lineStart-1, 0)
			e.replace(from, le, "")
			e.commit(from, from)
			pre := "\n" + blank + "\n" + prefix
			if from == 0 {
				pre = prefix
			}
			e.setPhantom(&phantom{at: from, pre: pre, conts: conts})
			return
		}
		e.replace(lineStart, le+1, "")
		e.commit(lineStart, lineStart)
		pre := blank + "\n" + prefix
		if lineStart == 0 {
			pre = prefix
		}
		e.setPhantom(&phantom{at: lineStart, pre: pre, post: "\n" + blank + "\n", before: true, conts: conts})
		return
	}
	e.begin(stepEdit)
	// The marker, the spaces after it and a task's box go; the text stays.
	e.replace(it.start, l.start, "")
	pos := it.start
	// The text becomes a paragraph of its own: apart from the item above.
	if lineStart > 0 && e.leafBefore(l) != nil {
		e.replace(lineStart, lineStart, blankPrefix(blankMarkers(outer))+"\n")
		pos += len(blankPrefix(blankMarkers(outer))) + 1
	}
	e.commit(pos, pos)
}

// leafBefore returns the leaf before l, or nil.
func (e *Editor) leafBefore(l *leaf) *leaf {
	if i := e.indexOf(l); i > 0 {
		return e.leaves[i-1]
	}
	return nil
}

// unquote takes a leaf out of its innermost quotation: its lines lose the
// quotation's marker.
func (e *Editor) unquote(l *leaf) {
	end := blockEnd(e.src, l)
	q := l.conts[len(l.conts)-1]
	depth := 0
	for _, c := range l.conts {
		if c.kind == cQuote {
			depth++
		}
	}
	e.begin(stepEdit)
	// From the last line up, so the offsets above stay.
	var starts []int
	for off := lineStartAt(e.src, max(q.start, l.lineStart)); off <= end; {
		starts = append(starts, off)
		n := lineEndAt(e.src, off)
		if n >= end || n >= len(e.src) {
			break
		}
		off = n + 1
	}
	caret := e.caret
	for i := len(starts) - 1; i >= 0; i-- {
		s := starts[i]
		le := lineEndAt(e.src, s)
		// The depth-th > of the line.
		seen, at := 0, -1
		for k := s; k < le; k++ {
			if e.src[k] == '>' {
				seen++
				if seen == depth {
					at = k
					break
				}
			} else if e.src[k] != ' ' && e.src[k] != '\t' {
				break
			}
		}
		if at < 0 {
			continue
		}
		n := 1
		if at+1 < le && e.src[at+1] == ' ' {
			n = 2
		}
		e.replace(at, at+n, "")
		if caret > at {
			caret -= min(n, caret-at)
		}
	}
	e.commit(caret, caret)
}

// tableShortcut turns a paragraph of one line of cells between pipes into
// a table, at Enter: "| a | b |" makes a table of two columns with an empty
// row for the caret. It reports whether it did.
func (e *Editor) tableShortcut(l *leaf) bool {
	text := strings.TrimSpace(e.src[l.start:l.end])
	if strings.Contains(text, "\n") || len(text) < 3 || text[0] != '|' || text[len(text)-1] != '|' {
		return false
	}
	cells := strings.Split(text[1:len(text)-1], "|")
	if len(cells) == 0 {
		return false
	}
	cont := e.contPrefix(l)
	delim, row := "|", "|"
	for range cells {
		delim += " --- |"
		row += "   |"
	}
	ins := "\n" + cont + delim + "\n" + cont + row
	end := l.end
	e.begin(stepEdit)
	e.replace(end, end, ins)
	// The caret goes in the first cell of the new row.
	c := end + len(ins) - len(row) + 2
	e.commit(c, c)
	return true
}

// ---------------------------------------------------------------------------
// Tables

// cellPos returns the table, row and column of the cell holding off.
func (e *Editor) cellPos(off int) (*leaf, int, int) {
	u := e.unitAt(off)
	if u == nil {
		return nil, -1, -1
	}
	t := e.tableOf(u)
	if t == nil {
		return nil, -1, -1
	}
	_, r, c := cellAt(t, off)
	return t, r, c
}

// moveCell moves the caret to the end of the cell dir cells away, through
// the rows; past the last, a new row.
func (e *Editor) moveCell(dir int) {
	t, r, c := e.cellPos(e.caret)
	if t == nil {
		return
	}
	cols := t.tab.cols()
	idx := r*cols + c + dir
	if idx < 0 {
		return
	}
	nr, nc := idx/cols, idx%cols
	if nr >= len(t.tab.rows) {
		e.appendRow(t, 0)
		return
	}
	row := t.tab.rows[nr]
	if nc >= len(row) {
		nc = len(row) - 1
	}
	cell := row[nc]
	e.Select(cell.start, cell.end)
}

// nextRow moves the caret to the same column of the next row; past the
// last, a new row.
func (e *Editor) nextRow() {
	t, r, c := e.cellPos(e.caret)
	if t == nil {
		return
	}
	if r+1 >= len(t.tab.rows) {
		e.appendRow(t, c)
		return
	}
	row := t.tab.rows[r+1]
	cell := row[min(c, len(row)-1)]
	e.setCaret(cell.end, false)
}

// appendRow adds an empty row at the end of a table and puts the caret in
// its column col.
func (e *Editor) appendRow(t *leaf, col int) {
	e.insertRow(t, len(t.tab.rows)-1, col)
}

// insertRow inserts an empty row after row r (the header is row 0) and
// puts the caret in its column col.
func (e *Editor) insertRow(t *leaf, r, col int) {
	cols := t.tab.cols()
	if cols == 0 {
		return
	}
	cont := blankMarkers(e.src[t.lineStart:t.start])
	row := "|" + strings.Repeat("   |", cols)
	// After the row's line, or the delimiter row's for the header.
	var at int
	if r <= 0 {
		at = t.tab.delim[1]
	} else {
		at = lineEndAt(e.src, t.tab.rows[r][0].start)
	}
	ins := "\n" + cont + row
	e.begin(stepEdit)
	e.replace(at, at, ins)
	c := at + 1 + len(cont) + 2 + col*4
	e.commit(c, c)
}

// ---------------------------------------------------------------------------
// Indentation

// tab indents (or with shift, outdents) the item at the caret, moves to the
// next cell of a table, or indents a line of code.
func (e *Editor) tab(shift bool) bool {
	e.layoutIfNeeded()
	if e.phantom != nil {
		return true
	}
	u := e.unitAt(e.caret)
	if u == nil {
		return false
	}
	if e.isCell(u) {
		if shift {
			e.moveCell(-1)
		} else {
			e.moveCell(1)
		}
		return true
	}
	if u.kind == kCode {
		e.indentCode(u, shift)
		return true
	}
	if _, ok := u.innerItem(); ok {
		if shift {
			e.outdentItem(u)
		} else {
			e.indentItem(u)
		}
		return true
	}
	return true
}

// indentCode indents the lines of code the selection touches, or types
// spaces at the caret.
func (e *Editor) indentCode(l *leaf, shift bool) {
	unit := "    "
	if strings.EqualFold(l.lang, "go") || strings.EqualFold(l.lang, "makefile") {
		unit = "\t"
	}
	a, z := e.selection()
	if a == z && !shift {
		e.begin(stepTyping)
		e.replace(a, a, unit)
		e.commit(a+len(unit), a+len(unit))
		return
	}
	e.begin(stepEdit)
	anchor, caret := e.anchor, e.caret
	for i := len(l.lines) - 1; i >= 0; i-- {
		ln := l.lines[i]
		if ln.off > z || ln.off+len(ln.text) < a {
			continue
		}
		if shift {
			n := 0
			for n < len(ln.text) && n < len(unit) && (ln.text[n] == ' ' || ln.text[n] == '\t') {
				n++
				if ln.text[n-1] == '\t' {
					break
				}
			}
			e.replace(ln.off, ln.off+n, "")
		} else {
			e.replace(ln.off, ln.off, unit)
		}
	}
	_ = anchor
	_ = caret
	e.commit(e.anchor, e.caret)
}

// itemRange returns the source range of the item a leaf is in, from the
// start of its first line to the end of its last.
func (e *Editor) itemRange(it cont) (int, int) {
	b := e.doc.Block(it.id)
	end := lineEndAt(e.src, b.End)
	for c := it.id + 1; c < b.Next && c < e.doc.Len(); c++ {
		end = max(end, lineEndAt(e.src, e.doc.Block(c).End))
	}
	return lineStartAt(e.src, it.start), end
}

// itemSiblings returns the items of the list an item is in.
func (e *Editor) itemSiblings(it cont) []int {
	parent := e.doc.Block(it.id).Parent
	var out []int
	for c := range e.doc.Children(parent) {
		out = append(out, c)
	}
	return out
}

// indentItem makes the item at the caret a child of the item before it.
func (e *Editor) indentItem(l *leaf) {
	it, _ := l.innerItem()
	sibs := e.itemSiblings(it)
	k := -1
	for i, s := range sibs {
		if s == it.id {
			k = i
		}
	}
	if k <= 0 {
		return
	}
	prev := e.doc.Block(sibs[k-1])
	// The item before's content column, past its marker.
	w := markerWidth(e.src, prev.Start)
	pad := strings.Repeat(" ", w)
	start, end := e.itemRange(it)
	e.indentLines(start, end, pad, len(blankMarkers(e.src[lineStartAt(e.src, it.start):it.start])))
}

// outdentItem moves a nested item up a level, after its parent.
func (e *Editor) outdentItem(l *leaf) {
	var items []cont
	for _, c := range l.conts {
		if c.kind == cItem {
			items = append(items, c)
		}
	}
	if len(items) < 2 {
		e.leaveItem(l)
		return
	}
	it, parent := items[len(items)-1], items[len(items)-2]
	w := markerWidth(e.src, parent.start)
	start, end := e.itemRange(it)
	e.outdentLines(start, end, w, len(blankMarkers(e.src[lineStartAt(e.src, it.start):it.start]))-w)
}

// markerWidth returns how far an item's content is from its marker.
func markerWidth(src string, at int) int {
	e := lineEndAt(src, at)
	p := at
	for p < e && src[p] != ' ' && src[p] != '\t' {
		p++
	}
	sp := 0
	for p+sp < e && src[p+sp] == ' ' && sp < 4 {
		sp++
	}
	if sp == 0 || sp == 4 || p+sp == e {
		sp = 1
	}
	return p - at + sp
}

// indentLines puts pad at column col of every line from start to end.
func (e *Editor) indentLines(start, end int, pad string, col int) {
	e.begin(stepEdit)
	anchor, caret := e.anchor, e.caret
	var starts []int
	for off := start; off <= end && off <= len(e.src); {
		starts = append(starts, off)
		n := lineEndAt(e.src, off)
		if n >= end || n >= len(e.src) {
			break
		}
		off = n + 1
	}
	for i := len(starts) - 1; i >= 0; i-- {
		s := starts[i]
		le := lineEndAt(e.src, s)
		at := min(s+col, le)
		if strings.TrimSpace(e.src[s:le]) == "" {
			continue
		}
		e.replace(at, at, pad)
		if anchor >= at {
			anchor += len(pad)
		}
		if caret >= at {
			caret += len(pad)
		}
	}
	e.commit(anchor, caret)
}

// outdentLines takes up to n spaces from column col of every line from
// start to end.
func (e *Editor) outdentLines(start, end, n, col int) {
	e.begin(stepEdit)
	anchor, caret := e.anchor, e.caret
	var starts []int
	for off := start; off <= end && off <= len(e.src); {
		starts = append(starts, off)
		le := lineEndAt(e.src, off)
		if le >= end || le >= len(e.src) {
			break
		}
		off = le + 1
	}
	for i := len(starts) - 1; i >= 0; i-- {
		s := starts[i]
		le := lineEndAt(e.src, s)
		at := min(s+max(col, 0), le)
		k := 0
		for k < n && at+k < le && e.src[at+k] == ' ' {
			k++
		}
		if k == 0 {
			continue
		}
		e.replace(at, at+k, "")
		if anchor > at {
			anchor -= min(k, anchor-at)
		}
		if caret > at {
			caret -= min(k, caret-at)
		}
	}
	e.commit(anchor, caret)
}
