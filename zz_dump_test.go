package livemark

import (
	"encoding/json"
	"fmt"
	"html"
	"image/png"
	"os"
	"regexp"
	"strings"
	"testing"
	"unicode"

	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/livemark/theme"
)

var kindName = map[leafKind]string{kPara: "para", kHeading: "heading", kCode: "code", kMath: "math", kHTML: "html", kRule: "rule", kTable: "table", kRaw: "raw", kPhantom: "phantom"}

func styleTags(e *Editor, sp ui.Span) string {
	var t []string
	root := e.res.Root()
	accent := uiColor(root.Child(theme.Element{Type: "link"}).Style().Color)
	marker := uiColor(root.Child(theme.Element{Type: "syntax"}).Style().Color)
	muted := uiColor(root.Child(theme.Element{Type: "quote"}).Style().Color)
	markBg := uiColor(root.Child(theme.Element{Type: "mark"}).Style().TextBackground)
	if sp.Weight >= 600 {
		t = append(t, "b")
	}
	if sp.Italic {
		t = append(t, "i")
	}
	if sp.Strikethrough {
		t = append(t, "s")
	}
	if sp.Underline {
		t = append(t, "u")
	}
	if sp.Font == "monospace" {
		t = append(t, "code")
	}
	if sp.Color == accent {
		t = append(t, "link")
	}
	if sp.Color == marker {
		t = append(t, "mk")
	}
	if sp.Color == muted {
		t = append(t, "muted")
	}
	if sp.Background == markBg {
		t = append(t, "mark")
	}
	if sp.BaselineShift > 0 {
		t = append(t, "sup")
	}
	if sp.BaselineShift < 0 {
		t = append(t, "sub")
	}
	return strings.Join(t, ",")
}

func dispString(e *Editor, d display) string {
	var sb strings.Builder
	for _, p := range d.pieces {
		txt := strings.ReplaceAll(p.text, "\n", "⏎")
		if p.obj != nil {
			txt = "[OBJ]"
		}
		tags := styleTags(e, p.span)
		k := ""
		switch p.kind {
		case pAtom:
			k = "@"
		case pDeco:
			k = "~"
		}
		if tags != "" || k != "" {
			sb.WriteString("«" + k + tags + ":" + txt + "»")
		} else {
			sb.WriteString(txt)
		}
	}
	return sb.String()
}

func dumpEditor(e *Editor) string {
	var sb strings.Builder
	for _, l := range e.leaves {
		var cs []string
		for _, c := range l.conts {
			switch c.kind {
			case cQuote:
				cs = append(cs, "Q")
			case cItem:
				s := "I"
				if c.ordered {
					s += fmt.Sprintf("%d%c", c.num, c.marker)
				} else {
					s += string(c.marker)
				}
				if c.task != 0 {
					s += fmt.Sprintf("[t%d]", c.task)
				}
				if !c.tight {
					s += "L"
				}
				cs = append(cs, s)
			case cFootnote:
				cs = append(cs, "F:"+c.label)
			}
		}
		fmt.Fprintf(&sb, "%-8s %v first=%v [%d,%d) ", kindName[l.kind], cs, l.firstOf, l.start, l.end)
		if l.kind == kHeading {
			fmt.Fprintf(&sb, "h%d ", l.level)
		}
		if l.pending {
			sb.WriteString("PENDING ")
		}
		if l.kind == kCode {
			fmt.Fprintf(&sb, "lang=%q fenced=%v ", l.lang, l.fenced)
		}
		if l.kind == kTable {
			sb.WriteString("\n")
			for r, row := range l.tab.rows {
				for c, cell := range row {
					d := ""
					if cell.view != nil {
						d = dispString(e, cell.view.disp)
					}
					fmt.Fprintf(&sb, "    r%dc%d [%d,%d) %q\n", r, c, cell.start, cell.end, d)
				}
			}
			continue
		}
		if l.view != nil {
			if l.kind == kCode || l.kind == kMath || l.kind == kHTML || l.kind == kRaw {
				var ls []string
				for _, ln := range l.lines {
					ls = append(ls, ln.text)
				}
				fmt.Fprintf(&sb, "lines=%q", ls)
				if l.view.block != nil {
					sb.WriteString(" +BLOCKOBJ")
				}
			} else {
				fmt.Fprintf(&sb, "%s", dispString(e, l.view.disp))
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func newDumpEditor(src string, focus bool) (*Editor, *ui.Tester) {
	e := New(src, testOptions())
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			e.View(c).Grow(1)
		})
	}, 1000, 700)
	tt.Frame()
	tt.Frame()
	if focus {
		e.Focus()
		tt.Frame()
		tt.Frame()
	}
	return e, tt
}

