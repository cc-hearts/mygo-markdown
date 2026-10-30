package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"github.com/CarlOpenLab/livemark"
	"github.com/CarlOpenLab/livemark/render"
	mathrender "github.com/CarlOpenLab/livemark/render/math"
	mermaidrender "github.com/CarlOpenLab/livemark/render/mermaid"
	"github.com/CarlOpenLab/livemark/syntax"
	"github.com/CarlOpenLab/livemark/theme"
)

// debounce is how long typing rests before the theme is parsed again.
const debounce = 150 * time.Millisecond

// The window's themes, made once: the editor compares pointers to know
// whether the frame changed appearance.
var (
	lightTheme = ui.LightTheme()
	darkTheme  = ui.DarkTheme()
)

// app is the playground's state, kept across frames: the CSS being
// written, the theme it made, the document it styles, and what the
// inspector and the syntax tree show.
type app struct {
	win atomic.Pointer[mygo.Window]

	// The CSS editor and the theme parsed from it.
	css      *editor.State
	cssVer   uint64
	editAt   time.Time
	parsed   uint64
	diags    []theme.Diagnostic
	theme    *theme.Theme
	preset   int
	focusCSS bool

	// The document, and the options that draw it.
	doc     *livemark.Editor
	docOpts livemark.Options

	// The inspector's path at pathAt, of pathSrc.
	pathEls []theme.Element
	pathAt  int
	pathSrc string

	// The syntax tree panel.
	showTree   bool
	tree       *syntax.Tree
	treeDump   string
	treeEditor *editor.State

	split   float32
	dark    bool
	darkSet bool

	// file and fileMod are the theme file open on desktop, which the
	// watcher reloads when it changes elsewhere.
	mu      sync.Mutex
	file    string
	fileMod time.Time
}

// newApp returns the playground at its first preset.
func newApp() *app {
	a := &app{split: 36}
	a.css = &editor.State{
		Buffer:     editor.NewBuffer(presets[0].css),
		SoftWrap:   true,
		LineHeight: 1.5,
		Label:      "主题 CSS",
	}
	a.css.Buffer.SetLanguage(editor.LanguageFor("theme.css"))
	a.cssVer = a.css.Buffer.Version()

	a.docOpts = livemark.Options{
		Math:        mathrender.New(),
		Diagrams:    []render.DiagramRenderer{mermaidrender.New()},
		Placeholder: "写点什么…",
		Label:       "示例文档",
		Invalidate:  a.invalidate,
	}
	a.doc = livemark.New(sampleDoc, a.docOpts)
	a.parse()
	return a
}

// window returns the window once it is made, nil before.
func (a *app) window() *mygo.Window { return a.win.Load() }

// invalidate asks the window for a frame, from any goroutine.
func (a *app) invalidate() {
	if w := a.window(); w != nil {
		w.Update(func() {})
	}
}

// view builds the playground: the top bar, the CSS editor beside the
// document, the syntax tree, the diagnostics and the inspector.
func (a *app) view(c *ui.Context) {
	if !a.darkSet {
		a.dark, a.darkSet = c.Theme().Dark, true
	}
	if c.Theme().Dark != a.dark {
		if a.dark {
			c.SetTheme(darkTheme)
		} else {
			c.SetTheme(lightTheme)
		}
	}
	t := c.Theme()
	a.tick(c)

	ui.Column(c).Fill().Background(t.Background).Children(func() {
		a.topBar(c, t)
		ui.SplitPercent(c, &a.split, func() { a.cssColumn(c, t) }, func() { a.docColumn(c, t) }).Grow(1).Shrink(1)
		if a.showTree {
			a.treePanel(c, t)
		}
		a.diagBar(c, t)
		a.statusBar(c, t)
	})
}

// topBar holds the title, the file, and the switches.
func (a *app) topBar(c *ui.Context, t *ui.Theme) {
	ui.Row(c).FillWidth().Height(44).Padding(14, 12).Gap(10).AlignItems(ui.Center).
		Background(t.Surface).Children(func() {
		ui.Text(c, "livemark").Bold().FontSize(14)
		ui.Text(c, "主题 Playground").FontSize(12).TextColor(t.TextMuted)
		ui.Text(c, a.fileName()).FontSize(11).TextColor(t.TextMuted)
		ui.Spacer(c)
		if desktop {
			if ui.Button(c, "打开…").FontSize(11).Clicked() {
				a.openTheme()
			}
			if ui.Button(c, "保存…").FontSize(11).Clicked() {
				a.saveTheme()
			}
		}
		ui.Toggle(c, &a.showTree, "语法树").FontSize(11)
		ui.Toggle(c, &a.dark, "深色").FontSize(11)
	})
	divider(c, t)
}

