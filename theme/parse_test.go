package theme

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// diagStrings returns the diagnostics of a sheet as text.
func diagStrings(css string) []string {
	_, diags := Parse("t.css", css)
	var out []string
	for _, d := range diags {
		out = append(out, d.String())
	}
	return out
}

func TestParseDiagnostics(t *testing.T) {
	cases := []struct {
		name, css string
		want      []string
	}{
		{"clean", "heading.h1 { font-size: 2em; color: #333 !important }", nil},
		{"misspelled property", "paragraph {\n  font-szie: 2em;\n}",
			[]string{`2:3: warning: unknown property "font-szie" (did you mean "font-size"?)`}},
		{"invalid value", "paragraph { color: 12px }",
			[]string{`1:20: warning: invalid value for color: "12px"`}},
		{"negative padding", "paragraph { padding-top: -1px }",
			[]string{`1:26: warning: invalid value for padding-top: "-1px": it cannot be negative`}},
		{"unknown element", "headng { color: red }",
			[]string{`1:1: warning: unknown element "headng" (did you mean "heading"?)`}},
		{"unknown class", "heading.h7 { color: red }",
			[]string{`1:9: warning: heading has no class "h7" (did you mean "h1"?)`}},
		{"class of a digit", "heading.1 { color: red }",
			[]string{`1:8: error: a class name cannot start with a digit (heading levels are .h1 to .h6): the selector is dropped`}},
		{"unknown class of any type", ".chcked { color: red }",
			[]string{`1:2: warning: no element has the class "chcked" (did you mean "checked"?)`}},
		{"hover", "link:hover { color: red }",
			[]string{`1:6: warning: nodes have no hover state: the selector is dropped`}},
		{"sibling", "heading + paragraph { color: red }",
			[]string{`1:9: warning: the sibling combinators + and ~ are not supported: the selector is dropped`}},
		{"id", "#write { color: red }",
			[]string{`1:1: warning: ids are not supported: nodes have none: the selector is dropped`}},
		{"one bad selector of a list", "heading.1, paragraph { color: red }",
			[]string{`1:8: error: a class name cannot start with a digit (heading levels are .h1 to .h6): the selector is dropped`}},
		{"nested rule", "quote { paragraph { color: red } }",
			[]string{`1:9: error: nested rules are not supported`}},
		{"missing colon", "quote { color red }",
			[]string{`1:9: error: expected ":" after "color"`}},
		{"unclosed block", "quote { color: red;\nparagraph { color: blue }",
			[]string{`1:7: error: unclosed "{"`, `2:1: error: nested rules are not supported`}},
		{"stray brace", "} quote { color: red }",
			[]string{`1:1: error: unexpected "}"`}},
		{"no block", "quote color: red;",
			[]string{`1:17: error: expected "{" after the selector`}},
		{"import", `@import "x.css"; quote { color: red }`,
			[]string{`1:1: warning: @import is not supported: put the rules in this sheet`}},
		{"unknown at-rule", "@keyframes x { from { color: red } } quote { color: red }",
			[]string{`1:1: warning: @keyframes is not supported`}},
		{"media feature", "@media (min-width: 400px) { quote { color: red } }",
			[]string{`1:8: warning: only the media feature (prefers-color-scheme: dark | light) is supported`}},
		{"does not apply", "strong { margin-top: 1em }",
			[]string{`1:10: warning: margin-top does not apply to strong`}},
		{"applies through a shorthand", "paragraph { margin: 1em 0 }", nil},
		{"pseudo-element of another", "heading::marker { color: red }",
			[]string{`1:10: warning: heading has no ::marker`}},
		{"global pseudo-element", "paragraph::selection { color: red }",
			[]string{`1:12: warning: ::selection belongs to the document: write it alone or on :root`}},
		{"current", "::search-text:current { color: red } ::selection:current { color: red }",
			[]string{`1:49: error: ::selection takes no pseudo-class ":current": the selector is dropped`}},
		{"dashed border", "code.block { border: 1px dashed red }",
			[]string{`1:22: warning: borders are drawn solid: dashed is not supported`}},
		{"bolder", "strong { font-weight: bolder }",
			[]string{`1:23: warning: invalid value for font-weight: "bolder": bolder and lighter are not supported: give a weight`}},
		{"woff font", "@font-face { font-family: X; src: url(x.woff2) format(\"woff2\") }",
			[]string{`1:30: warning: only TrueType and OpenType fonts (.ttf, .otf, .ttc) are supported`,
				`1:1: warning: @font-face needs a font-family and a src of a TrueType or OpenType font`}},
		{"var is read later", "quote { color: var(--nope) }", nil},
		{"custom property", "quote { --My-Var: 12px 3em; }", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := diagStrings(c.css)
			if strings.Join(got, "\n") != strings.Join(c.want, "\n") {
				t.Errorf("diagnostics\n got %q\nwant %q", got, c.want)
			}
		})
	}
}

