package syntax

import (
	"strings"

	"github.com/egoist/mygo/ui/markdown/parser"
)

// inlines adds the inline content of block i, a paragraph, a heading or a
// table cell, as the children of n.
func (b *builder) inlines(n *node, i int) {
	ins := b.doc.Inlines(i)
	b.walkInlines(n, ins, 0, len(ins))
}

// walkInlines adds the inlines from lo to hi, which are siblings, as the
// children of parent.
func (b *builder) walkInlines(parent *node, ins []parser.Inline, lo, hi int) {
	for i := lo; i < hi && i < len(ins); {
		in := &ins[i]
		next := max(in.Next, i+1)
		switch in.Kind {
		case parser.Text:
			b.textSyntax(parent, in.Start, in.End)
		case parser.Code:
			b.codeSpan(parent, in)
		case parser.Emphasis, parser.Strong, parser.Strikethrough:
			b.wrapped(parent, ins, i)
		case parser.Link:
			b.link(parent, ins, i)
		case parser.Image:
			b.image(parent, ins, i)
		case parser.Math:
			b.mathSpan(parent, in)
		case parser.FootnoteRef:
			n := b.new(parent, FootnoteRef, withAttr(element("footnote-ref"), "label", in.Text), in.Start, in.End)
			n.label = in.Text
			n.revStart, n.revEnd = in.Start, in.End
			n.syntax = append(n.syntax, Range{in.Start, in.End})
		case parser.HTML:
			next = b.htmlInline(parent, ins, i, hi)
		}
		i = max(next, i+1)
	}
}

// textSyntax adds the syntax of a text inline: the backslash of an escape,
// and a character reference.
func (b *builder) textSyntax(parent *node, start, end int) {
	src := b.src
	for p := start; p < end; {
		switch src[p] {
		case '\\':
			if p+1 < end && isASCIIPunct(src[p+1]) {
				b.syntaxSibling(parent, "escape", p, p+1)
				p += 2
				continue
			}
		case '&':
			if n, txt := parser.Entity(src[p:end]); n > 0 && txt != "" {
				b.syntaxSibling(parent, "entity", p, p+n)
				p += n
				continue
			}
		}
		p++
	}
}

// codeSpan adds a code span: its backticks its syntax.
func (b *builder) codeSpan(parent *node, in *parser.Inline) {
	ticks := 0
	for in.Start+ticks < in.End && b.src[in.Start+ticks] == '`' {
		ticks++
	}
	cs, ce := in.Start+ticks, max(in.End-ticks, in.Start+ticks)
	b.syntaxSibling(parent, "code", in.Start, cs)
	n := b.new(parent, Code, elementClass("code", "inline"), in.Start, in.End)
	n.literal = in.Text
	n.revStart, n.revEnd = in.Start, in.End
	n.syntax = append(n.syntax, Range{in.Start, cs})
	if ce < in.End {
		b.syntaxSibling(parent, "code", ce, in.End)
		n.syntax = append(n.syntax, Range{ce, in.End})
	}
}

// wrapped adds emphasis, strong emphasis or strikethrough: its markers
// beside it, and its content as its children.
func (b *builder) wrapped(parent *node, ins []parser.Inline, i int) {
	in := &ins[i]
	cs, ce := inlineChildren(ins, i, in.Start)
	kind := Strong
	switch in.Kind {
	case parser.Emphasis:
		kind = Emphasis
	case parser.Strikethrough:
		kind = Strike
	}
	b.syntaxSibling(parent, kind.Element(), in.Start, cs)
	n := b.new(parent, kind, element(kind.Element()), in.Start, in.End)
	n.revStart, n.revEnd = in.Start, in.End
	if cs == in.Start {
		// Nothing inside: show it as written.
		n.syntax = append(n.syntax, Range{in.Start, in.End})
		return
	}
	n.syntax = append(n.syntax, Range{in.Start, cs})
	b.walkInlines(n, ins, i+1, min(in.Next, len(ins)))
	if ce < in.End {
		b.syntaxSibling(parent, kind.Element(), ce, in.End)
		n.syntax = append(n.syntax, Range{ce, in.End})
	}
}

