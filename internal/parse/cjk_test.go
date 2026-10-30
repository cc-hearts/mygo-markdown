package parse

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/egoist/mygo/ui/markdown/parser"
)

// parseFriendly parses src as the editor and the syntax tree do: with the
// CJK-friendly emphasis swaps of CJKFriendly.
func parseFriendly(src string) *parser.Document {
	d := parser.New(Exts)
	d.Set(CJKFriendly(src))
	return d
}

// paragraphs returns the indices of the document's paragraphs, in order.
func paragraphs(d *parser.Document) []int {
	var out []int
	for i := 0; i < d.Len(); i++ {
		if d.Block(i).Kind == parser.Paragraph {
			out = append(out, i)
		}
	}
	return out
}

// paraText returns the text of a paragraph as displayed: its Text inlines
// read from the source, which has the characters CJKFriendly swapped.
func paraText(d *parser.Document, src string, block int) string {
	var b strings.Builder
	for _, in := range d.Inlines(block) {
		switch in.Kind {
		case parser.Text:
			b.WriteString(src[in.Start:in.End])
		case parser.SoftBreak, parser.LineBreak:
			b.WriteByte(' ')
		}
	}
	return b.String()
}

// strongTexts returns the text of a paragraph's strongs, read from the
// source.
func strongTexts(d *parser.Document, src string, block int) []string {
	ins := d.Inlines(block)
	var out []string
	for k := range ins {
		if ins[k].Kind != parser.Strong {
			continue
		}
		var b strings.Builder
		for j := k + 1; j < ins[k].Next && j < len(ins); j++ {
			if ins[j].Kind == parser.Text {
				b.WriteString(src[ins[j].Start:ins[j].End])
			}
		}
		out = append(out, b.String())
	}
	return out
}

func TestStrongBesideCJKPunctuation(t *testing.T) {
	for _, c := range []struct{ src, text, bold string }{
		{"**注意：**这是**重点**内容。", "注意：这是重点内容。", "注意：|重点"},
		{"中文**（括号）**中文", "中文（括号）中文", "（括号）"},
		{"**English:**中文", "English:中文", "English:"},
		{"“我说**是**”", "“我说是”", "是"},
		{"**“名称”**：**值**", "“名称”：值", "“名称”|值"},
	} {
		d := parseFriendly(c.src)
		blocks := paragraphs(d)
		if len(blocks) == 0 {
			t.Fatalf("%q has no paragraph", c.src)
		}
		if got := paraText(d, c.src, blocks[0]); got != c.text {
			t.Errorf("%q reads %q, want %q", c.src, got, c.text)
		}
		if got := strings.Join(strongTexts(d, c.src, blocks[0]), "|"); got != c.bold {
			t.Errorf("%q has bold %q, want %q", c.src, got, c.bold)
		}
	}
}

func TestStrongInListItems(t *testing.T) {
	src := "- **步骤一：**安装依赖\n- **步骤二：**运行\n"
	d := parseFriendly(src)
	blocks := paragraphs(d)
	if len(blocks) != 2 {
		t.Fatalf("got %d paragraphs, want 2", len(blocks))
	}
	if got, want := paraText(d, src, blocks[0]), "步骤一：安装依赖"; got != want {
		t.Errorf("first item reads %q, want %q", got, want)
	}
	if got, want := strings.Join(strongTexts(d, src, blocks[0]), "|"), "步骤一："; got != want {
		t.Errorf("first item bold %q, want %q", got, want)
	}
	if got, want := paraText(d, src, blocks[1]), "步骤二：运行"; got != want {
		t.Errorf("second item reads %q, want %q", got, want)
	}
}

func TestStrongAfterQuoteMarker(t *testing.T) {
	src := "> **注意：**引用\n"
	d := parseFriendly(src)
	blocks := paragraphs(d)
	if len(blocks) != 1 {
		t.Fatalf("got %d paragraphs, want 1", len(blocks))
	}
	if got := strings.Join(strongTexts(d, src, blocks[0]), "|"); got != "注意：" {
		t.Errorf("quote bold %q, want %q", got, "注意：")
	}
}

func TestCJKFriendlyKeepsOffsets(t *testing.T) {
	src := "**注意：**内容，**（括号）**中文，`**代码：**中`\n\n```\n**注意：**中文\n```\n"
	got := CJKFriendly(src)
	if len(got) != len(src) || got == src {
		t.Fatalf("CJKFriendly(%q) = %q", src, got)
	}
	for i := 0; i < len(src); {
		r, n := utf8.DecodeRuneInString(src[i:])
		if r < utf8.RuneSelf && got[i] != src[i] {
			t.Fatalf("ASCII at %d changed: %q", i, got[i])
		}
		if g, gn := utf8.DecodeRuneInString(got[i:]); gn != n || g == utf8.RuneError {
			t.Fatalf("character at %d changed length: %q for %q", i, g, r)
		}
		i += n
	}
	if plain := "No CJK here: **bold** and *em*."; CJKFriendly(plain) != plain {
		t.Fatal("text without CJK changed")
	}
	// The text of a code block is the source's: inline emphasis is not
	// parsed there.
	d := parseFriendly(src)
	found := false
	for i := 0; i < d.Len(); i++ {
		b := d.Block(i)
		if b.Kind != parser.CodeBlock {
			continue
		}
		found = true
		if text := src[b.Start:b.End]; !strings.Contains(text, "**注意：**中文") {
			t.Errorf("code block is %q", text)
		}
	}
	if !found {
		t.Fatal("no code block")
	}
}

func TestCJKFriendlyLongestStandIn(t *testing.T) {
	// A swapped character keeps its length, so the parser's offsets and
	// what the editor draws from the source stay aligned.
	for _, src := range []string{
		"**注意：**内容",
		"**汉：**字",
		"「**引号：**」",
		"**a（b）**：c",
	} {
		got := CJKFriendly(src)
		if len(got) != len(src) {
			t.Errorf("CJKFriendly(%q) is %d bytes, want %d", src, len(got), len(src))
		}
	}
}
