package livemark

import "strings"

// The app edits the document through these, as an AI tool or a command
// does: each is one step of undo, so the user takes it back as their own.

// ReplaceAll makes src the document, as an edit of what changed rather
// than a new document: the caret stays where it was, around the change,
// and undo brings the old text back.
func (e *Editor) ReplaceAll(src string) {
	if src == e.src {
		return
	}
	p := 0
	for p < len(e.src) && p < len(src) && e.src[p] == src[p] {
		p++
	}
	s := 0
	for s < len(e.src)-p && s < len(src)-p && e.src[len(e.src)-1-s] == src[len(src)-1-s] {
		s++
	}
	// Keep the change on whole runes.
	for p > 0 && p < len(e.src) && !runeStart(e.src[p]) {
		p--
	}
	for s > 0 && len(e.src)-s < len(e.src) && !runeStart(e.src[len(e.src)-s]) {
		s--
	}
	e.Replace(p, len(e.src)-s, src[p:len(src)-s])
}

// Replace replaces the source's bytes [from, to) with text, as one step of
// undo.
func (e *Editor) Replace(from, to int, text string) {
	e.dropPhantom()
	from = max(0, min(from, len(e.src)))
	to = max(from, min(to, len(e.src)))
	e.breakCoalescing()
	e.begin(stepEdit)
	e.replace(from, to, text)
	e.commit(e.anchor, e.caret)
	e.breakCoalescing()
	e.invalidate()
}

// AppendBlock adds Markdown as a block of its own at the end of the
// document, and puts the caret at its end.
func (e *Editor) AppendBlock(md string) {
	md = strings.Trim(md, "\n")
	if md == "" {
		return
	}
	body := strings.TrimRight(e.src, "\n")
	sep := "\n\n"
	if body == "" {
		sep = ""
	}
	e.dropPhantom()
	e.breakCoalescing()
	e.begin(stepEdit)
	e.replace(len(body), len(e.src), sep+md+"\n")
	end := len(body) + len(sep) + len(md)
	e.commit(end, end)
	e.breakCoalescing()
	e.invalidate()
}

// PrependBlock adds Markdown as a block of its own at the start of the
// document.
func (e *Editor) PrependBlock(md string) {
	md = strings.Trim(md, "\n")
	if md == "" {
		return
	}
	rest := strings.TrimLeft(e.src, "\n")
	sep := "\n\n"
	if rest == "" {
		sep = "\n"
	}
	e.dropPhantom()
	e.breakCoalescing()
	e.begin(stepEdit)
	e.replace(0, len(e.src)-len(rest), md+sep)
	e.commit(len(md), len(md))
	e.breakCoalescing()
	e.invalidate()
}

// InsertAtCaret types Markdown at the caret, as pasted.
func (e *Editor) InsertAtCaret(md string) { e.insertText(md) }

// Caret returns the caret's offset in the source.
func (e *Editor) Caret() int { return e.caret }

func runeStart(b byte) bool { return b&0xC0 != 0x80 }

// WindowPoint returns where the caret at source offset off shows in the
// window, at the middle of its line, as of the last frame: for an app's
// popover by the caret, or a test's click.
func (e *Editor) WindowPoint(off int) (x, y float32, ok bool) {
	r, ok := e.caretRect(off)
	if !ok {
		return 0, 0, false
	}
	return e.elemRect.X + r.X, e.elemRect.Y + r.Y + r.H/2, true
}
