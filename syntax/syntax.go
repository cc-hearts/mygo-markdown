package syntax

import (
	"iter"
	"slices"

	"github.com/CarlOpenLab/livemark/internal/parse"
	"github.com/CarlOpenLab/livemark/theme"
)

// Kind is what a node of a syntax tree is. Its name is the type the
// node's element has for a theme: "heading", "strong", "syntax"…
type Kind uint8

// Kinds of nodes.
const (
	Document Kind = iota
	Paragraph
	Heading
	Quote
	List
	Item
	// Code is a code span, its element .inline, or a block of code, its
	// element .block.
	Code
	// Math is inline math, its element .inline or .display, or a block of
	// math, its element .block.
	Math
	// HTML is raw HTML, .inline or .block.
	HTML
	Table
	Row
	Cell
	Rule
	Footnote
	FrontMatter
	LinkDef
	Strong
	Emphasis
	Strike
	Underline
	Mark
	Sup
	Sub
	Kbd
	Link
	Image
	FootnoteRef
	// Syntax is syntax that shows at the caret: the markers of a
	// construct, and the fences of a block of code.
	Syntax
	// Token is a token of highlighted code. The editor makes them as it
	// highlights; a parsed tree holds none.
	Token
)

var kindNames = [...]string{
	"document", "paragraph", "heading", "quote", "list", "item", "code",
	"math", "html", "table", "row", "cell", "rule", "footnote",
	"front-matter", "link-def", "strong", "emphasis", "strike", "underline",
	"mark", "sup", "sub", "kbd", "link", "image", "footnote-ref", "syntax",
	"token",
}

func (k Kind) String() string {
	if int(k) < len(kindNames) {
		return kindNames[k]
	}
	return "unknown"
}

// Element returns the type of the elements of kind k, as a theme selector
// names it.
func (k Kind) Element() string { return k.String() }

// Range is a run of bytes of a source, from Start to End, End excluded.
type Range struct{ Start, End int }

// Len returns how many bytes the range holds.
func (r Range) Len() int { return r.End - r.Start }

// Empty reports whether the range holds no bytes.
func (r Range) Empty() bool { return r.End <= r.Start }

// Task is the check box of a task list item.
type Task uint8

const (
	NoTask Task = iota
	Unchecked
	Checked
)

// Tree is a Markdown document parsed as the editor reads it: front matter
// blanked, CJK emphasis corrected, GitHub's flavor with footnotes and
// math. It is a snapshot, read-only.
type Tree struct {
	src  string
	root *node
}

// Parse reads src as the editor reads it.
func Parse(src string) *Tree {
	d, fm := parse.Parse(src)
	b := &builder{src: src, doc: d}
	t := &Tree{src: src}
	t.root = b.build(fm)
	return t
}

// Source returns the source the tree was parsed from.
func (t *Tree) Source() string {
	if t == nil {
		return ""
	}
	return t.src
}

// Root returns the document node, the root of the tree.
func (t *Tree) Root() Node {
	if t == nil || t.root == nil {
		return Node{}
	}
	return Node{n: t.root, caret: -1}
}

// Walk returns the nodes of the tree in document order, depth first,
// starting at the root.
func (t *Tree) Walk() iter.Seq[Node] {
	root := t.Root()
	return func(yield func(Node) bool) {
		var walk func(Node) bool
		walk = func(n Node) bool {
			if !n.Valid() {
				return true
			}
			if !yield(n) {
				return false
			}
			for c := range n.Children() {
				if !walk(c) {
					return false
				}
			}
			return true
		}
		walk(root)
	}
}

// PathAt returns the nodes from the root to the innermost node at byte
// offset off, the root alone when no node holds it. The elements of the
// nodes show their states at off: a construct the offset is in has
// :revealed.
func (t *Tree) PathAt(off int) []Node {
	if t == nil || t.root == nil {
		return nil
	}
	off = min(max(off, 0), len(t.src))
	var path []Node
	n := t.root
	for n != nil {
		path = append(path, Node{n: n, caret: off})
		n = childAt(n, off)
	}
	return path
}

// Node is a node of a tree, a value. The zero Node is invalid: its
// methods return zero values.
type Node struct {
	n *node
	// caret is the offset whose states the element shows, -1 for none.
	caret int
}

// Valid reports whether the node is one of a tree.
func (n Node) Valid() bool { return n.n != nil }

// Kind returns what the node is.
func (n Node) Kind() Kind {
	if n.n == nil {
		return Document
	}
	return n.n.kind
}

