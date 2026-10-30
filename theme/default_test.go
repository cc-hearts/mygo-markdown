package theme_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/livemark/theme"
)

// env returns the environment the editor gives a theme in a window theme:
// its colors as --ui-*, each as #rrggbbaa so that they come through to the
// byte.
func env(t *ui.Theme) theme.Env {
	h := func(c ui.Color) string { return fmt.Sprintf("#%02x%02x%02x%02x", c.R, c.G, c.B, c.A) }
	return theme.Env{Dark: t.Dark, Vars: map[string]string{
		"--ui-text": h(t.Text), "--ui-text-muted": h(t.TextMuted), "--ui-accent": h(t.Accent),
		"--ui-border": h(t.Border), "--ui-surface": h(t.Surface), "--ui-background": h(t.Background),
		"--ui-selection": h(t.Selection), "--ui-accent-text": h(t.AccentText),
	}}
}

func tc(c ui.Color) theme.Color { return theme.Color{R: c.R, G: c.G, B: c.B, A: c.A} }

// look is the look the editor drew before themes, as layout.go's setLook
// and paint.go resolved it: what default.css must come to.
type look struct {
	text, muted, accent, marker, border, selection     theme.Color
	codeBg, panel, codeText, quoteBar, markBg, grid    theme.Color
	ink, label, placeholder, box, search, searchActive theme.Color
}

func lookOf(t *ui.Theme) look {
	lk := look{
		text: tc(t.Text), muted: tc(t.TextMuted), accent: tc(t.Accent), marker: tc(t.TextMuted.Alpha(0.7)),
		border: tc(t.Border), selection: tc(t.Selection),
		label: tc(t.TextMuted.Alpha(0.75)), placeholder: tc(t.TextMuted.Alpha(0.6)), box: tc(t.TextMuted.Alpha(0.8)),
		search: tc(ui.RGBA(250, 204, 21, 0.35)), searchActive: tc(ui.RGBA(250, 150, 20, 0.55)),
	}
	if t.Dark {
		lk.codeBg, lk.panel, lk.codeText = tc(ui.RGBA(255, 255, 255, 0.08)), tc(ui.RGBA(255, 255, 255, 0.045)), tc(ui.Hex("#e4a3a3"))
		lk.quoteBar, lk.markBg, lk.grid = tc(ui.RGBA(255, 255, 255, 0.16)), tc(ui.RGBA(250, 204, 21, 0.32)), tc(ui.RGBA(255, 255, 255, 0.14))
		lk.ink = theme.Color{R: 255, G: 255, B: 255, A: 255}
	} else {
		lk.codeBg, lk.panel, lk.codeText = tc(ui.RGBA(0, 0, 0, 0.055)), tc(ui.Hex("#f6f7f9")), tc(ui.Hex("#c7254e"))
		lk.quoteBar, lk.markBg, lk.grid = tc(ui.RGBA(0, 0, 0, 0.12)), tc(ui.RGBA(250, 204, 21, 0.45)), tc(ui.RGBA(0, 0, 0, 0.12))
		lk.ink = theme.Color{A: 255}
	}
	return lk
}

// node returns the node of a path of elements, each written as a type, its
// classes and its states: "item.task.checked", "image:broken".
func node(r *theme.Resolver, path ...string) *theme.Node {
	n := r.Root()
	for _, p := range path {
		var e theme.Element
		for {
			i := strings.LastIndexByte(p, ':')
			if i < 0 {
				break
			}
			switch p[i+1:] {
			case "first-child":
				e.States |= theme.FirstChild
			case "revealed":
				e.States |= theme.Revealed
			case "broken":
				e.States |= theme.Broken
			}
			p = p[:i]
		}
		parts := strings.Split(p, ".")
		e.Type, e.Classes = parts[0], parts[1:]
		n = n.Child(e)
	}
	return n
}

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }

func TestDefaultParses(t *testing.T) {
	if _, diags := theme.Parse("default.css", theme.DefaultCSS()); len(diags) != 0 {
		t.Fatalf("default.css: %v", diags)
	}
}

