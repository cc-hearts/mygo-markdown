package livemark

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

// harness runs an editor in a headless window.
type harness struct {
	t    *testing.T
	e    *Editor
	tt   *ui.Tester
	clip transfer.Data
}

func newHarness(t *testing.T, src string) *harness {
	t.Helper()
	h := &harness{t: t}
	h.e = New(src, Options{
		Placeholder: "开始写作…",
		ReadClipboard: func() transfer.Data {
			return h.clip
		},
		WriteClipboard: func(d transfer.Data) { h.clip = d },
	})
	h.tt = ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			h.e.View(c).Grow(1)
		})
	}, 1000, 700)
	h.tt.Frame()
	h.tt.Frame()
	h.focus()
	return h
}

// focus clicks into the editor so that it takes the keyboard.
func (h *harness) focus() {
	h.e.Focus()
	h.tt.Frame()
	h.tt.Frame()
}

// frame runs a frame.
func (h *harness) frame() { h.tt.Frame() }

// caretAt puts the caret at off, as a click would.
func (h *harness) caretAt(off int) {
	h.e.dropPhantom()
	h.e.anchor, h.e.caret = off, off
	h.e.wantReveal = true
	h.frame()
}

// typ types text.
func (h *harness) typ(s string) {
	h.tt.Type(s)
	h.frame()
}

// key presses a key.
func (h *harness) key(mods ui.Modifiers, k ui.Key) {
	h.tt.Key(mods, k)
	h.frame()
}

// want checks the source.
func (h *harness) want(src string) {
	h.t.Helper()
	if got := h.e.Source(); got != src {
		h.t.Fatalf("source = %q, want %q", got, src)
	}
}

// shot writes the window to a PNG in the test's output directory, for a
// look at what it draws.
func (h *harness) shot(name string) {
	dir := os.Getenv("MDEDIT_SHOTS")
	if dir == "" {
		return
	}
	h.frame()
	img := h.tt.Image()
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		h.t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		h.t.Fatal(err)
	}
}

const sample = `# Muse 编辑器

这是一个 **所见即所得** 的 *Markdown* 编辑器，支持 ` + "`行内代码`" + `、[链接](https://example.com) 和 ~~删除线~~。

## 列表

- 第一项
- 第二项
  - 嵌套项
- [ ] 待办
- [x] 已完成

1. 有序一
2. 有序二

> 引用的文字
> 第二行

` + "```go\nfunc main() {\n\tfmt.Println(\"hi\")\n}\n```" + `

$$
E = mc^2
$$

| 名称 | 数量 |
| :-- | --: |
| 苹果 | 3 |
| 香蕉 | 12 |

---

行内公式 $a^2+b^2=c^2$ 结束。
`

func TestRenderSample(t *testing.T) {
	h := newHarness(t, sample)
	h.shot("sample")
	if len(h.e.leaves) < 10 {
		t.Fatalf("leaves = %d", len(h.e.leaves))
	}
	for _, l := range h.e.leaves {
		if l.h <= 0 && l.kind != kTable {
			t.Errorf("leaf %v at %d has no height", l.kind, l.start)
		}
	}
}

func TestTypeInEmptyDocument(t *testing.T) {
	h := newHarness(t, "")
	h.shot("empty")
	h.typ("# Hello")
	h.want("# Hello")
	h.key(0, ui.KeyEnter)
	h.typ("world")
	h.want("# Hello\n\nworld")
	h.shot("typed")
}

func TestTypingAtCaret(t *testing.T) {
	h := newHarness(t, "hello world")
	h.caretAt(5)
	h.typ(",")
	h.want("hello, world")
	h.key(0, ui.KeyBackspace)
	h.want("hello world")
}

func TestEnterSplitsParagraph(t *testing.T) {
	h := newHarness(t, "hello world")
	h.caretAt(5)
	h.key(0, ui.KeyEnter)
	h.want("hello\n\nworld")
	if !strings.HasPrefix(h.e.Source()[h.e.caret:], "world") {
		t.Fatalf("caret at %d", h.e.caret)
	}
}