// link adds a link: its text as its children, its brackets and
// destination its syntax, unless it is written as its address.
func (b *builder) link(parent *node, ins []parser.Inline, i int) {
	in := &ins[i]
	cs, ce := inlineChildren(ins, i, in.Start)
	el := element("link")
	if in.Auto {
		el.Classes = append(el.Classes, "auto")
	}
	el = withAttr(el, "href", in.Dest)
	if in.Title != "" {
		el = withAttr(el, "title", in.Title)
	}
	if noChildren(ins, i) {
		// A link without text shows its source.
		b.syntaxSibling(parent, "link", in.Start, in.End)
		n := b.new(parent, Link, el, in.Start, in.End)
		n.dest, n.title, n.auto = in.Dest, in.Title, in.Auto
		n.revStart, n.revEnd = in.Start, in.End
		n.syntax = append(n.syntax, Range{in.Start, in.End})
		return
	}
	if in.Auto && cs == in.Start && ce == in.End {
		// An address written as itself has no syntax to hide.
		n := b.new(parent, Link, el, in.Start, in.End)
		n.dest, n.title, n.auto = in.Dest, in.Title, in.Auto
		n.revStart, n.revEnd = in.Start, in.End
		b.walkInlines(n, ins, i+1, min(in.Next, len(ins)))
		return
	}
	b.syntaxSibling(parent, "link", in.Start, cs)
	n := b.new(parent, Link, el, in.Start, in.End)
	n.dest, n.title, n.auto = in.Dest, in.Title, in.Auto
	n.revStart, n.revEnd = in.Start, in.End
	n.syntax = append(n.syntax, Range{in.Start, cs})
	b.walkInlines(n, ins, i+1, min(in.Next, len(ins)))
	if ce < in.End {
		b.syntaxSibling(parent, "link", ce, in.End)
		n.syntax = append(n.syntax, Range{ce, in.End})
	}
}

// image adds an image: its description as its children, its source as a
// whole the caret shows.
func (b *builder) image(parent *node, ins []parser.Inline, i int) {
	in := &ins[i]
	cs, ce := inlineChildren(ins, i, in.Start+2)
	el := element("image")
	if parent.kind == Paragraph && aloneImage(ins, b.src) {
		el.Classes = append(el.Classes, "alone")
	}
	el = withAttr(el, "src", in.Dest)
	alt := b.plainText(ins, i+1, min(in.Next, len(ins)))
	if alt != "" {
		el = withAttr(el, "alt", alt)
	}
	if in.Title != "" {
		el = withAttr(el, "title", in.Title)
	}
	b.syntaxSibling(parent, "image", in.Start, cs)
	n := b.new(parent, Image, el, in.Start, in.End)
	n.dest, n.title = in.Dest, in.Title
	n.revStart, n.revEnd = in.Start, in.End
	n.syntax = append(n.syntax, Range{in.Start, cs})
	b.walkInlines(n, ins, i+1, min(in.Next, len(ins)))
	if ce < in.End {
		b.syntaxSibling(parent, "image", ce, in.End)
		n.syntax = append(n.syntax, Range{ce, in.End})
	}
}

// mathSpan adds inline math: its delimiters its syntax and its TeX its
// literal.
func (b *builder) mathSpan(parent *node, in *parser.Inline) {
	open := 1
	for _, d := range []string{"$$", `\(`, `\[`} {
		if strings.HasPrefix(b.src[in.Start:in.End], d) {
			open = len(d)
		}
	}
	cs, ce := min(in.Start+open, in.End), max(in.End-open, in.Start+open)
	b.syntaxSibling(parent, "math", in.Start, cs)
	n := b.new(parent, Math, elementClass("math", mathClass(in)), in.Start, in.End)
	n.literal = in.Text
	n.revStart, n.revEnd = in.Start, in.End
	n.syntax = append(n.syntax, Range{in.Start, cs})
	if ce < in.End {
		b.syntaxSibling(parent, "math", ce, in.End)
		n.syntax = append(n.syntax, Range{ce, in.End})
	}
}

// mathClass returns a formula's class: .display for one shown on its own
// lines, .inline otherwise.
func mathClass(in *parser.Inline) string {
	if in.Display {
		return "display"
	}
	return "inline"
}