// Element returns the element selectors match for the node: its type, its
// classes and attributes, and the states it is in at the offset the node
// was got at, as :revealed.
func (n Node) Element() theme.Element {
	if n.n == nil {
		return theme.Element{}
	}
	el := n.n.el
	el.Classes = slices.Clone(el.Classes)
	el.Attrs = slices.Clone(el.Attrs)
	if n.caret >= 0 && n.n.revStart >= 0 && n.caret >= n.n.revStart && n.caret <= n.n.revEnd {
		el.States |= theme.Revealed
	}
	return el
}

// Range returns the node's bytes in the source.
func (n Node) Range() Range {
	if n.n == nil {
		return Range{}
	}
	return Range{n.n.start, n.n.end}
}

// Syntax returns the ranges of the node's syntax in the source: the
// markers of a construct, a heading's #s, a quote's >s, a fence and its
// info, a rule's source.
func (n Node) Syntax() []Range {
	if n.n == nil {
		return nil
	}
	return slices.Clone(n.n.syntax)
}

// Parent returns the node holding the node, and whether it has one.
func (n Node) Parent() (Node, bool) {
	if n.n == nil || n.n.parent == nil {
		return Node{}, false
	}
	return Node{n: n.n.parent, caret: n.caret}, true
}

// Children returns the nodes the node holds, in source order.
func (n Node) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if n.n == nil {
			return
		}
		for _, k := range n.n.kids {
			if !yield(Node{n: k, caret: n.caret}) {
				return
			}
		}
	}
}

// Level returns a heading's level, from 1 to 6.
func (n Node) Level() int {
	if n.n == nil {
		return 0
	}
	return n.n.level
}

// Lang returns a code block's language: the first word of its info
// string.
func (n Node) Lang() string {
	if n.n == nil {
		return ""
	}
	return n.n.lang
}

// Info returns a code block's info string.
func (n Node) Info() string {
	if n.n == nil {
		return ""
	}
	return n.n.info
}

// Dest returns a link's or an image's destination.
func (n Node) Dest() string {
	if n.n == nil {
		return ""
	}
	return n.n.dest
}

// Title returns a link's or an image's title.
func (n Node) Title() string {
	if n.n == nil {
		return ""
	}
	return n.n.title
}

// Label returns a footnote's or a footnote reference's label.
func (n Node) Label() string {
	if n.n == nil {
		return ""
	}
	return n.n.label
}

// Literal returns the text of a code span, a formula's TeX, or the source
// of an HTML tag.
func (n Node) Literal() string {
	if n.n == nil {
		return ""
	}
	return n.n.literal
}

// Tag returns the name of an HTML inline's tag, in lower case, or "" for
// HTML that is not a tag.
func (n Node) Tag() string {
	if n.n == nil {
		return ""
	}
	return n.n.tag
}

// Task returns a list item's check box.
func (n Node) Task() Task {
	if n.n == nil {
		return NoTask
	}
	return n.n.task
}

// Ordered reports whether a list is numbered.
func (n Node) Ordered() bool {
	return n.n != nil && n.n.ordered
}

// Start returns the number an ordered list starts at, and 0 for a bullet
// list.
func (n Node) Start() int {
	if n.n == nil {
		return 0
	}
	return n.n.startNum
}

// Number returns the number a list item shows.
func (n Node) Number() int {
	if n.n == nil {
		return 0
	}
	return n.n.number
}

// Depth returns how deep a list or an item is nested in other lists, from
// 0.
func (n Node) Depth() int {
	if n.n == nil {
		return 0
	}
	return n.n.depth
}

// Align returns a table cell's alignment: "left", "center", "right", or ""
// when the table does not align it.
func (n Node) Align() string {
	if n.n == nil {
		return ""
	}
	return n.n.align
}

// Auto reports whether a link is written as its address.
func (n Node) Auto() bool {
	return n.n != nil && n.n.auto
}

// Fenced reports whether a block of code is fenced rather than indented.
func (n Node) Fenced() bool {
	return n.n != nil && n.n.fenced
}

// Setext reports whether a heading is underlined with = or - rather than
// marked with #.
func (n Node) Setext() bool {
	return n.n != nil && n.n.setext
}

// String returns the node for a debugger: its element and its range.
func (n Node) String() string {
	if n.n == nil {
		return "<invalid node>"
	}
	return n.Element().String() + " " + rangeString(n.Range())
}
