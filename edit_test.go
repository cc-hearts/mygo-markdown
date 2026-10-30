package livemark

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

// shows reports whether the leaf at the caret displays text holding s.
func (h *harness) shows(s string) bool {
	h.e.layoutIfNeeded()
	u := h.e.unitAt(h.e.caret)
	if u == nil || u.view == nil {
		return false
	}
	return strings.Contains(u.view.disp.text, s)
}

func TestInlineMathRevealsAtCaret(t *testing.T) {
	h := newHarness(t, "公式 $x^2$ 结束")
	h.caretAt(0)
	if h.shows("$") {
		t.Fatal("TeX shows away from the caret")
	}
	h.caretAt(strings.Index(h.e.Source(), "$") + 1)
	if !h.shows("$x^2$") {
		t.Fatalf("TeX hidden at the caret: %q", h.e.leaves[0].view.disp.text)
	}
}

func TestImageRevealsSource(t *testing.T) {
	h := newHarness(t, "前文\n\n![图](missing.png)\n\n后文")
	h.caretAt(0)
	img := h.e.leaves[1]
	if strings.Contains(img.view.disp.text, "![") {
		t.Fatalf("image source shows away from the caret: %q", img.view.disp.text)
	}
	h.caretAt(strings.Index(h.e.Source(), "图"))
	if !h.shows("![图](missing.png)") {
		t.Fatalf("image source hidden at the caret")
	}
}

func TestCodeInfoString(t *testing.T) {
	h := newHarness(t, "```\nx\n```")
	h.caretAt(strings.Index(h.e.Source(), "x"))
	h.key(0, ui.KeyLeft)
	if h.e.caret != 3 {
		t.Fatalf("left from the code went to %d, want the info string at 3", h.e.caret)
	}
	h.typ("go")
	h.want("```go\nx\n```")
	h.key(0, ui.KeyEnter)
	if h.e.caret != strings.Index(h.e.Source(), "x") {
		t.Fatalf("enter in the info string went to %d", h.e.caret)
	}
}

func TestCodeEnterKeepsIndent(t *testing.T) {
	h := newHarness(t, "```go\nfunc f() {\n\tx := 1\n}\n```")
	h.caretAt(strings.Index(h.e.Source(), "1") + 1)
	h.key(0, ui.KeyEnter)
	h.typ("y")
	h.want("```go\nfunc f() {\n\tx := 1\n\ty\n}\n```")
}

func TestBackspaceEmptyCodeBlockRemovesIt(t *testing.T) {
	h := newHarness(t, "前文\n\n```\n\n```\n\n后文")
	h.caretAt(strings.Index(h.e.Source(), "```\n") + 4)
	h.key(0, ui.KeyBackspace)
	if strings.Contains(h.e.Source(), "```") {
		t.Fatalf("the empty block stayed: %q", h.e.Source())
	}
}

func TestBackspaceListItemBecomesParagraph(t *testing.T) {
	h := newHarness(t, "- one\n- two")
	h.caretAt(strings.Index(h.e.Source(), "two"))
	h.key(0, ui.KeyBackspace)
	h.want("- one\n\ntwo")
}

func TestBackspaceInsideQuoteLiftsParagraph(t *testing.T) {
	h := newHarness(t, "> quoted")
	h.caretAt(2)
	h.key(0, ui.KeyBackspace)
	h.want("quoted")
}

func TestDeleteJoinsNextParagraph(t *testing.T) {
	h := newHarness(t, "one\n\ntwo")
	h.caretAt(3)
	h.key(0, ui.KeyDelete)
	h.want("onetwo")
}

func TestTabNestsAndShiftTabLifts(t *testing.T) {
	h := newHarness(t, "- a\n- b")
	h.caretAt(strings.Index(h.e.Source(), "b"))
	h.key(0, ui.KeyTab)
	h.want("- a\n  - b")
	h.key(ui.Shift, ui.KeyTab)
	h.want("- a\n- b")
}

func TestLinkShortcut(t *testing.T) {
	h := newHarness(t, "see docs here")
	h.e.Select(4, 8)
	h.frame()
	h.key(ui.Cmd, ui.KeyK)
	h.want("see [docs]() here")
	h.typ("https://x.io")
	h.want("see [docs](https://x.io) here")
}

func TestPasteInQuoteContinuesIt(t *testing.T) {
	h := newHarness(t, "> start")
	h.caretAt(len("> start"))
	h.clip = transfer.TextData(" one\ntwo")
	h.key(ui.Cmd, ui.KeyV)
	h.want("> start one\n> two")
}

func TestComposeInEmptyDocument(t *testing.T) {
	h := newHarness(t, "")
	h.tt.Compose("zhong", 5)
	h.frame()
	h.want("")
	if h.e.compose != "zhong" {
		t.Fatalf("compose = %q", h.e.compose)
	}
	h.tt.Compose("", 0)
	h.tt.Type("中文")
	h.frame()
	h.want("中文")
}

func TestUndoTypingByWords(t *testing.T) {
	h := newHarness(t, "")
	for _, s := range []string{"a", "b", " ", "c", "d"} {
		h.typ(s)
	}
	h.want("ab cd")
	h.key(ui.Cmd, ui.KeyZ)
	h.want("ab ")
	h.key(ui.Cmd, ui.KeyZ)
	h.want("")
}