func TestZZDump(t *testing.T) {
	path := os.Getenv("MDDUMP")
	if path == "" {
		t.Skip()
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range strings.Split(string(b), "\n=====\n") {
		e, _ := newDumpEditor(c, false)
		fmt.Printf("----- %q\n%s", c, dumpEditor(e))
	}
}

type zzSpec struct {
	Markdown string `json:"markdown"`
	HTML     string `json:"html"`
	Example  int    `json:"example"`
	Section  string `json:"section"`
}

func zzEditorText(e *Editor) (string, bool) {
	var sb strings.Builder
	raw := false
	var leafText func(l *leaf)
	leafText = func(l *leaf) {
		switch l.kind {
		case kCode, kMath, kHTML:
			for _, ln := range l.lines {
				sb.WriteString(ln.text)
				sb.WriteString("\n")
			}
		case kRaw:
			raw = true
		case kTable:
			for _, row := range l.tab.rows {
				for _, cell := range row {
					if cell.view != nil {
						for _, p := range cell.view.disp.pieces {
							if p.obj == nil {
								sb.WriteString(p.text)
							}
						}
					}
				}
			}
		case kRule:
		default:
			if l.view != nil {
				for _, p := range l.view.disp.pieces {
					if p.obj == nil {
						sb.WriteString(p.text)
					} else if p.obj != nil && p.obj.alt != "" {
					}
				}
			}
		}
	}
	for _, l := range e.leaves {
		leafText(l)
	}
	return sb.String(), raw
}

var zzTag = regexp.MustCompile(`<[^>]*>`)

func zzNorm(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if unicode.IsSpace(r) {
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

func TestZZSpec(t *testing.T) {
	path := os.Getenv("MDSPEC")
	if path == "" {
		t.Skip()
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var ex []zzSpec
	if err := json.Unmarshal(b, &ex); err != nil {
		t.Fatal(err)
	}
	only := os.Getenv("MDSECTION")
	bad := 0
	for _, x := range ex {
		if only != "" && !strings.Contains(x.Section, only) {
			continue
		}
		var e *Editor
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("PANIC #%d (%s) %q: %v\n", x.Example, x.Section, x.Markdown, r)
					e = nil
				}
			}()
			e, _ = newDumpEditor(x.Markdown, false)
		}()
		if e == nil {
			bad++
			continue
		}
		got, raw := zzEditorText(e)
		want := html.UnescapeString(zzTag.ReplaceAllString(x.HTML, ""))
		g, w := zzNorm(got), zzNorm(want)
		if g != w {
			// Raw HTML shows as written in the editor.
			if strings.Contains(x.HTML, "<") && zzNorm(html.UnescapeString(zzTag.ReplaceAllString(got, ""))) == w {
				continue
			}
			bad++
			fmt.Printf("#%d (%s) raw=%v\nmd   %q\nhtml %q\ngot  %q\nwant %q\n%s\n", x.Example, x.Section, raw, x.Markdown, x.HTML, g, w, dumpEditor(e))
		}
	}
	fmt.Println("mismatches:", bad)
}

// TestZZReveal dumps the leaf holding each caret offset, focused, with the
// caret there: what shows of the syntax around it.
func TestZZReveal(t *testing.T) {
	path := os.Getenv("MDREVEAL")
	if path == "" {
		t.Skip()
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range strings.Split(string(b), "\n=====\n") {
		e, tt := newDumpEditor(c, true)
		fmt.Printf("===== %q\n", c)
		prev := ""
		for off := 0; off <= len(c); off++ {
			e.dropPhantom()
			e.anchor, e.caret = off, off
			tt.Frame()
			u := e.unitAt(off)
			if u == nil || u.view == nil {
				fmt.Printf("  %3d: (no unit)\n", off)
				continue
			}
			norm := e.normalize(off, 1)
			s := dispString(e, u.view.disp)
			line := fmt.Sprintf("%s", s)
			if line != prev || norm != off {
				fmt.Printf("  %3d (norm %3d): %s\n", off, norm, line)
			}
			prev = line
		}
	}
}

// TestZZShot renders the file at MDSHOT to MDSHOTOUT, unfocused.
func TestZZShot(t *testing.T) {
	path := os.Getenv("MDSHOT")
	if path == "" {
		t.Skip()
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	e := New(string(b), testOptions())
	h := 2400
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			e.View(c).Grow(1)
		})
	}, 900, h)
	tt.Frame()
	tt.Frame()
	tt.Frame()
	f, err := os.Create(os.Getenv("MDSHOTOUT"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
	fmt.Print(dumpEditor(e))
}

// TestZZCrash puts the caret at every offset of every spec example, and
// moves it every way, looking for panics.
func TestZZCrash(t *testing.T) {
	path := os.Getenv("MDCRASH")
	if path == "" {
		t.Skip()
	}
	var srcs []string
	for _, p := range strings.Split(path, ",") {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(p, ".json") {
			var ex []zzSpec
			if err := json.Unmarshal(b, &ex); err != nil {
				t.Fatal(err)
			}
			for _, x := range ex {
				srcs = append(srcs, x.Markdown)
			}
		} else {
			srcs = append(srcs, strings.Split(string(b), "\n=====\n")...)
		}
	}
	n := 0
	for _, src := range srcs {
		e, tt := newDumpEditor(src, true)
		for off := 0; off <= len(src); off++ {
			for _, act := range []string{"put", "left", "right", "up", "down", "wordl", "wordr", "home", "end"} {
				func() {
					defer func() {
						if r := recover(); r != nil {
							n++
							if n < 40 {
								fmt.Printf("PANIC %s at %d in %q: %v\n", act, off, src, r)
							}
							e, tt = newDumpEditor(src, true)
						}
					}()
					e.dropPhantom()
					e.anchor, e.caret = off, off
					tt.Frame()
					switch act {
					case "left":
						e.setCaret(e.step(e.caret, -1, false), false)
					case "right":
						e.setCaret(e.step(e.caret, 1, false), false)
					case "wordl":
						e.setCaret(e.step(e.caret, -1, true), false)
					case "wordr":
						e.setCaret(e.step(e.caret, 1, true), false)
					case "up":
						e.vertical(-1, false)
					case "down":
						e.vertical(1, false)
					case "home", "end":
						s, z := e.lineEdges(e.caret)
						_, _ = s, z
					}
					e.caretRect(e.caret)
					e.normalize(e.caret, 1)
					e.normalize(e.caret, -1)
					tt.Frame()
				}()
			}
		}
	}
	fmt.Println("panics:", n)
}

// TestZZEditCrash edits at every offset of every example, looking for
// panics and for carets out of the source.
func TestZZEditCrash(t *testing.T) {
	path := os.Getenv("MDCRASH")
	if path == "" {
		t.Skip()
	}
	var srcs []string
	for _, p := range strings.Split(path, ",") {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(p, ".json") {
			var ex []zzSpec
			if err := json.Unmarshal(b, &ex); err != nil {
				t.Fatal(err)
			}
			for _, x := range ex {
				srcs = append(srcs, x.Markdown)
			}
		} else {
			srcs = append(srcs, strings.Split(string(b), "\n=====\n")...)
		}
	}
	n := 0
	acts := []string{"x", "中", "enter", "senter", "bs", "del", "tab", "stab", "bold", "quote", "list", "h2", "bsword", "paste", "undo"}
	for _, src := range srcs {
		for off := 0; off <= len(src); off++ {
			for _, act := range acts {
				func() {
					var e *Editor
					var tt *ui.Tester
					defer func() {
						if r := recover(); r != nil {
							n++
							if n < 60 {
								fmt.Printf("PANIC %s at %d in %q: %v\n", act, off, src, r)
							}
						}
					}()
					e, tt = newDumpEditor(src, true)
					e.anchor, e.caret = off, off
					e.caret = e.normalize(off, 1)
					e.anchor = e.caret
					tt.Frame()
					switch act {
					case "x", "中":
						e.typeText(act)
					case "enter":
						e.enter(false)
					case "senter":
						e.enter(true)
					case "bs":
						e.backspace(false)
					case "bsword":
						e.backspace(true)
					case "del":
						e.deleteForward(false)
					case "tab":
						e.tab(false)
					case "stab":
						e.tab(true)
					case "bold":
						e.ToggleBold()
					case "quote":
						e.ToggleQuote()
					case "list":
						e.ToggleList("-")
					case "h2":
						e.SetHeading(2)
					case "paste":
						e.insertText("a\nb")
					case "undo":
						e.typeText("y")
						e.Undo()
						if e.Source() != src {
							n++
							fmt.Printf("UNDO MISMATCH at %d in %q: %q\n", off, src, e.Source())
						}
					}
					tt.Frame()
					if e.caret < 0 || e.caret > len(e.src) || e.anchor < 0 || e.anchor > len(e.src) {
						n++
						fmt.Printf("CARET OUT %s at %d in %q: caret %d anchor %d len %d\n", act, off, src, e.caret, e.anchor, len(e.src))
					}
					e.caretRect(e.caret)
					tt.Frame()
				}()
			}
		}
	}
	fmt.Println("problems:", n)
}