func TestEnterAtEndMakesPhantom(t *testing.T) {
	h := newHarness(t, "one\n\ntwo\n")
	h.caretAt(3)
	h.key(0, ui.KeyEnter)
	h.want("one\n\ntwo\n")
	if h.e.phantom == nil {
		t.Fatal("no empty paragraph after Enter at the end")
	}
	h.typ("x")
	h.want("one\n\nx\n\ntwo\n")
}

func TestListEnter(t *testing.T) {
	h := newHarness(t, "- a")
	h.caretAt(3)
	h.key(0, ui.KeyEnter)
	h.want("- a\n- ")
	h.typ("b")
	h.want("- a\n- b")
	h.key(0, ui.KeyEnter)
	h.key(0, ui.KeyEnter)
	h.typ("p")
	h.want("- a\n- b\n\np")
}

func TestOrderedListEnterIncrements(t *testing.T) {
	h := newHarness(t, "1. a")
	h.caretAt(4)
	h.key(0, ui.KeyEnter)
	h.want("1. a\n2. ")
}

func TestBackspaceHeadingToParagraph(t *testing.T) {
	h := newHarness(t, "## Title")
	h.caretAt(3)
	h.key(0, ui.KeyBackspace)
	h.want("Title")
}

func TestBackspaceJoinsParagraphs(t *testing.T) {
	h := newHarness(t, "one\n\ntwo")
	h.caretAt(5)
	h.key(0, ui.KeyBackspace)
	h.want("onetwo")
}

func TestBoldShortcut(t *testing.T) {
	h := newHarness(t, "make this bold")
	h.e.Select(10, 14)
	h.frame()
	h.key(ui.Cmd, ui.KeyB)
	h.want("make this **bold**")
	h.key(ui.Cmd, ui.KeyB)
	h.want("make this bold")
}

func TestMarkersRevealAtCaret(t *testing.T) {
	h := newHarness(t, "a **b** c")
	h.caretAt(0)
	l := h.e.leaves[0]
	if strings.Contains(l.view.disp.text, "*") {
		t.Fatalf("markers show away from the caret: %q", l.view.disp.text)
	}
	h.caretAt(4)
	l = h.e.leaves[0]
	if !strings.Contains(l.view.disp.text, "**") {
		t.Fatalf("markers hidden at the caret: %q", l.view.disp.text)
	}
}

func TestArrowsWalkThroughMarkers(t *testing.T) {
	h := newHarness(t, "x**b**y")
	h.caretAt(0)
	var seen []int
	for i := 0; i < 7; i++ {
		h.key(0, ui.KeyRight)
		seen = append(seen, h.e.caret)
	}
	want := []int{1, 2, 3, 4, 5, 6, 7}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("carets = %v, want %v", seen, want)
		}
	}
}

func TestArrowsCrossBlocks(t *testing.T) {
	h := newHarness(t, "# One\n\ntwo")
	h.caretAt(5)
	h.key(0, ui.KeyRight)
	if h.e.caret != 7 {
		t.Fatalf("caret = %d, want 7", h.e.caret)
	}
	h.key(0, ui.KeyLeft)
	if h.e.caret != 5 {
		t.Fatalf("caret = %d, want 5", h.e.caret)
	}
	h.key(0, ui.KeyDown)
	if h.e.caret < 7 {
		t.Fatalf("down: caret = %d", h.e.caret)
	}
	h.key(0, ui.KeyUp)
	if h.e.caret > 5 {
		t.Fatalf("up: caret = %d", h.e.caret)
	}
}

func TestCodeFenceAutoCloses(t *testing.T) {
	h := newHarness(t, "")
	h.typ("```")
	h.want("```\n\n```")
	h.typ("go")
	h.key(0, ui.KeyEnter)
	h.typ("x := 1")
	h.want("```go\nx := 1\n```")
	h.shot("code")
}