// TestDefaultIsTheOldLook checks that default.css, in MyGo's light and dark
// themes, comes to the colors and the lengths the editor had written in.
func TestDefaultIsTheOldLook(t *testing.T) {
	for _, wt := range []*ui.Theme{ui.LightTheme(), ui.DarkTheme()} {
		name := "light"
		if wt.Dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			r := theme.New().Resolver(env(wt))
			lk := lookOf(wt)
			check := func(what string, got, want any) {
				t.Helper()
				if g, ok := got.(float32); ok {
					if !near(g, want.(float32)) {
						t.Errorf("%s: got %v, want %v", what, got, want)
					}
					return
				}
				if fmt.Sprint(got) != fmt.Sprint(want) {
					t.Errorf("%s: got %v, want %v", what, got, want)
				}
			}

			root := r.Root().Style()
			check("text", root.Color, lk.text)
			check("caret", root.CaretColor, lk.accent)
			check("size", root.FontSize, float32(16))
			check("line height", root.LineHeight, float32(1.75))
			check("column", root.MaxWidth, float32(820))
			check("top room", root.Padding.Top, float32(16*2.2))
			check("side room", root.Padding.Left, float32(40))
			check("font", strings.Join(root.FontFamily, ","), "system-ui")

			sizes := []float32{2.1, 1.65, 1.4, 1.2, 1, 0.875}
			weights := []int{700, 700, 650, 650, 650, 650}
			tops := []float32{1.4, 1.2, 1.0, 0.9, 0.8, 0.8}
			for i := range 6 {
				h := node(r, fmt.Sprintf("heading.h%d", i+1)).Style()
				check(fmt.Sprintf("h%d size", i+1), h.FontSize, 16*sizes[i])
				check(fmt.Sprintf("h%d weight", i+1), h.FontWeight, weights[i])
				check(fmt.Sprintf("h%d top", i+1), h.Margin.Top, 16*tops[i])
				check(fmt.Sprintf("h%d bottom", i+1), h.Margin.Bottom, float32(16*0.45))
				check(fmt.Sprintf("h%d line height", i+1), h.LineHeight, float32(1.35))
			}
			p := node(r, "paragraph").Style()
			check("paragraph margins", [2]float32{p.Margin.Top, p.Margin.Bottom}, [2]float32{16 * 0.7, 16 * 0.7})

			q := node(r, "quote").Style()
			check("quote indent", q.Padding.Left, float32(16*1.2))
			check("quote bar", q.Border[3], theme.Border{Width: 3, Color: lk.quoteBar})
			check("quote text", node(r, "quote", "paragraph").Style().Color, lk.muted)

			check("syntax", node(r, "paragraph", "syntax.strong").Style().Color, lk.marker)
			check("html tag", node(r, "paragraph", "html.inline").Style().Color, lk.marker)
			check("link", node(r, "paragraph", "link").Style().Color, lk.accent)
			check("bold", node(r, "paragraph", "strong").Style().FontWeight, 700)
			check("italic", node(r, "paragraph", "emphasis").Style().Italic, true)
			check("strike", node(r, "paragraph", "strike").Style().Decoration.LineThrough, true)
			check("underline", node(r, "paragraph", "underline").Style().Decoration.Underline, true)
			check("mark", node(r, "paragraph", "mark").Style().TextBackground, lk.markBg)
			check("bold in mark", node(r, "paragraph", "mark", "strong").Style().TextBackground, lk.markBg)
			check("syntax in mark", node(r, "paragraph", "mark", "syntax.strong").Style().TextBackground, theme.Color{})

			code := node(r, "paragraph", "code.inline").Style()
			check("code font", strings.Join(code.FontFamily, ","), "monospace")
			check("code size", code.FontSize, float32(16*0.875))
			check("code color", code.Color, lk.codeText)
			check("code background", code.TextBackground, lk.codeBg)
			check("code radius", code.TextRadius, float32(max(3, 16.0/5)))
			h1code := node(r, "heading.h1", "code.inline").Style()
			check("code radius in h1", h1code.TextRadius, float32(max(3, 16*2.1/5)))
			check("code in a link", node(r, "paragraph", "link", "code.inline").Style().Color, lk.accent)
			check("revealed math", node(r, "paragraph", "math.inline:revealed").Style().Color, lk.codeText)
			kbd := node(r, "paragraph", "kbd").Style()
			check("kbd", [3]any{kbd.FontSize, kbd.TextBackground, kbd.TextRadius}, [3]any{float32(16 * 0.85), lk.codeBg, float32(3)})
			sup := node(r, "paragraph", "sup").Style()
			check("sup", [2]float32{sup.FontSize, sup.Shift}, [2]float32{12, 16 * 0.35})
			sub := node(r, "paragraph", "sub").Style()
			check("sub", [2]float32{sub.FontSize, sub.Shift}, [2]float32{12, -16 * 0.2})
			fr := node(r, "paragraph", "footnote-ref").Style()
			check("footnote reference", [2]any{fr.FontSize, fr.Color}, [2]any{float32(12), lk.accent})

			cb := node(r, "code.block").Style()
			check("code block", [5]any{cb.Background, cb.Radius, cb.Padding.Top, cb.Padding.Left, cb.Margin.Top},
				[5]any{lk.panel, float32(6), float32(16 * 0.75), float32(16), float32(16 * 0.8)})
			check("code block font", [2]float32{cb.FontSize, cb.LineHeight}, [2]float32{14, 1.6})
			info := node(r, "code.block").Pseudo("info", 0)
			check("code label", [2]any{info.Color, info.FontSize}, [2]any{lk.label, float32(16 * 0.7)})
			check("fence", node(r, "code.block", "syntax.fence").Style().Color, lk.marker)
			check("info string", node(r, "code.block", "syntax.info").Style().Color, lk.muted)

			check("math ink", node(r, "paragraph", "math.inline").Style().Color, lk.ink)
			check("block math ink", node(r, "math.block").Style().Color, lk.ink)
			mb := node(r, "math.block:revealed").Style()
			check("math source", [4]any{mb.Background, mb.Color, mb.Padding.Left, mb.FontSize}, [4]any{lk.panel, lk.text, float32(16), float32(14)})
			fm := node(r, "front-matter").Style()
			check("front matter", [3]any{fm.Background, fm.Padding.Left, fm.Margin.Top}, [3]any{lk.panel, float32(16), float32(16 * 0.6)})
			check("html block", node(r, "html.block").Style().Color, lk.muted)
			check("link definition", node(r, "link-def").Style().Color, lk.muted)

			tb := node(r, "table").Style()
			check("grid", tb.Border, [4]theme.Border{{1, lk.grid}, {1, lk.grid}, {1, lk.grid}, {1, lk.grid}})
			check("header row", node(r, "table", "row.header").Style().Background, lk.panel)
			hc := node(r, "table", "row.header", "cell.header").Style()
			check("header cell", [3]any{hc.FontWeight, hc.Padding.Top, hc.Padding.Left}, [3]any{700, float32(16 * 0.45), float32(16 * 0.7)})

			rule := node(r, "rule").Style()
			check("rule", [3]any{rule.Height, rule.Border[0].Width, rule.Border[0].Color}, [3]any{float32(16 * 1.2), float32(1.5), lk.border})

			item := node(r, "list.bullet.tight", "item.task")
			check("item indent", item.Style().Padding.Left, float32(16*1.75))
			m := item.Pseudo("marker", 0)
			check("marker", [3]any{m.Color, m.Margin.Right, m.FontFeatures}, [3]any{lk.muted, float32(16 * 0.45), "tnum"})
			box := item.Pseudo("checkbox", 0)
			check("box", [4]any{box.Width, box.Border[0], box.Radius, box.Background}, [4]any{float32(16 * 0.95), theme.Border{Width: 1.3, Color: lk.box}, float32(3.5), theme.Color{}})
			checked := node(r, "list.bullet.tight", "item.task.checked")
			cbox := checked.Pseudo("checkbox", 0)
			check("checked box", [3]any{cbox.Background, cbox.Border[0].Width, cbox.Color}, [3]any{lk.accent, float32(0), theme.Color{R: 255, G: 255, B: 255, A: 255}})
			done := node(r, "list.bullet.tight", "item.task.checked", "paragraph:first-child").Style()
			check("done", [2]any{done.Color, done.Decoration.LineThrough}, [2]any{lk.muted, true})
			check("syntax of done", node(r, "list.bullet.tight", "item.task.checked", "paragraph:first-child", "syntax.code").Style().Decoration.LineThrough, false)
			tight := node(r, "list.bullet.tight", "item", "paragraph").Style()
			check("tight item", [2]float32{tight.Margin.Top, tight.Margin.Bottom}, [2]float32{16 * 0.2, 0})
			loose := node(r, "list.bullet.loose", "item", "paragraph").Style()
			check("loose item", [2]float32{loose.Margin.Top, loose.Margin.Bottom}, [2]float32{16 * 0.7, 16 * 0.7})

			img := node(r, "paragraph", "image:broken").Style()
			check("broken image", [5]any{img.Background, img.Border[0], img.Radius, img.FontSize, img.Color},
				[5]any{lk.panel, theme.Border{Width: 1, Color: lk.border}, float32(6), float32(16 * 0.8), lk.muted})
			footnote := node(r, "footnote")
			check("footnote indent", footnote.Style().Padding.Left, float32(16*1.75))
			fl := footnote.Pseudo("label", 0)
			check("footnote label", [2]any{fl.FontSize, fl.Color}, [2]any{float32(12), lk.accent})

			check("selection", r.Root().Pseudo("selection", 0).Background, lk.selection)
			check("search", r.Root().Pseudo("search-text", 0).Background, lk.search)
			check("current match", r.Root().Pseudo("search-text", theme.Current).Background, lk.searchActive)
			check("placeholder", r.Root().Pseudo("placeholder", 0).Color, lk.placeholder)

			for _, v := range []struct {
				name string
				want ui.Color
			}{{"--diagram-muted", wt.TextMuted}, {"--diagram-line", wt.Border}, {"--diagram-fill", wt.Surface}, {"--diagram-accent", wt.Accent}} {
				got, ok := node(r, "code.block").Style().VarColor(v.name)
				check(v.name, [2]any{got, ok}, [2]any{tc(v.want), true})
			}
		})
	}
}