func TestParseRecovers(t *testing.T) {
	s, diags := Parse("t.css", `
quote { color red; padding-left: 2em; }
paragraph { margin: 1em 0 !important; color: #123 ! important }
`)
	if len(diags) != 1 {
		t.Fatalf("diagnostics: %v", diags)
	}
	if len(s.rules) != 2 {
		t.Fatalf("rules: %d", len(s.rules))
	}
	if d := s.rules[0].decls; len(d) != 1 || d[0].prop != "padding-left" {
		t.Errorf("quote's declarations: %+v", d)
	}
	d := s.rules[1].decls
	if len(d) != 2 || !d[0].important || !d[1].important || tokensText(d[1].val) != "#123" {
		t.Errorf("paragraph's declarations: %+v", d)
	}
}

func TestParseFontFace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "paper.css")
	css := `@font-face {
  font-family: "Paper Serif";
  src: url(fonts/a.woff2) format("woff2"), url("fonts/paper serif.ttf") format("truetype");
  font-weight: bold;
  font-style: italic;
}`
	if err := os.WriteFile(path, []byte(css), 0o644); err != nil {
		t.Fatal(err)
	}
	s, diags, err := ParseFile(path)
	if err != nil || len(diags) != 0 {
		t.Fatalf("err %v, diagnostics %v", err, diags)
	}
	want := FontFace{Family: "Paper Serif", Path: filepath.Join(dir, "fonts", "paper serif.ttf"), Weight: 700, Italic: true}
	if f := s.FontFaces(); len(f) != 1 || f[0] != want {
		t.Errorf("font faces: %+v, want %+v", f, want)
	}
	if s.Name() != "paper.css" {
		t.Errorf("name: %q", s.Name())
	}
}

func TestParseMedia(t *testing.T) {
	cases := []struct {
		q    string
		want media
	}{
		{"", mediaAll},
		{"(prefers-color-scheme: dark)", mediaDark},
		{"screen and (prefers-color-scheme: light)", mediaLight},
		{"(prefers-color-scheme: dark), (prefers-color-scheme: light)", mediaAll},
		{"print", mediaNone},
		{"(prefers-color-scheme: dark) and (prefers-color-scheme: light)", mediaNone},
	}
	for _, c := range cases {
		d := &diagnostics{src: c.q}
		if got := parseMedia(tokenize(c.q, d), d); got != c.want || len(d.list) != 0 {
			t.Errorf("%q: got %d %v, want %d", c.q, got, d.list, c.want)
		}
	}
}

// FuzzParse checks that no sheet makes the parser or the cascade panic,
// and that diagnostics point inside the sheet.
func FuzzParse(f *testing.F) {
	f.Add(DefaultCSS())
	f.Add(`quote strong { color: var(--a, color-mix(in srgb, red 30%, transparent)); margin: calc(1em - 2px) 0 }`)
	f.Add(`@media (prefers-color-scheme: dark) { :root { --a: var(--b) } } item::marker { list-style: "→" }`)
	f.Add(`@font-face { src: url( x ) } a[b^="c" i]:not(.d, .e)::selection { border: thin dashed }`)
	f.Fuzz(func(t *testing.T, css string) {
		// A sheet that takes this long hangs: panic in the timer, which
		// the fuzzer takes as a crash and keeps the sheet of.
		watchdog := time.AfterFunc(2*time.Second, func() { panic(fmt.Sprintf("FuzzParse hangs on %q", css)) })
		defer watchdog.Stop()
		s, diags := Parse("f.css", css)
		lines := strings.Count(css, "\n") + 1
		for _, d := range diags {
			if d.Line < 1 || d.Line > lines || d.Col < 1 {
				t.Fatalf("diagnostic out of the sheet: %s", d)
			}
		}
		r := New(s).Resolver(Env{Dark: len(css)%2 == 0, Vars: map[string]string{"--ui-text": css}})
		for _, path := range [][]string{{"quote", "paragraph", "strong"}, {"list.tight", "item.task.checked", "paragraph:first-child"}, {"code.block"}} {
			n := r.Root()
			for _, e := range path {
				n = n.Child(elem(e))
			}
			n.Style()
			n.Pseudo("marker", 0)
		}
		r.Root().Pseudo("search-text", Current)
	})
}
