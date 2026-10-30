package parse

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/markdown/parser"
)

func TestTextFrontMatter(t *testing.T) {
	tests := []struct {
		name, src string
		fm        int
	}{
		{"none", "hello", 0},
		{"front matter", "---\na: 1\n---\nbody", 12},
		{"crlf", "---\r\na: 1\r\n---\r\nbody\r\n", 15},
		{"empty", "---\n---\n", 0},
		{"unclosed", "---\na: 1\n", 0},
		{"indented", " ---\na\n---\n", 0},
		{"dots", "---\na: 1\n...\nbody", 12},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, fm := Text(tt.src)
			if fm != tt.fm {
				t.Fatalf("front matter ends at %d, want %d", fm, tt.fm)
			}
			if len(text) != len(tt.src) {
				t.Fatalf("text is %d bytes, src %d", len(text), len(tt.src))
			}
			if text[fm:] != tt.src[fm:] {
				t.Errorf("text after the front matter changed: %q", text[fm:])
			}
			for i := 0; i < fm; i++ {
				if tt.src[i] != '\n' && tt.src[i] != '\r' && text[i] != ' ' {
					t.Errorf("byte %d of the front matter is %q", i, text[i])
				}
			}
		})
	}
}

func TestTextCJK(t *testing.T) {
	src := "**注意：**内容"
	text, _ := Text(src)
	if len(text) != len(src) {
		t.Fatalf("masked text is %d bytes, src %d", len(text), len(src))
	}
	if text == src {
		t.Fatal("nothing was swapped beside the CJK punctuation")
	}
	d := parser.New(Exts)
	d.Set(text)
	var strong bool
	for _, in := range d.Inlines(1) {
		if in.Kind == parser.Strong {
			strong = true
		}
	}
	if !strong {
		t.Error("the masked source has no strong emphasis")
	}
}

func TestParseExtensions(t *testing.T) {
	d, fm := Parse("[^n] $x$ ~~s~~\n\n[^n]: note\n")
	if fm != 0 {
		t.Errorf("front matter: %d", fm)
	}
	kinds := map[parser.InlineKind]bool{}
	for _, in := range d.Inlines(1) {
		kinds[in.Kind] = true
	}
	for _, k := range []parser.InlineKind{parser.FootnoteRef, parser.Math, parser.Strikethrough} {
		if !kinds[k] {
			t.Errorf("no %v inline in %v", k, kinds)
		}
	}
}

func TestFrontMatterEnd(t *testing.T) {
	// The offset is the end of the closing line, not of its newline.
	src := "---\na: 1\n---\n"
	if got := frontMatterEnd(src); got != 12 {
		t.Errorf("frontMatterEnd: %d, want 12", got)
	}
	if got := frontMatterEnd("---\r\na: 1\r\n---\r\n"); got != 15 {
		t.Errorf("frontMatterEnd CRLF: %d, want 15", got)
	}
}

func TestMaskKeepsLines(t *testing.T) {
	src := "---\na: 1\n---\nbody"
	masked := maskFrontMatter(src, frontMatterEnd(src))
	if strings.Count(masked, "\n") != strings.Count(src, "\n") {
		t.Errorf("newlines changed: %q", masked)
	}
	if !strings.HasPrefix(masked, "   \n") {
		t.Errorf("masked front matter: %q", masked)
	}
}