// htmlInline adds raw HTML within text. A formatting tag pairs with its
// closing tag into the element it formats; another shows as itself.
// htmlInline returns the index after the last sibling it read.
func (b *builder) htmlInline(parent *node, ins []parser.Inline, i, hi int) int {
	in := &ins[i]
	name, closing, formatting := htmlTagName(in.Text)
	if !formatting || name == "br" {
		el := elementClass("html", "inline")
		if raw := rawTagName(in.Text); raw != "" {
			el = withAttr(el, "tag", raw)
		}
		n := b.new(parent, HTML, el, in.Start, in.End)
		n.tag = rawTagName(in.Text)
		n.revStart, n.revEnd = in.Start, in.End
		n.syntax = append(n.syntax, Range{in.Start, in.End})
		b.syntaxSibling(parent, "html", in.Start, in.End)
		return i + 1
	}
	if closing {
		// A closing tag without its opening tag shows as itself.
		n := b.new(parent, HTML, withAttr(elementClass("html", "inline"), "tag", name), in.Start, in.End)
		n.tag = name
		return i + 1
	}
	j := b.matchingTag(ins, i, hi, name)
	if j < 0 {
		// Unclosed: the tag styles the rest of the block, as the editor
		// reads it.
		kind, class := tagKind(name)
		el := element(kind.Element())
		if class != "" {
			el.Classes = append(el.Classes, class)
		}
		b.syntaxSibling(parent, "html", in.Start, in.End)
		n := b.new(parent, kind, el, in.Start, parent.end)
		n.tag = name
		n.revStart, n.revEnd = in.Start, parent.end
		n.syntax = append(n.syntax, Range{in.Start, in.End})
		b.walkInlines(n, ins, i+1, hi)
		return hi
	}
	close := &ins[j]
	kind, class := tagKind(name)
	el := element(kind.Element())
	if class != "" {
		el.Classes = append(el.Classes, class)
	}
	b.syntaxSibling(parent, "html", in.Start, in.End)
	n := b.new(parent, kind, el, in.Start, close.End)
	n.tag = name
	n.revStart, n.revEnd = in.Start, close.End
	n.syntax = append(n.syntax, Range{in.Start, in.End})
	if close.Start > in.End {
		b.walkInlines(n, ins, i+1, j)
	}
	b.syntaxSibling(parent, "html", close.Start, close.End)
	n.syntax = append(n.syntax, Range{close.Start, close.End})
	return j + 1
}

// matchingTag returns the index of the closing tag matching the opening
// tag at i among its siblings, or -1.
func (b *builder) matchingTag(ins []parser.Inline, i, hi int, name string) int {
	depth := 0
	for j := i + 1; j < hi && j < len(ins); j = max(ins[j].Next, j+1) {
		in := &ins[j]
		if in.Kind != parser.HTML {
			continue
		}
		n, closing, ok := htmlTagName(in.Text)
		if !ok || n != name {
			continue
		}
		if closing {
			if depth == 0 {
				return j
			}
			depth--
		} else {
			depth++
		}
	}
	return -1
}

// tagKind returns the kind of element a formatting tag makes, and its
// class: "html" for one written as a tag.
func tagKind(name string) (Kind, string) {
	switch name {
	case "b", "strong":
		return Strong, "html"
	case "i", "em":
		return Emphasis, "html"
	case "s", "del", "strike":
		return Strike, "html"
	case "u", "ins":
		return Underline, ""
	case "mark":
		return Mark, ""
	case "sup":
		return Sup, ""
	case "sub":
		return Sub, ""
	case "kbd":
		return Kbd, ""
	case "code":
		return Code, "html"
	}
	return HTML, ""
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
	return "", closing, false
}

// rawTagName reads the name of any HTML tag, in lower case, or "" for
// HTML that is not a tag.
func rawTagName(tag string) string {
	if !strings.HasPrefix(tag, "<") || !strings.HasSuffix(tag, ">") {
		return ""
	}
	t := strings.TrimSpace(tag[1 : len(tag)-1])
	t = strings.TrimPrefix(t, "/")
	t = strings.TrimSpace(t)
	if t == "" || t[0] == '!' || t[0] == '?' {
		return ""
	}
	if i := strings.IndexAny(t, " \t\r\n/"); i >= 0 {
		t = t[:i]
	}
	return strings.ToLower(t)
}

// noChildren reports whether inline i holds no inlines.
func noChildren(ins []parser.Inline, i int) bool {
	return i+1 >= ins[i].Next || i+1 >= len(ins)
}

// inlineChildren returns where the children of inline i start and end in
// the source, or from and from when it has none.
func inlineChildren(ins []parser.Inline, i, from int) (int, int) {
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

// aloneImage reports whether a block's inlines are one image and nothing
// else but spaces.
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

// plainText returns the text of the inlines from lo to hi, as parser
// .PlainText does, with the characters CJKFriendly swapped taken back.
func (b *builder) plainText(ins []parser.Inline, lo, hi int) string {
	parsed := b.doc.Source()
	var sb strings.Builder
	for k := lo; k < hi && k < len(ins); k++ {
		in := &ins[k]
		switch in.Kind {
		case parser.Text:
			if in.End <= len(parsed) && in.End <= len(b.src) && in.Text == parsed[in.Start:in.End] {
				sb.WriteString(b.src[in.Start:in.End])
			} else {
				sb.WriteString(in.Text)
			}
		case parser.Code, parser.HTML, parser.Math:
			sb.WriteString(in.Text)
		case parser.SoftBreak, parser.LineBreak:
			sb.WriteByte(' ')
		case parser.FootnoteRef:
			sb.WriteString("[^" + in.Text + "]")
		}
	}
	return sb.String()
}

// isASCIIPunct reports whether c is ASCII punctuation.
func isASCIIPunct(c byte) bool {
	return c >= '!' && c <= '/' || c >= ':' && c <= '@' || c >= '[' && c <= '`' || c >= '{' && c <= '~'
}
