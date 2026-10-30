package theme

import (
	"math"
	"strings"
	"testing"
)

// uiVars are the variables the editor sets from MyGo's light theme.
var uiVars = map[string]string{
	"--ui-text": "#18181b", "--ui-text-muted": "#71717a", "--ui-accent": "#2563eb",
	"--ui-border": "#d9d9de", "--ui-surface": "#f4f4f5", "--ui-selection": "#2563eb40",
}

// only returns a resolver of sheets alone, without default.css.
func only(t *testing.T, dark bool, css ...string) *Resolver {
	t.Helper()
	th := &Theme{}
	for _, c := range css {
		s, diags := Parse("t.css", c)
		for _, d := range diags {
			t.Errorf("unexpected diagnostic %s", d)
		}
		th.layers = append(th.layers, s)
	}
	return th.Resolver(Env{Dark: dark, Vars: uiVars})
}

// at returns the node of a path of elements written as selectors.
func at(r *Resolver, path ...string) *Node {
	n := r.Root()
	for _, s := range path {
		n = n.Child(elem(s))
	}
	return n
}

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }

var (
	red   = Color{255, 0, 0, 255}
	green = Color{0, 128, 0, 255}
	blue  = Color{0, 0, 255, 255}
	black = Color{0, 0, 0, 255}
)

func TestCascadeOrder(t *testing.T) {
	r := only(t, false,
		`paragraph { color: red } quote paragraph { color: green } paragraph { color: blue }`,
		`paragraph { color: red }`,
	)
	if c := at(r, "paragraph").Style().Color; c != red {
		t.Errorf("a later layer wins: %v", c)
	}
	if c := at(r, "quote", "paragraph").Style().Color; c != red {
		t.Errorf("a later layer wins over specificity: %v", c)
	}
	r = only(t, false, `quote paragraph { color: green } paragraph { color: blue }`)
	if c := at(r, "quote", "paragraph").Style().Color; c != green {
		t.Errorf("specificity wins over order: %v", c)
	}
	if c := at(r, "paragraph").Style().Color; c != blue {
		t.Errorf("paragraph: %v", c)
	}
	r = only(t, false, `paragraph { color: red !important } paragraph { color: blue }`, `paragraph { color: green }`)
	if c := at(r, "paragraph").Style().Color; c != red {
		t.Errorf("important wins over later layers: %v", c)
	}
	r = only(t, false, `paragraph { color: red !important }`, `paragraph { color: green !important }`)
	if c := at(r, "paragraph").Style().Color; c != green {
		t.Errorf("a later layer wins among important ones: %v", c)
	}
}

func TestInheritance(t *testing.T) {
	r := only(t, false, `
quote { color: red; padding-left: 20px; font-size: 20px; }
quote heading.h1 { padding-left: inherit; }
quote heading.h2 { color: initial; font-size: unset; padding-left: unset; }
`)
	p := at(r, "quote", "paragraph").Style()
	if p.Color != red || p.Padding.Left != 0 || p.FontSize != 20 {
		t.Errorf("color and font-size inherit, padding does not: %+v", p)
	}
	if x := at(r, "quote", "heading.h1").Style(); x.Padding.Left != 20 {
		t.Errorf("padding: inherit: %v", x.Padding.Left)
	}
	y := at(r, "quote", "heading.h2").Style()
	if y.Color != black || y.FontSize != 20 || y.Padding.Left != 0 {
		t.Errorf("initial and unset: %+v", y)
	}
	if root := r.Root().Style(); root.FontSize != 16 || root.LineHeight != 1.75 || root.FontWeight != 400 || root.MaxWidth != 820 {
		t.Errorf("initial values: %+v", root)
	}
}