// cssColumn is the preset choices above the CSS editor.
func (a *app) cssColumn(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Background(t.Surface).Children(func() {
		ui.Row(c).FillWidth().Padding(10, 8).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "预设").FontSize(11).TextColor(t.TextMuted)
			for i, p := range presets {
				b := ui.Button(c.Key(p.name), p.name).FontSize(11)
				if i == a.preset {
					b.Background(t.Accent).TextColor(t.AccentText)
				}
				if b.Clicked() {
					a.loadPreset(i)
				}
			}
			ui.Spacer(c)
			if a.preset >= 0 && a.css.Buffer.String() != presets[a.preset].css {
				ui.Text(c, "已修改").FontSize(11).TextColor(t.TextMuted)
			}
		})
		divider(c, t)
		ed := editor.View(c, a.css).Fill()
		if a.focusCSS {
			ed.Focus()
			a.focusCSS = false
		}
	})
}

// docColumn is the document the theme styles.
func (a *app) docColumn(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		a.doc.View(c).Grow(1)
	})
}

// treePanel shows the public syntax tree of the document.
func (a *app) treePanel(c *ui.Context, t *ui.Theme) {
	divider(c, t)
	ui.Column(c).FillWidth().Height(180).Shrink(0).Background(t.Surface).Children(func() {
		ui.Row(c).FillWidth().Height(24).Padding(10, 6).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, "语法树").FontSize(11).Bold()
			ui.Text(c, fmt.Sprintf("%d 个节点", strings.Count(a.treeDump, "\n"))).FontSize(11).TextColor(t.TextMuted)
		})
		editor.View(c, a.treeState()).Fill()
	})
}

// treeState is a read-only editor of the syntax tree's dump.
func (a *app) treeState() *editor.State {
	if a.treeEditor == nil {
		a.treeEditor = &editor.State{
			Buffer:      editor.NewBuffer(""),
			ReadOnly:    true,
			LineNumbers: editor.LineNumbersOff,
			SoftWrap:    false,
			Label:       "语法树",
		}
	}
	a.syncTree()
	return a.treeEditor
}

// syncTree parses the document's syntax tree again when it changed.
func (a *app) syncTree() {
	if a.doc == nil {
		return
	}
	tree := a.doc.Syntax()
	if tree == a.tree {
		return
	}
	a.tree = tree
	a.treeDump = dumpTree(tree)
	if a.treeEditor != nil {
		a.treeEditor.Buffer.Set(a.treeDump)
	}
}

// diagBar lists the theme's errors and warnings; a click goes to the line
// in the CSS editor.
func (a *app) diagBar(c *ui.Context, t *ui.Theme) {
	divider(c, t)
	ui.Column(c).FillWidth().Background(t.Surface).Children(func() {
		ui.Row(c).FillWidth().Height(26).Padding(10, 4).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, "诊断").FontSize(11).Bold()
			if len(a.diags) == 0 {
				ui.Text(c, "没有错误或警告").FontSize(11).TextColor(t.TextMuted)
				return
			}
			ui.Text(c, fmt.Sprintf("%d 条", len(a.diags))).FontSize(11).TextColor(t.TextMuted)
		})
		if len(a.diags) == 0 {
			return
		}
		h := min(float32(len(a.diags))*24, 120)
		ui.Scroll(c).FillWidth().Height(h).Children(func() {
			ui.Column(c).FillWidth().Padding(6, 10).Gap(1).Children(func() {
				for i, d := range a.diags {
					col := t.Warning
					if d.Severity == theme.Error {
						col = t.Danger
					}
					b := ui.Button(c.Key(fmt.Sprintf("diag%d", i)),
						fmt.Sprintf("%d:%d  %s", d.Line, d.Col, d.Message)).
						FillWidth().Height(23).FontSize(11).TextColor(col)
					if b.Clicked() {
						a.jumpTo(d)
					}
				}
			})
		})
	})
}

// statusBar shows the theme path at the caret, and inserts it as a
// selector.
func (a *app) statusBar(c *ui.Context, t *ui.Theme) {
	divider(c, t)
	els := a.inspect()
	path := joinPath(els)
	ui.Row(c).FillWidth().Height(30).Padding(12, 6).Gap(8).AlignItems(ui.Center).
		Background(t.Surface).Children(func() {
		ui.Text(c, "节点").FontSize(11).TextColor(t.TextMuted)
		ui.Text(c, path).Font("monospace").FontSize(11).SingleLine().Ellipsis("…").Grow(1)
		sel := ""
		if len(els) > 1 {
			sel = els[len(els)-1].String()
		}
		b := ui.Button(c, "插入选择器").FontSize(11).Disabled(sel == "")
		if b.Clicked() && sel != "" {
			a.insertSelector(sel)
		}
	})
}

