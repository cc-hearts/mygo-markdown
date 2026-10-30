package livemark

import (
	"fmt"
	"slices"
	"testing"

	"github.com/CarlOpenLab/livemark/syntax"
	"github.com/CarlOpenLab/livemark/theme"
)

// The consistency test compares what the editor's leaves say about a
// document with the public syntax tree of the same source: at every
// offset, the innermost block the editor lays out is the innermost block
// the syntax tree holds, with the same type, classes and attributes.

// leafBlock returns the leaf block element of a leaf, and whether it is
// one: containers and empty leaves are not compared.
func leafBlock(l *leaf) (theme.Element, bool) {
	if l.id < 0 && l.kind != kRaw || l.pending {
		return theme.Element{}, false
	}
	switch l.kind {
	case kPara:
		if l.table != nil {
			el := theme.Element{Type: "cell"}
			row, col := l.rowCol()
			if row == 0 {
				el.Classes = append(el.Classes, "header")
			}
			if align := l.table.tab.align; col < len(align) {
				if a := alignName(align[col]); a != "" {
					el.Attrs = append(el.Attrs, theme.Attr{Name: "align", Value: a})
				}
			}
			return el, true
		}
		return theme.Element{Type: "paragraph"}, true
	case kHeading:
		el := theme.Element{Type: "heading"}
		el.Classes = append(el.Classes, fmt.Sprintf("h%d", l.level))
		if l.setext {
			el.Classes = append(el.Classes, "setext")
		}
		el.Attrs = append(el.Attrs, theme.Attr{Name: "level", Value: fmt.Sprint(l.level)})
		return el, true
	case kCode:
		el := theme.Element{Type: "code", Classes: []string{"block"}}
		if l.fenced {
			el.Classes = append(el.Classes, "fenced")
		} else {
			el.Classes = append(el.Classes, "indented")
		}
		if l.lang != "" {
			el.Attrs = append(el.Attrs, theme.Attr{Name: "lang", Value: l.lang})
		}
		return el, true
	case kMath:
		return theme.Element{Type: "math", Classes: []string{"block"}}, true
	case kHTML:
		return theme.Element{Type: "html", Classes: []string{"block"}}, true
	case kRule:
		return theme.Element{Type: "rule"}, true
	case kRaw:
		if l.frontMatter {
			return theme.Element{Type: "front-matter"}, true
		}
		return theme.Element{Type: "link-def"}, true
	}
	return theme.Element{}, false
}

// leafCovers reports whether the editor lays out the offset in the leaf,
// as the context menu of the caret finds it.
func leafCovers(l *leaf, off int) bool {
	if l.kind == kPhantom {
		return false
	}
	start := l.start
	switch l.kind {
	case kCode, kMath, kHTML, kRaw, kHeading:
		start = l.lineStart
	}
	return off >= start && off <= l.end
}

// syntaxLeafBlock returns the innermost block element of the syntax path
// at off that a leaf lays out.
func syntaxLeafBlock(tr *syntax.Tree, off int) (theme.Element, bool) {
	path := tr.PathAt(off)
	for i := len(path) - 1; i >= 0; i-- {
		el := path[i].Element()
		switch path[i].Kind() {
		case syntax.Paragraph, syntax.Heading, syntax.Rule, syntax.Cell,
			syntax.FrontMatter, syntax.LinkDef:
			return el, true
		case syntax.Code, syntax.Math, syntax.HTML:
			if el.HasClass("block") {
				return el, true
			}
		}
	}
	return theme.Element{}, false
}

// sameBlock compares the type, the classes that decide what a block is,
// and its attributes.
func sameBlock(a, b theme.Element) bool {
	if a.Type != b.Type {
		return false
	}
	keep := func(c string) bool {
		switch c {
		case "block", "fenced", "indented", "h1", "h2", "h3", "h4", "h5", "h6",
			"setext", "header", "inline", "display":
			return true
		}
		return false
	}
	var ac, bc []string
	for _, c := range a.Classes {
		if keep(c) {
			ac = append(ac, c)
		}
	}
	for _, c := range b.Classes {
		if keep(c) {
			bc = append(bc, c)
		}
	}
	slices.Sort(ac)
	slices.Sort(bc)
	if !slices.Equal(ac, bc) {
		return false
	}
	attr := func(e theme.Element, name string) string {
		v, _ := e.Attr(name)
		return v
	}
	for _, name := range []string{"level", "lang", "align"} {
		if attr(a, name) != attr(b, name) {
			return false
		}
	}
	return true
}

func TestSyntaxConsistency(t *testing.T) {
	corpus := []string{
		"plain text\n\nsecond paragraph\n",
		"# One\n\n## Two\n\nSetext\n======\n",
		"> quote **bold** and `code`\n>\n> - item\n",
		"- a\n- b\n\n1. one\n2. two\n\n- [x] done\n",
		"~~~go\nfmt.Println(\"x\")\n~~~\n\n    indented\n",
		"$$\nx^2\n$$\n\ntext $y$ more\n",
		"<div>\nhi\n</div>\n\ninline <span>s</span>\n",
		"| a | b |\n|:--|--:|\n| 1 | 2 |\n",
		"---\ntitle: t\n---\n\nbody\n",
		"[x]: http://a\n\n[x]\n",
		"---\n\nrule above\n",
		"[^n]: note\n\nref [^n]\n",
		"&amp; and \\* escaped\n",
		"**注意：**内容\n",
	}
	for _, src := range corpus {
		e := New(src, Options{})
		tr := syntax.Parse(src)
		// The leaves the editor lays out, table cells included.
		var leaves []*leaf
		for _, l := range e.leaves {
			if l.kind == kTable {
				leaves = append(leaves, l.tab.cells()...)
				continue
			}
			leaves = append(leaves, l)
		}
		compared := 0
		for off := 0; off <= len(src); off++ {
			var want theme.Element
			found := false
			for _, l := range leaves {
				if el, ok := leafBlock(l); ok && leafCovers(l, off) {
					want, found = el, true
					break
				}
			}
			if !found {
				continue
			}
			got, ok := syntaxLeafBlock(tr, off)
			if !ok {
				continue
			}
			compared++
			if !sameBlock(want, got) {
				t.Errorf("%q at %d: editor %s, syntax %s", src, off, want.String(), got.String())
			}
		}
		if compared < len(src)/4 {
			t.Errorf("%q: only %d offsets compared of %d", src, compared, len(src)+1)
		}
	}
}