func TestVariables(t *testing.T) {
	r := only(t, false, `
:root { --c: red; --m: 4px; --a: var(--b); --b: var(--a); --self: var(--self); --d: var(--c); }
quote { --c: green; color: green; }
paragraph { color: var(--c); margin: var(--m) 0; }
heading.h1 { color: var(--nope, blue); }
heading.h2 { color: var(--nope); }
heading.h3 { color: var(--a, blue); }
heading.h4 { color: var(--ui-accent); }
heading.h5 { color: var(--d); }
`)
	if s := at(r, "paragraph").Style(); s.Color != red || s.Margin.Top != 4 || s.Margin.Bottom != 4 || s.Margin.Left != 0 {
		t.Errorf("var(): %+v", s)
	}
	if s := at(r, "quote", "paragraph").Style(); s.Color != green {
		t.Errorf("variables inherit and override: %v", s.Color)
	}
	if s := at(r, "heading.h1").Style(); s.Color != blue {
		t.Errorf("fallback: %v", s.Color)
	}
	if s := at(r, "quote", "heading.h2").Style(); s.Color != green {
		t.Errorf("an invalid color is unset, which inherits: %v", s.Color)
	}
	if s := at(r, "heading.h3").Style(); s.Color != blue {
		t.Errorf("a cycle is invalid, so the fallback applies: %v", s.Color)
	}
	if s := at(r, "heading.h4").Style(); s.Color != (Color{0x25, 0x63, 0xeb, 255}) {
		t.Errorf("--ui-*: %v", s.Color)
	}
	if s := at(r, "quote", "heading.h5").Style(); s.Color != red {
		t.Errorf("a variable's var() is read where it is declared: %v", s.Color)
	}
	if v, ok := r.Root().Style().Var("--m"); !ok || v != "4px" {
		t.Errorf("Var: %q %v", v, ok)
	}
}

func TestMedia(t *testing.T) {
	css := `paragraph { color: red } @media (prefers-color-scheme: dark) { paragraph { color: green } }`
	if c := at(only(t, false, css), "paragraph").Style().Color; c != red {
		t.Errorf("light: %v", c)
	}
	if c := at(only(t, true, css), "paragraph").Style().Color; c != green {
		t.Errorf("dark: %v", c)
	}
}

func TestLengths(t *testing.T) {
	r := only(t, false, `
:root { font-size: 20px; }
heading { font-size: 2em; padding-top: 1em; padding-left: 1rem; margin-top: calc(1em + 2px); }
heading strong { font-size: 50%; }
code.inline { font-size: .875em; border-radius: max(3px, calc(.2em / .875)); }
paragraph { line-height: 24px; }
quote { line-height: 1.5; }
`)
	h := at(r, "heading").Style()
	if h.FontSize != 40 || h.Padding.Top != 40 || h.Padding.Left != 20 || h.Margin.Top != 42 {
		t.Errorf("em, rem, calc: %+v", h)
	}
	if s := at(r, "heading", "strong").Style(); s.FontSize != 20 {
		t.Errorf("%%: %v", s.FontSize)
	}
	if c := at(r, "paragraph", "code.inline").Style(); !near(c.FontSize, 17.5) || !near(c.Radius, 4) {
		t.Errorf("code: size %v radius %v", c.FontSize, c.Radius)
	}
	if c := at(r, "heading", "code.inline").Style(); !near(c.FontSize, 35) || !near(c.Radius, 8) {
		t.Errorf("code in a heading: size %v radius %v", c.FontSize, c.Radius)
	}
	if p := at(r, "paragraph").Style(); !near(p.LineHeight, 1.2) {
		t.Errorf("line-height in px: %v", p.LineHeight)
	}
	if c := at(r, "paragraph", "code.inline").Style(); !near(c.LineHeight*c.FontSize, 24) {
		t.Errorf("a line height in px inherits as one: %v", c.LineHeight)
	}
	if c := at(r, "quote", "heading").Style(); c.LineHeight != 1.5 {
		t.Errorf("a multiple inherits as one: %v", c.LineHeight)
	}
}