// divider draws a hairline across the column.
func divider(c *ui.Context, t *ui.Theme) {
	ui.Box(c).FillWidth().Height(1).Background(t.Border)
}

// tick parses the CSS again once typing has rested, and schedules the
// frame that does, while it is pending.
func (a *app) tick(c *ui.Context) {
	ver := a.css.Buffer.Version()
	now := c.Now()
	if ver != a.cssVer {
		a.cssVer, a.editAt = ver, now
	}
	if ver == a.parsed {
		return
	}
	if rest := debounce - now.Sub(a.editAt); rest > 0 {
		c.After(rest)
		return
	}
	a.parse()
}

// parse makes the theme of the CSS editor's text, and gives it to the
// document.
func (a *app) parse() {
	css := a.css.Buffer.String()
	ver := a.css.Buffer.Version()
	sheet, diags := theme.Parse("playground", css)
	a.diags = diags
	a.theme = theme.New(sheet)
	a.parsed = ver
	a.css.SetDiagnostics(ver, editorDiagnostics(css, diags))

	a.docOpts.Theme = a.theme
	a.doc.SetOptions(a.docOpts)
}

// loadPreset puts a preset's CSS in the editor, as one edit that undo
// takes back.
func (a *app) loadPreset(i int) {
	if i < 0 || i >= len(presets) {
		return
	}
	a.preset = i
	a.setCSS(presets[i].css)
	a.parse()
	a.invalidate()
}

// setCSS replaces the CSS editor's text, keeping it undoable.
func (a *app) setCSS(css string) {
	b := a.css.Buffer
	if b.String() == css {
		return
	}
	a.css.Select(0, b.Len())
	a.css.Insert(css)
	a.css.GoTo(editor.Pos{})
	a.cssVer = b.Version()
}

// inspect returns the theme path at the caret, cached between frames.
func (a *app) inspect() []theme.Element {
	src, caret := a.doc.Source(), a.doc.Caret()
	if a.pathEls == nil || a.pathSrc != src || a.pathAt != caret {
		a.pathEls = a.doc.StylePath(caret)
		a.pathSrc, a.pathAt = src, caret
	}
	return a.pathEls
}

// joinPath writes elements as the inspector shows them.
func joinPath(els []theme.Element) string {
	parts := make([]string, len(els))
	for i, el := range els {
		parts[i] = el.String()
	}
	return strings.Join(parts, " › ")
}

// insertSelector adds a rule for sel at the caret of the CSS editor, and
// puts the caret where its declarations go.
func (a *app) insertSelector(sel string) {
	off := a.css.Cursor()
	rule := "\n" + sel + " {\n  \n}\n"
	a.css.Insert(rule)
	inner := off + len("\n"+sel+" {\n  ")
	a.css.GoTo(a.css.Buffer.Pos(inner))
	a.focusCSS = true
}

// jumpTo scrolls the CSS editor to a diagnostic.
func (a *app) jumpTo(d theme.Diagnostic) {
	off := offsetAt(a.css.Buffer.String(), d.Line, d.Col)
	a.css.GoTo(a.css.Buffer.Pos(off))
	a.focusCSS = true
}

// editorDiagnostics turns a theme's diagnostics into the CSS editor's,
// at byte offsets of css.
func editorDiagnostics(css string, ds []theme.Diagnostic) []editor.Diagnostic {
	out := make([]editor.Diagnostic, 0, len(ds))
	for _, d := range ds {
		start := offsetAt(css, d.Line, d.Col)
		end := start
		if end < len(css) {
			_, size := utf8.DecodeRuneInString(css[end:])
			end += size
		}
		sev := editor.SeverityWarning
		if d.Severity == theme.Error {
			sev = editor.SeverityError
		}
		out = append(out, editor.Diagnostic{Start: start, End: end, Severity: sev, Message: d.Message, Source: "theme"})
	}
	return out
}

// offsetAt returns the byte of a line and column, both from 1 and the
// column counting characters.
func offsetAt(src string, line, col int) int {
	line, col = max(line, 1), max(col, 1)
	off := 0
	for n := 1; n < line; n++ {
		i := strings.IndexByte(src[off:], '\n')
		if i < 0 {
			return len(src)
		}
		off += i + 1
	}
	for c := 1; c < col && off < len(src); c++ {
		_, size := utf8.DecodeRuneInString(src[off:])
		off += size
	}
	return min(off, len(src))
}

// fileName is the name of the open theme file, or a placeholder.
func (a *app) fileName() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.file == "" {
		return "未命名主题"
	}
	return filepath.Base(a.file)
}
