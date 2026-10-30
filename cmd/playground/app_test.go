package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/livemark/syntax"
	"github.com/CarlOpenLab/livemark/theme"
)

// TestPresetsParseCleanly checks that the playground's own themes use the
// CSS subset as it is documented, with no diagnostics.
func TestPresetsParseCleanly(t *testing.T) {
	for _, p := range presets {
		_, ds := theme.Parse(p.name, p.css)
		for _, d := range ds {
			t.Errorf("%s: %s", p.name, d)
		}
	}
}

// TestSampleCoversVocabulary checks that the sample document shows every
// element a theme can name, so that a selector's effect is seen at once.
func TestSampleCoversVocabulary(t *testing.T) {
	seen := map[string]bool{}
	var walk func(n syntax.Node)
	walk = func(n syntax.Node) {
		el := n.Element()
		seen[el.Type] = true
		for _, c := range el.Classes {
			seen[el.Type+"."+c] = true
		}
		for c := range n.Children() {
			walk(c)
		}
	}
	walk(syntax.Parse(sampleDoc).Root())

	want := []string{
		"document", "front-matter", "heading", "heading.h1", "heading.h2", "heading.h3",
		"heading.h4", "heading.h5", "heading.h6", "heading.setext", "paragraph",
		"quote", "list", "list.bullet", "list.ordered", "list.tight", "list.loose",
		"item", "item.task", "item.checked", "code", "code.block", "code.fenced",
		"code.indented", "code.inline", "math", "math.block", "math.inline",
		"html", "html.block", "html.inline", "table", "row", "row.header",
		"row.even", "row.odd", "cell", "cell.header", "rule", "footnote",
		"footnote-ref", "link-def", "strong", "emphasis", "strike", "underline",
		"mark", "sup", "sub", "kbd", "link", "link.auto", "image", "image.alone",
		"syntax",
	}
	for _, w := range want {
		if !seen[w] {
			t.Errorf("the sample has no %s", w)
		}
	}
}

// TestTypingReparsesAfterPause checks that the CSS editor's text becomes
// the document's theme once typing rests, and not before.
func TestTypingReparsesAfterPause(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	before := a.theme

	tt.ClickAt(200, 300)
	tt.Frame()
	tt.Type(":root { color: #123456; }")
	tt.Advance(100 * time.Millisecond)
	if a.theme != before {
		t.Fatal("the theme was parsed before typing rested")
	}
	tt.Advance(100 * time.Millisecond)
	if a.theme == before {
		t.Fatal("the theme was not parsed after typing rested")
	}
	if got := a.doc.Options().Theme; got != a.theme {
		t.Fatal("the document did not get the theme")
	}
}

// TestDiagnosticsJump checks that a diagnostic of the theme reaches the
// CSS editor, and that a click goes to its line.
func TestDiagnosticsJump(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	a.setCSS("heading {\n  colr: red;\n}\n")
	a.parse()
	tt.Frame()
	if len(a.diags) == 0 {
		t.Fatal("the typo made no diagnostic")
	}
	i := 0
	for j, d := range a.diags {
		if d.Line == 2 && d.Severity == theme.Warning {
			i = j
		}
	}
	if ds := a.css.Diagnostics(); len(ds) != len(a.diags) {
		t.Fatalf("the editor shows %d diagnostics, the theme made %d", len(ds), len(a.diags))
	}
	a.jumpTo(a.diags[i])
	if got := a.css.Buffer.Pos(a.css.Cursor()).Line; got != 1 {
		t.Fatalf("the caret is on line %d, want 1", got)
	}
}

// TestInsertSelector checks that the inspector's selector becomes a rule
// whose declarations go where the caret is left.
func TestInsertSelector(t *testing.T) {
	a := newApp()
	a.insertSelector("strong:revealed")
	src := a.css.Buffer.String()
	if !strings.Contains(src, "strong:revealed {") {
		t.Fatalf("no rule in %q", src)
	}
	line := a.css.Buffer.Pos(a.css.Cursor())
	if got := a.css.Buffer.Line(line.Line); got != "  " {
		t.Fatalf("the caret is on %q, not the blank line of the rule", got)
	}
}

// TestOffsetAt checks the conversion of a theme's line and column into a
// byte offset.
func TestOffsetAt(t *testing.T) {
	const src = "a\n你好\nccc"
	cases := []struct {
		line, col, want int
	}{
		{1, 1, 0},
		{1, 2, 1},
		{2, 1, 2},
		{2, 2, 5},
		{2, 3, 8},
		{3, 3, 11},
		{3, 4, len(src)},
		{9, 1, len(src)},
	}
	for _, c := range cases {
		if got := offsetAt(src, c.line, c.col); got != c.want {
			t.Errorf("offsetAt(%d, %d) = %d, want %d", c.line, c.col, got, c.want)
		}
	}
}
