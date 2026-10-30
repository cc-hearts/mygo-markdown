package livemark

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui/markdown/parser"

	"github.com/CarlOpenLab/livemark/syntax"
	"github.com/CarlOpenLab/livemark/theme"
)

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

// Syntax returns the document parsed as the editor reads it, as a
// snapshot: a later change of the source makes another tree, not a
// different one. The same tree is returned until the source changes.
func (e *Editor) Syntax() *syntax.Tree {
	if e.syn == nil || e.synRev != e.rev {
		e.syn = syntax.Parse(e.src)
		e.synRev = e.rev
	}
	return e.syn
}

// StylePath returns the elements a theme matches at a source offset, as
// an inspector shows them: the document, the containers and the table
// around the block there, the block, and the inline constructs the offset
// is in, with their states.
func (e *Editor) StylePath(off int) []theme.Element {
	e.sync()
	off = max(0, min(off, len(e.src)))
	u := e.unitAt(off)
	if u == nil {
		return []theme.Element{{Type: "document"}}
	}
	var path []theme.Element
	for n := e.styledLeaf(u).node; n != nil; n = n.Parent() {
		path = append(path, n.Element())
	}
	slices.Reverse(path)
	if u.id < 0 {
		return path
	}
	return append(path, e.inlinePath(u, off)...)
}

// inlinePath returns the elements of the inline constructs around off in
// a leaf's text, outermost first: strong, link, code.inline…
func (e *Editor) inlinePath(l *leaf, off int) []theme.Element {
	ins := e.doc.Inlines(l.id)
	rev := -1
	if at := e.revealAt(l); at >= 0 {
		rev = at + l.lineStart
	}
	var out []theme.Element
	var walk func(i, end int)
	walk = func(i, end int) {
		for i < end {
			in := &ins[i]
			next := min(max(in.Next, i+1), len(ins))
			if off < in.Start || off >= in.End {
				i = next
				continue
			}
			el, ok := inlineElement(ins, i, rev >= in.Start && rev < in.End)
			if !ok {
				return
			}
			out = append(out, el)
			if next > i+1 {
				walk(i+1, next)
			}
			return
		}
	}
	walk(0, len(ins))
	return out
}

// inlineElement returns the element of the inline at i, of its children's
// text, and whether it is a construct a theme styles.
func inlineElement(ins []parser.Inline, i int, revealed bool) (theme.Element, bool) {
	in := &ins[i]
	switch in.Kind {
	case parser.Code:
		return theme.Element{Type: "code", Classes: []string{"inline"}}, true
	case parser.Emphasis:
		return theme.Element{Type: "emphasis"}, true
	case parser.Strong:
		return theme.Element{Type: "strong"}, true
	case parser.Strikethrough:
		return theme.Element{Type: "strike"}, true
	case parser.Link:
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
		return el, true
	case parser.Image:
		el := theme.Element{Type: "image"}
		if in.Dest != "" {
			el.Attrs = append(el.Attrs, theme.Attr{Name: "src", Value: in.Dest})
		}
		if in.Title != "" {
			el.Attrs = append(el.Attrs, theme.Attr{Name: "title", Value: in.Title})
		}
		return el, true
	case parser.Math:
		el := theme.Element{Type: "math", Classes: []string{"inline"}}
		if revealed {
			el.States |= theme.Revealed
		}
		return el, true
	case parser.FootnoteRef:
		el := theme.Element{Type: "footnote-ref"}
		if in.Text != "" {
			el.Attrs = append(el.Attrs, theme.Attr{Name: "label", Value: in.Text})
		}
		return el, true
	}
	return theme.Element{}, false
}
