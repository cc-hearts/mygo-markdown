package syntax

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/CarlOpenLab/livemark/theme"
)

// sample is a document covering all elements of the vocabulary.
var sample = strings.Join([]string{
	"---",
	"title: x",
	"---",
	"",
	"# Head",
	"",
	"> **bold** and *em* and ~~strike~~ and `code` and [link](u \"t\") ![alt](i.png) $x$ <u>U</u> <span>s</span> <br> \\* &amp; [^n]",
	"",
	"- [x] done",
	"- item",
	"",
	"1. one",
	"2. two",
	"",
	"~~~go",
	"fmt",
	"~~~",
	"",
	"$$",
	"x^2",
	"$$",
	"",
	"| a | b |",
	"|:--|--:|",
	"| 1 | 2 |",
	"",
	"---",
	"",
	"[^n]: note",
	"",
	"[x]: http://a",
	"",
}, "\n")

// dump returns the tree, one node a line, indented by depth.
func dump(t *Tree) string {
	var sb strings.Builder
	for n := range t.Walk() {
		depth := 0
		for p, ok := n.Parent(); ok; p, ok = p.Parent() {
			depth++
		}
		sb.WriteString(strings.Repeat("  ", depth))
		sb.WriteString(n.Element().String())
		fmt.Fprintf(&sb, " [%d,%d)", n.Range().Start, n.Range().End)
		for _, r := range n.Syntax() {
			fmt.Fprintf(&sb, " syntax[%d,%d)", r.Start, r.End)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

const sampleDump = `document [0,274)
  front-matter:first-child [0,16) syntax[0,3) syntax[13,16)
  heading.h1[level="1"] [18,24) syntax[18,19)
  quote [26,150) syntax[26,27)
    paragraph:first-child:last-child [28,150)
      syntax.strong [28,30)
      strong:first-child [28,36) syntax[28,30) syntax[34,36)
      syntax.strong [34,36)
      syntax.emphasis [41,42)
      emphasis [41,45) syntax[41,42) syntax[44,45)
      syntax.emphasis [44,45)
      syntax.strike [50,52)
      strike [50,60) syntax[50,52) syntax[58,60)
      syntax.strike [58,60)
      syntax.code [65,66)
      code.inline [65,71) syntax[65,66) syntax[70,71)
      syntax.code [70,71)
      syntax.link [76,77)
      link[href="u"][title="t"] [76,89) syntax[76,77) syntax[81,89)
      syntax.link [81,89)
      syntax.image [90,92)
      image[src="i.png"][alt="alt"] [90,103) syntax[90,92) syntax[95,103)
      syntax.image [95,103)
      syntax.math [104,105)
      math.inline [104,107) syntax[104,105) syntax[106,107)
      syntax.math [106,107)
      syntax.html [108,111)
      underline [108,116) syntax[108,111) syntax[112,116)
      syntax.html [112,116)
      html.inline[tag="span"] [117,123) syntax[117,123)
      syntax.html [117,123)
      html.inline[tag="span"] [124,131) syntax[124,131)
      syntax.html [124,131)
      html.inline[tag="br"] [132,136) syntax[132,136)
      syntax.html [132,136)
      syntax.escape [137,138)
      syntax.entity [140,145)
      footnote-ref[label="n"]:last-child [146,150) syntax[146,150)
  list.bullet.tight[depth="0"] [152,169)
    item.task.checked[depth="0"]:first-child [152,162) syntax[152,158)
      paragraph:first-child:last-child [158,162)
    item[depth="0"]:last-child [163,169) syntax[163,165)
      paragraph:first-child:last-child [165,169)
  list.ordered.tight[depth="0"][start="1"] [171,184)
    item[number="1"][depth="0"]:first-child [171,177) syntax[171,174)
      paragraph:first-child:last-child [174,177)
    item[number="2"][depth="0"]:last-child [178,184) syntax[178,181)
      paragraph:first-child:last-child [181,184)
  code.block.fenced[lang="go"][info="go"] [186,199) syntax[186,189) syntax[189,191) syntax[196,199)
    syntax.fence [186,189)
    syntax.info [189,191)
  math.block [201,210) syntax[201,203) syntax[208,210)
  table [212,241) syntax[222,231)
    row.header:first-child [212,219)
      cell.header[align="left"]:first-child [214,215)
      cell.header[align="right"]:last-child [218,219)
    row.odd:last-child [232,239)
      cell[align="left"]:first-child [234,235)
      cell[align="right"]:last-child [238,239)
  rule [243,246) syntax[243,246)
    syntax.rule [243,246)
  footnote[label="n"] [248,258) syntax[248,254)
    paragraph:first-child:last-child [254,258)
  link-def:last-child [260,273)
`

func TestTree(t *testing.T) {
	if got := dump(Parse(sample)); got != sampleDump {
		t.Errorf("tree of the sample:\n%s\nwant:\n%s", got, sampleDump)
	}
}

// path returns the elements of the tree from the root to the node at off.
func path(t *Tree, off int) string {
	var parts []string
	for _, n := range t.PathAt(off) {
		parts = append(parts, n.Element().String())
	}
	return strings.Join(parts, " › ")
}

func TestPathAt(t *testing.T) {
	tr := Parse(sample)
	const q = "document › quote › paragraph:first-child:last-child › "
	tests := []struct {
		off  int
		want string
	}{
		{0, "document › front-matter:first-child"},
		{5, "document › front-matter:first-child"},
		{19, `document › heading.h1[level="1"]`},
		{27, "document › quote"},
		{29, q + "syntax.strong"},
		{31, q + "strong:first-child:revealed"},
		{34, q + "syntax.strong"},
		{42, q + "emphasis:revealed"},
		{66, q + "code.inline:revealed"},
		{77, q + `link[href="u"][title="t"]:revealed`},
		{91, q + "syntax.image"},
		{96, q + "syntax.image"},
		{105, q + "math.inline:revealed"},
		{109, q + "syntax.html"},
		{111, q + "underline:revealed"},
		{113, q + "syntax.html"},
		{118, q + "syntax.html"},
		{137, q + "syntax.escape"},
		{141, q + "syntax.entity"},
		{147, q + `footnote-ref[label="n"]:last-child:revealed`},
		{153, `document › list.bullet.tight[depth="0"] › item.task.checked[depth="0"]:first-child`},
		{159, `document › list.bullet.tight[depth="0"] › item.task.checked[depth="0"]:first-child › paragraph:first-child:last-child`},
		{187, `document › code.block.fenced[lang="go"][info="go"]:revealed › syntax.fence`},
		{190, `document › code.block.fenced[lang="go"][info="go"]:revealed › syntax.info`},
		{202, "document › math.block:revealed"},
		{204, "document › math.block:revealed"},
		{213, "document › table › row.header:first-child"},
		{214, `document › table › row.header:first-child › cell.header[align="left"]:first-child`},
		{244, "document › rule:revealed › syntax.rule"},
		{249, `document › footnote[label="n"]`},
		{255, `document › footnote[label="n"] › paragraph:first-child:last-child`},
		{261, "document › link-def:last-child"},
		{274, "document"},
	}
	for _, tt := range tests {
		if got := path(tr, tt.off); got != tt.want {
			t.Errorf("path at %d:\n got %s\nwant %s", tt.off, got, tt.want)
		}
	}
}

// first returns the first node of kind k, or an invalid node.
func first(t *Tree, k Kind) Node {
	for n := range t.Walk() {
		if n.Kind() == k {
			return n
		}
	}
	return Node{}
}

// all returns the nodes of kind k.
func all(t *Tree, k Kind) []Node {
	var out []Node
	for n := range t.Walk() {
		if n.Kind() == k {
			out = append(out, n)
		}
	}
	return out
}

func hasClass(n Node, c string) bool { return slices.Contains(n.Element().Classes, c) }

func attr(n Node, name string) string {
	v, _ := n.Element().Attr(name)
	return v
}

func TestAccessors(t *testing.T) {
	t.Run("heading", func(t *testing.T) {
		h := first(Parse("# H\n"), Heading)
		if h.Level() != 1 || h.Setext() || !hasClass(h, "h1") {
			t.Errorf("heading: %v %v %v", h.Level(), h.Setext(), h.Element().Classes)
		}
		if want := []Range{{0, 1}}; !slices.Equal(h.Syntax(), want) {
			t.Errorf("syntax: %v, want %v", h.Syntax(), want)
		}
		if h.Range() != (Range{0, 3}) {
			t.Errorf("range: %v", h.Range())
		}
	})
	t.Run("setext", func(t *testing.T) {
		h := first(Parse("T\n===\n"), Heading)
		if !h.Setext() || !hasClass(h, "setext") {
			t.Errorf("setext: %v %v", h.Setext(), h.Element().Classes)
		}
		if want := []Range{{2, 5}}; !slices.Equal(h.Syntax(), want) {
			t.Errorf("syntax: %v, want %v", h.Syntax(), want)
		}
		if h.Range() != (Range{0, 5}) {
			t.Errorf("range: %v", h.Range())
		}
	})
	t.Run("code block", func(t *testing.T) {
		c := first(Parse("```go title\nx\n```\n"), Code)
		if !c.Fenced() || c.Lang() != "go" || c.Info() != "go title" {
			t.Errorf("code: fenced=%v lang=%q info=%q", c.Fenced(), c.Lang(), c.Info())
		}
		if want := []Range{{0, 3}, {3, 11}, {14, 17}}; !slices.Equal(c.Syntax(), want) {
			t.Errorf("syntax: %v, want %v", c.Syntax(), want)
		}
		if attr(c, "lang") != "go" || attr(c, "info") != "go title" {
			t.Errorf("attrs: %v", c.Element().Attrs)
		}
	})
	t.Run("bullet list", func(t *testing.T) {
		tr := Parse("- [x] a\n- b\n")
		l := first(tr, List)
		if l.Ordered() || !hasClass(l, "bullet") || !hasClass(l, "tight") || l.Depth() != 0 {
			t.Errorf("list: %v %v", l.Ordered(), l.Element().Classes)
		}
		items := all(tr, Item)
		if len(items) != 2 || items[0].Task() != Checked || items[1].Task() != NoTask {
			t.Fatalf("items: %v", items)
		}
		if !hasClass(items[0], "checked") || !hasClass(items[0], "task") || attr(items[0], "number") != "" {
			t.Errorf("item 0: %v", items[0].Element())
		}
	})
	t.Run("ordered list", func(t *testing.T) {
		tr := Parse("3. a\n4. b\n")
		l := first(tr, List)
		if !l.Ordered() || l.Start() != 3 || !hasClass(l, "ordered") || attr(l, "start") != "3" {
			t.Errorf("list: %v %v %v", l.Ordered(), l.Start(), l.Element())
		}
		items := all(tr, Item)
		if len(items) != 2 || items[0].Number() != 3 || items[1].Number() != 4 {
			t.Errorf("items: %v %v", items[0].Number(), items[1].Number())
		}
	})
	t.Run("table", func(t *testing.T) {
		tr := Parse("| a | b |\n|:-:|--:|\n| 1 | 2 |\n")
		cells := all(tr, Cell)
		if len(cells) != 4 {
			t.Fatalf("cells: %d", len(cells))
		}
		if cells[0].Align() != "center" || cells[1].Align() != "right" || !hasClass(cells[0], "header") {
			t.Errorf("header cells: %v %v %v", cells[0].Align(), cells[1].Align(), cells[0].Element().Classes)
		}
		if hasClass(cells[2], "header") || cells[2].Align() != "center" {
			t.Errorf("data cell: %v", cells[2].Element())
		}
		rows := all(tr, Row)
		if !hasClass(rows[0], "header") || !hasClass(rows[1], "odd") {
			t.Errorf("rows: %v %v", rows[0].Element().Classes, rows[1].Element().Classes)
		}
	})
	t.Run("link", func(t *testing.T) {
		tr := Parse("[t](u \"T\")\n")
		l := first(tr, Link)
		if l.Dest() != "u" || l.Title() != "T" || l.Auto() {
			t.Errorf("link: %q %q %v", l.Dest(), l.Title(), l.Auto())
		}
		if attr(l, "href") != "u" || attr(l, "title") != "T" {
			t.Errorf("attrs: %v", l.Element().Attrs)
		}
	})
	t.Run("auto link", func(t *testing.T) {
		l := first(Parse("www.example.com\n"), Link)
		if !l.Auto() || !hasClass(l, "auto") || len(l.Syntax()) != 0 {
			t.Errorf("link: %v %v %v", l.Auto(), l.Element().Classes, l.Syntax())
		}
	})
	t.Run("image", func(t *testing.T) {
		img := first(Parse("![a *b*](i \"T\")\n"), Image)
		if img.Dest() != "i" || img.Title() != "T" || attr(img, "alt") != "a b" || !hasClass(img, "alone") {
			t.Errorf("image: %q %q %q %v", img.Dest(), img.Title(), attr(img, "alt"), img.Element().Classes)
		}
	})
	t.Run("math", func(t *testing.T) {
		m := first(Parse("$x$\n"), Math)
		if m.Literal() != "x" || !hasClass(m, "inline") {
			t.Errorf("math: %q %v", m.Literal(), m.Element().Classes)
		}
		m = first(Parse("a $$x$$ b\n"), Math)
		if !hasClass(m, "display") || m.Literal() != "x" {
			t.Errorf("display math: %v %q", m.Element().Classes, m.Literal())
		}
	})
	t.Run("footnotes", func(t *testing.T) {
		tr := Parse("[^n]\n\n[^n]: note\n")
		r := first(tr, FootnoteRef)
		if r.Label() != "n" || attr(r, "label") != "n" {
			t.Errorf("ref: %q %v", r.Label(), r.Element().Attrs)
		}
		f := first(tr, Footnote)
		if f.Label() != "n" || attr(f, "label") != "n" {
			t.Errorf("footnote: %q %v", f.Label(), f.Element().Attrs)
		}
	})
	t.Run("html tags", func(t *testing.T) {
		tr := Parse("<kbd>K</kbd> <sup>2</sup>\n")
		if n := first(tr, Kbd); !n.Valid() {
			t.Error("no kbd")
		}
		if n := first(tr, Sup); !n.Valid() {
			t.Error("no sup")
		}
	})
	t.Run("cjk emphasis", func(t *testing.T) {
		s := first(Parse("**注意：**内容\n"), Strong)
		if !s.Valid() {
			t.Fatal("no strong beside CJK punctuation")
		}
		if r := s.Range(); r != (Range{0, 13}) {
			t.Errorf("range: %v", r)
		}
	})
	t.Run("front matter", func(t *testing.T) {
		tr := Parse("---\na: 1\n---\n")
		fm := first(tr, FrontMatter)
		if fm.Range() != (Range{0, 12}) || len(fm.Syntax()) != 2 {
			t.Errorf("front matter: %v %v", fm.Range(), fm.Syntax())
		}
	})
	t.Run("link definition", func(t *testing.T) {
		def := first(Parse("[x]: http://a\n"), LinkDef)
		if def.Range() != (Range{0, 13}) {
			t.Errorf("link-def: %v", def.Range())
		}
	})
	t.Run("escape and entity", func(t *testing.T) {
		tr := Parse("\\* &amp;\n")
		syn := all(tr, Syntax)
		if len(syn) != 2 || !hasClass(syn[0], "escape") || !hasClass(syn[1], "entity") {
			t.Errorf("syntax: %v", syn)
		}
	})
}

func TestInvariants(t *testing.T) {
	srcs := []string{
		sample,
		"",
		"\n\n",
		"a",
		"# h\n\n> q\n\n- i\n",
		"~~~\n\n~~~\n",
		"<b>b</b> <i>i</b>\n",
		"| a |\n|---|\n",
	}
	for _, src := range srcs {
		tr := Parse(src)
		root := tr.Root()
		if root.Kind() != Document || !root.Valid() {
			t.Fatalf("%q: no root", src)
		}
		for n := range tr.Walk() {
			r := n.Range()
			if p, ok := n.Parent(); ok {
				pr := p.Range()
				if r.Start < pr.Start || r.End > pr.End {
					t.Errorf("%q: %v out of parent %v", src, n, p)
				}
			}
			// The first and the last child, syntax excluded, are marked.
			var kids []Node
			for c := range n.Children() {
				if c.Kind() != Syntax {
					kids = append(kids, c)
				}
			}
			for i, c := range kids {
				st := c.Element().States
				first, last := i == 0, i == len(kids)-1
				if (st&theme.FirstChild != 0) != first || (st&theme.LastChild != 0) != last {
					t.Errorf("%q: %v states %v at %d of %d", src, c, st, i, len(kids))
				}
			}
		}
		// Every offset is in the nodes of its path.
		for off := 0; off <= len(src); off++ {
			ps := tr.PathAt(off)
			if len(ps) == 0 || ps[0].Kind() != Document {
				t.Fatalf("%q: path at %d: %v", src, off, ps)
			}
			for _, n := range ps {
				r := n.Range()
				inside := off >= r.Start && off < r.End
				if n.Kind() != Syntax {
					inside = off >= r.Start && off <= r.End
				}
				if !inside {
					t.Errorf("%q: path at %d holds %v", src, off, n)
				}
			}
		}
	}
}

func TestElementCopies(t *testing.T) {
	h := first(Parse("# H\n"), Heading)
	el := h.Element()
	el.Classes[0] = "changed"
	if h.Element().Classes[0] == "changed" {
		t.Error("Element aliases the tree's classes")
	}
}

func TestKindElement(t *testing.T) {
	for k := Document; k <= Token; k++ {
		if k.Element() != k.String() {
			t.Errorf("kind %d: element %q, name %q", k, k.Element(), k.String())
		}
	}
}