func TestClickBelowCodeAtEndMakesParagraph(t *testing.T) {
	h := newHarness(t, "```\ncode\n```")
	last := h.e.leaves[len(h.e.leaves)-1]
	h.tt.ClickAt(h.e.colX+20, last.y+last.h+60)
	h.frame()
	h.typ("after")
	h.want("```\ncode\n```\n\nafter")
}

func TestSelectionAcrossBlocksDeletes(t *testing.T) {
	h := newHarness(t, "# Head\n\npara one\n\n- item")
	a := strings.Index(h.e.Source(), "ad")
	z := strings.Index(h.e.Source(), "item") + 2
	h.e.Select(a, z)
	h.frame()
	h.key(0, ui.KeyBackspace)
	h.want("# Heem")
}

func TestHeadingEnterAtStartInsertsAbove(t *testing.T) {
	h := newHarness(t, "# Title")
	h.caretAt(2)
	h.key(0, ui.KeyEnter)
	h.typ("intro")
	h.want("intro\n\n# Title")
}

func TestEnterAtEndPutsCaretOnTheNewLine(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		off  int
	}{
		{"paragraph", "Hello world", 11},
		{"quote", "> Quoted line", 13},
		{"heading", "# A Heading", 11},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, tc.src)
			before, ok := h.e.caretRect(tc.off)
			if !ok {
				t.Fatal("no caret before Enter")
			}
			h.caretAt(tc.off)
			h.key(0, ui.KeyEnter)
			if h.e.phantom == nil {
				t.Fatal("no phantom after Enter at the end")
			}
			r, ok := h.e.caretRect(h.e.caret)
			if !ok {
				t.Fatal("no caret in the phantom")
			}
			if r.Y <= before.Y {
				t.Fatalf("caret at %.2f did not move below the line at %.2f", r.Y, before.Y)
			}
		})
	}
}

func TestArrowsLeavePhantom(t *testing.T) {
	h := newHarness(t, "Hello world")
	h.caretAt(11)
	h.key(0, ui.KeyEnter)
	h.key(0, ui.KeyLeft)
	if h.e.phantom != nil {
		t.Fatal("Left kept the phantom")
	}
	if h.e.caret != 11 {
		t.Fatalf("caret = %d, want 11", h.e.caret)
	}
	if r, _ := h.e.caretRect(h.e.caret); r.Y > h.e.leaves[0].y+h.e.leaves[0].h {
		t.Fatalf("caret at %.2f is not on the line above", r.Y)
	}

	// Right at the end of the document stays in the phantom.
	h = newHarness(t, "Hello world")
	h.caretAt(11)
	h.key(0, ui.KeyEnter)
	h.key(0, ui.KeyRight)
	if h.e.phantom == nil {
		t.Fatal("Right dropped the phantom at the end")
	}

	// Up enters the line above.
	h = newHarness(t, "Hello world")
	h.caretAt(11)
	h.key(0, ui.KeyEnter)
	h.key(0, ui.KeyUp)
	if h.e.phantom != nil {
		t.Fatal("Up kept the phantom")
	}
	if r, _ := h.e.caretRect(h.e.caret); r.Y > h.e.leaves[0].y+h.e.leaves[0].h {
		t.Fatalf("caret at %.2f is not on the line above", r.Y)
	}
}

func TestClickKeepsThePhantomOnlyOnItsLine(t *testing.T) {
	h := newHarness(t, "Hello world")
	h.caretAt(11)
	h.key(0, ui.KeyEnter)
	para := h.e.leaves[0]
	h.e.setCaretAtPoint(11, para.y+para.h/2, false)
	if h.e.phantom != nil {
		t.Fatal("a click at the line's end kept the phantom")
	}
	h = newHarness(t, "Hello world")
	h.caretAt(11)
	h.key(0, ui.KeyEnter)
	ph := h.e.leaves[1]
	h.e.setCaretAtPoint(11, ph.y+1, false)
	if h.e.phantom == nil {
		t.Fatal("a click on the phantom's line dropped it")
	}
}

func TestTaskEnterContinuesTasks(t *testing.T) {
	h := newHarness(t, "- [ ] one")
	h.caretAt(len("- [ ] one"))
	h.key(0, ui.KeyEnter)
	h.typ("two")
	h.want("- [ ] one\n- [ ] two")
}

func TestAutoPairAndSkip(t *testing.T) {
	h := newHarness(t, "")
	h.typ("(")
	h.want("()")
	h.typ("a")
	h.typ(")")
	h.want("(a)")
	if h.e.caret != 3 {
		t.Fatalf("caret = %d after typing the closer", h.e.caret)
	}
}

func TestPipeInCellIsEscaped(t *testing.T) {
	h := newHarness(t, "| a | b |\n| --- | --- |\n| 1 | 2 |")
	h.caretAt(strings.Index(h.e.Source(), "1") + 1)
	h.typ("|")
	if !strings.Contains(h.e.Source(), `1\|`) {
		t.Fatalf("pipe not escaped: %q", h.e.Source())
	}
}

func TestReplaceAllKeepsUndo(t *testing.T) {
	h := newHarness(t, "one two")
	h.e.ReplaceAll("one 2 two")
	h.frame()
	h.want("one 2 two")
	h.e.Undo()
	h.frame()
	h.want("one two")
}