func TestPropagation(t *testing.T) {
	r := only(t, false, `
heading.h1 { text-decoration: line-through red; }
mark { background-color: yellow; border-radius: 2px; }
sup { font-size: .75em; vertical-align: super; }
quote { opacity: .5; color: rgb(0 0 255); }
quote strong { opacity: .5; }
item::marker { color: red; }
`)
	if s := at(r, "heading.h1", "strong").Style(); !s.Decoration.LineThrough || s.Decoration.Color != red {
		t.Errorf("decoration passes to inlines: %+v", s.Decoration)
	}
	if s := at(r, "heading.h1", "syntax.strong").Style(); s.Decoration.LineThrough {
		t.Error("decoration passes to syntax")
	}
	yellow := Color{255, 255, 0, 255}
	if s := at(r, "paragraph", "mark", "strong").Style(); s.TextBackground != yellow || s.TextRadius != 2 {
		t.Errorf("text background passes to inlines: %v %v", s.TextBackground, s.TextRadius)
	}
	if s := at(r, "paragraph", "mark", "syntax.strong").Style(); s.TextBackground.A != 0 {
		t.Errorf("text background passes to syntax: %v", s.TextBackground)
	}
	if s := at(r, "paragraph", "mark").Style(); s.Background != yellow {
		t.Errorf("own background: %v", s.Background)
	}
	if s := at(r, "paragraph", "sup", "strong").Style(); !near(s.Shift, 5.6) {
		t.Errorf("shift passes to inlines: %v", s.Shift)
	}
	if s := at(r, "paragraph", "sup", "sup").Style(); !near(s.Shift, 5.6+4.2) {
		t.Errorf("shifts add up: %v", s.Shift)
	}
	if s := at(r, "paragraph").Style(); s.Shift != 0 {
		t.Errorf("blocks do not shift: %v", s.Shift)
	}
	q := at(r, "quote").Style()
	if q.Opacity != .5 || q.Color != (Color{0, 0, 255, 128}) {
		t.Errorf("opacity: %v %v", q.Opacity, q.Color)
	}
	if s := at(r, "quote", "paragraph", "strong").Style(); s.Opacity != .25 || s.Color != (Color{0, 0, 255, 64}) {
		t.Errorf("opacities multiply: %v %v", s.Opacity, s.Color)
	}
	if m := at(r, "list", "item").Pseudo("marker", 0); m.Color != red || m.FontSize != 16 {
		t.Errorf("pseudo-element: %+v", m)
	}
}

func TestNodes(t *testing.T) {
	r := only(t, false, `paragraph { color: red }`)
	a := at(r, "quote", "paragraph")
	if b := at(r, "quote", "paragraph"); a != b {
		t.Error("the same path is not the same node")
	}
	if a.Style() != a.Style() {
		t.Error("the style is computed again")
	}
	if got := a.Path(); got != "document › quote › paragraph" {
		t.Errorf("path: %s", got)
	}
	if !a.Style().Specified("color") || a.Style().Specified("font-size") || a.Parent().Style().Specified("color") {
		t.Error("Specified")
	}
}

func TestVariablesGrowBounded(t *testing.T) {
	css := ":root { --a: x x x x x x x x x x; "
	prev := "--a"
	for _, name := range []string{"--b", "--c", "--d", "--e", "--f", "--g", "--h"} {
		css += name + ":" + strings.Repeat(" var("+prev+")", 10) + "; "
		prev = name
	}
	css += "} paragraph { color: var(--h, red); } quote { color: var(--c, red); }"
	r := only(t, false, css)
	if c := at(r, "paragraph").Style().Color; c != red {
		t.Errorf("a variable of 10^8 tokens: %v", c)
	}
	if _, ok := r.Root().Style().Var("--c"); !ok {
		t.Error("--c, of 1000 tokens, is left out")
	}
	if _, ok := r.Root().Style().Var("--d"); ok {
		t.Error("--d, of 10000 tokens, is kept")
	}
}