func TestMathBlockShortcut(t *testing.T) {
	h := newHarness(t, "")
	h.typ("$$")
	h.want("$$\n\n$$")
	h.typ("x^2")
	h.want("$$\nx^2\n$$")
}

func TestTableShortcutAndTab(t *testing.T) {
	h := newHarness(t, "")
	h.typ("| a | b |")
	h.key(0, ui.KeyEnter)
	h.want("| a | b |\n| --- | --- |\n|   |   |")
	h.typ("1")
	h.key(0, ui.KeyTab)
	h.typ("2")
	if got := h.e.Source(); !strings.Contains(got, "1") || !strings.Contains(got, "2") {
		t.Fatalf("source = %q", got)
	}
	h.shot("table")
}

func TestUndoRedo(t *testing.T) {
	h := newHarness(t, "abc")
	h.caretAt(3)
	h.typ("def")
	h.want("abcdef")
	h.key(ui.Cmd, ui.KeyZ)
	h.want("abc")
	h.key(ui.Cmd|ui.Shift, ui.KeyZ)
	h.want("abcdef")
}

func TestCompose(t *testing.T) {
	h := newHarness(t, "前文")
	h.caretAt(len("前文"))
	h.tt.Compose("ni", 2)
	h.frame()
	h.want("前文")
	h.tt.Compose("", 0)
	h.tt.Type("你")
	h.frame()
	h.want("前文你")
}

func TestCopyPaste(t *testing.T) {
	h := newHarness(t, "one **two** three")
	h.e.Select(4, 11)
	h.frame()
	h.key(ui.Cmd, ui.KeyC)
	if b, _ := h.clip.Read(transfer.Text); string(b) != "**two**" {
		t.Fatalf("copied %q", b)
	}
	h.caretAt(len(h.e.Source()))
	h.key(ui.Cmd, ui.KeyV)
	h.want("one **two** three**two**")
}

func TestTaskToggleByClick(t *testing.T) {
	h := newHarness(t, "- [ ] todo")
	l := h.e.leaves[0]
	x := h.e.colX + h.e.indentOf(l.conts) - h.e.baseSize()*0.9
	y := l.y + h.e.baseSize()*0.8
	h.tt.ClickAt(x, y+0)
	h.frame()
	h.want("- [x] todo")
}

func TestQuoteEnterExits(t *testing.T) {
	h := newHarness(t, "> quoted")
	h.caretAt(len("> quoted"))
	h.key(0, ui.KeyEnter)
	h.typ("more")
	h.want("> quoted\n>\n> more")
	h.key(0, ui.KeyEnter)
	h.key(0, ui.KeyEnter)
	h.typ("out")
	h.want("> quoted\n>\n> more\n\nout")
}

func TestHeadingShortcuts(t *testing.T) {
	h := newHarness(t, "title")
	h.caretAt(2)
	h.key(ui.Cmd, ui.Key2)
	h.want("## title")
	h.key(ui.Cmd, ui.Key0)
	h.want("title")
}

func TestPendingHashShowsAsText(t *testing.T) {
	h := newHarness(t, "")
	h.typ("#")
	h.want("#")
	if l := h.e.leaves[0]; !l.pending {
		t.Fatalf("# alone is not pending: %+v", l.kind)
	}
	h.typ(" x")
	h.want("# x")
	if l := h.e.leaves[0]; l.kind != kHeading || l.pending {
		t.Fatal("# x is not a heading")
	}
}

func TestSelectAllDelete(t *testing.T) {
	h := newHarness(t, "# a\n\nb\n\n- c")
	h.key(ui.Cmd, ui.KeyA)
	h.key(0, ui.KeyBackspace)
	h.want("")
	h.typ("new")
	h.want("new")
}
