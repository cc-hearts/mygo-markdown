package livemark

import (
	"strings"
	"testing"
)

// TestSyntaxSnapshot checks that Syntax returns the tree of the source as
// it is, and keeps it until the source changes.
func TestSyntaxSnapshot(t *testing.T) {
	e := New("# Title\n\nSome **bold** text.\n", Options{})
	tr := e.Syntax()
	if got := tr.Source(); got != e.Source() {
		t.Fatalf("Syntax source = %q, want %q", got, e.Source())
	}
	if tr.Root().Kind().String() != "document" {
		t.Fatalf("root = %v, want document", tr.Root().Kind())
	}
	if again := e.Syntax(); again != tr {
		t.Fatal("Syntax parsed again without a change of the source")
	}
	e.Replace(len(e.src), len(e.src), "More.\n")
	if e.Syntax() == tr {
		t.Fatal("Syntax kept the old tree after a change of the source")
	}
}

// TestStylePath checks the elements a theme matches at an offset, block
// and inline.
func TestStylePath(t *testing.T) {
	src := "> Some **bold** and `code`.\n"
	e := New(src, Options{})
	cases := []struct {
		at   string
		want []string
	}{
		{"bold", []string{"document", "quote", "paragraph", "strong"}},
		{"code", []string{"document", "quote", "paragraph", "code"}},
		{"Some", []string{"document", "quote", "paragraph"}},
	}
	for _, c := range cases {
		off := strings.Index(src, c.at)
		var got []string
		for _, el := range e.StylePath(off) {
			got = append(got, el.Type)
		}
		if strings.Join(got, " ") != strings.Join(c.want, " ") {
			t.Errorf("StylePath(%q) = %v, want %v", c.at, got, c.want)
		}
	}
}
