package theme

import (
	"strings"
	"testing"
)

func TestTokenize(t *testing.T) {
	d := &diagnostics{}
	ts := tokenize(`a.b-c > --x:hover{color:#fff;margin:-.5em +2px 1e2 50% "s\"q" url(x.png) \31 0}/* c */@media`, d)
	if len(d.list) != 0 {
		t.Fatalf("diagnostics: %v", d.list)
	}
	var got []string
	for _, tk := range ts {
		if tk.kind == tWhitespace {
			continue
		}
		got = append(got, tk.text())
	}
	want := []string{"a", ".", "b-c", ">", "--x", ":", "hover", "{", "color", ":", "#fff", ";", "margin", ":",
		"-0.5em", "2px", "100", "50%", `"s\"q"`, "url(x.png)", "10", "}", "@media"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("tokens\n got %q\nwant %q", got, want)
	}
}

func TestTokenizeKinds(t *testing.T) {
	cases := []struct {
		src  string
		kind tokKind
		val  string
		num  float64
	}{
		{"-.5em", tDimension, "em", -0.5},
		{"12PX", tDimension, "px", 12},
		{"3.5%", tPercentage, "", 3.5},
		{"+.5", tNumber, "", 0.5},
		{"rgb(", tFunction, "rgb", 0},
		{"URL( a.ttf )", tURL, "a.ttf", 0},
		{`url("a b.ttf")`, tFunction, "url", 0},
		{"-x", tIdent, "-x", 0},
		{"--x-y", tIdent, "--x-y", 0},
		{"#a1", tHash, "a1", 0},
		{"@font-face", tAtKeyword, "font-face", 0},
		{"'it''s'", tString, "it", 0},
		{"<!--", tCDO, "", 0},
		{"-->", tCDC, "", 0},
		{"中文", tIdent, "中文", 0},
	}
	for _, c := range cases {
		ts := tokenize(c.src, &diagnostics{})
		if len(ts) == 0 {
			t.Errorf("%q: no tokens", c.src)
			continue
		}
		if tk := ts[0]; tk.kind != c.kind || tk.val != c.val || tk.num != c.num {
			t.Errorf("%q: got kind %d val %q num %v, want kind %d val %q num %v", c.src, tk.kind, tk.val, tk.num, c.kind, c.val, c.num)
		}
	}
}

func TestTokenizeErrors(t *testing.T) {
	cases := []struct{ src, want string }{
		{"a {\n  color: red; /* open", "2:15: error: unclosed comment"},
		{"a { content: \"one\n}", "1:14: error: unclosed string"},
		{"x { src: url(a b) }", "1:10: error: malformed url(): quote an address with spaces, quotes or parentheses"},
		{"中文 { a: \"x\n", "1:9: error: unclosed string"},
	}
	for _, c := range cases {
		d := &diagnostics{src: c.src}
		tokenize(c.src, d)
		if len(d.list) == 0 || d.list[0].String() != c.want {
			t.Errorf("%q: got %v, want %s", c.src, d.list, c.want)
		}
	}
}
