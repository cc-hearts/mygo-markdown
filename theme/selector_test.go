package theme

import (
	"strings"
	"testing"
)

// elem reads an element written as a selector, for tests:
// "cell.header[align=center]:first-child".
func elem(s string) Element {
	var e Element
	i := strings.IndexAny(s, ".[:")
	if i < 0 {
		i = len(s)
	}
	e.Type, s = s[:i], s[i:]
	for s != "" {
		end := strings.IndexAny(s[1:], ".[:")
		if s[0] == '[' {
			end = strings.IndexByte(s, ']') - 1
		}
		if end < 0 {
			end = len(s) - 1
		}
		part := s[1 : end+1]
		switch s[0] {
		case '.':
			e.Classes = append(e.Classes, part)
		case '[':
			name, val, _ := strings.Cut(part, "=")
			e.Attrs = append(e.Attrs, Attr{name, strings.Trim(val, `"`)})
			end++
		case ':':
			for _, st := range stateNames {
				if st.name == part {
					e.States |= st.s
				}
			}
		}
		s = s[end+1:]
	}
	return e
}

// chain returns a node of a path of elements written as selectors, with
// no resolver: enough to match selectors against.
func chain(path ...string) *Node {
	n := &Node{el: Element{Type: "document"}}
	for _, s := range path {
		n = &Node{parent: n, el: elem(s)}
	}
	return n
}

func sel(t *testing.T, s string) *selector {
	t.Helper()
	d := &diagnostics{src: s}
	ss := parseSelectors(tokenize(s, d), d)
	if len(ss) != 1 {
		t.Fatalf("%q: %v", s, d.list)
	}
	return ss[0]
}

func TestSpecificity(t *testing.T) {
	cases := []struct {
		s    string
		want specificity
	}{
		{"*", specificity{0, 0}},
		{"heading", specificity{0, 1}},
		{"heading.h1", specificity{1, 1}},
		{"quote strong", specificity{0, 2}},
		{"list.tight > item > *", specificity{1, 2}},
		{"code.block[lang=go]:revealed", specificity{3, 1}},
		{"item:not(.checked, .task.checked)::marker", specificity{2, 2}},
		{":root", specificity{1, 0}},
		{"::search-text:current", specificity{1, 1}},
	}
	for _, c := range cases {
		if got := sel(t, c.s).spec; got != c.want {
			t.Errorf("%q: got %v, want %v", c.s, got, c.want)
		}
	}
}

func TestMatch(t *testing.T) {
	cases := []struct {
		s      string
		path   []string
		pseudo string
		ps     States
		want   bool
	}{
		{"quote strong", []string{"quote", "paragraph", "strong"}, "", 0, true},
		{"quote strong", []string{"paragraph", "strong"}, "", 0, false},
		{"quote > strong", []string{"quote", "paragraph", "strong"}, "", 0, false},
		{"list.tight > item > *", []string{"list.bullet.tight", "item", "paragraph"}, "", 0, true},
		{"list.tight > item > *", []string{"list.bullet.loose", "item", "paragraph"}, "", 0, false},
		{"list.tight > item > *", []string{"list.tight", "item", "paragraph", "strong"}, "", 0, false},
		{"item.checked > paragraph:first-child", []string{"list", "item.task.checked", "paragraph:first-child"}, "", 0, true},
		{"item.checked > paragraph:first-child", []string{"list", "item.task.checked", "paragraph:last-child"}, "", 0, false},
		{"item:not(.checked)", []string{"list", "item.task"}, "", 0, true},
		{"item:not(.checked)", []string{"list", "item.task.checked"}, "", 0, false},
		{`code.block[lang^="go"]`, []string{`code.block[lang="golang"]`}, "", 0, true},
		{`code[lang="GO" i]`, []string{`code.block[lang="go"]`}, "", 0, true},
		{`code[lang="GO"]`, []string{`code.block[lang="go"]`}, "", 0, false},
		{`link[href*="example"]`, []string{"paragraph", `link[href="https://example.com"]`}, "", 0, true},
		{`link[title]`, []string{"paragraph", `link[href="x"]`}, "", 0, false},
		{"item::marker", []string{"list", "item"}, "marker", 0, true},
		{"item::marker", []string{"list", "item"}, "", 0, false},
		{"item", []string{"list", "item"}, "marker", 0, false},
		{"::selection", nil, "selection", 0, true},
		{":root::selection", nil, "selection", 0, true},
		{"::search-text:current", nil, "search-text", 0, false},
		{"::search-text:current", nil, "search-text", Current, true},
		{"::search-text", nil, "search-text", Current, true},
		{":root", nil, "", 0, true},
		{":root", []string{"paragraph"}, "", 0, false},
		{"document > paragraph", []string{"paragraph"}, "", 0, true},
		{"heading code.inline", []string{"heading.h1", "code.inline"}, "", 0, true},
		{"math.block:revealed", []string{"math.block"}, "", 0, false},
	}
	for _, c := range cases {
		n := chain(c.path...)
		if got := sel(t, c.s).matches(n, c.pseudo, c.ps); got != c.want {
			t.Errorf("%q on %v %q %v: got %v", c.s, c.path, c.pseudo, c.ps, got)
		}
	}
}

func TestElementString(t *testing.T) {
	e := Element{Type: "code", Classes: []string{"block"}, Attrs: []Attr{{"lang", "go"}}, States: Revealed | FirstChild}
	if got := e.String(); got != `code.block[lang="go"]:first-child:revealed` {
		t.Errorf("got %s", got)
	}
	if got := elem(e.String()); got.String() != e.String() {
		t.Errorf("round trip: %s", got)
	}
}
