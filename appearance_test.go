package livemark

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The appearance baseline records how the editor draws a corpus of
// documents before the theme rewrite: the box of every leaf, the style of
// every run of text, and the pixels of the frame. The rewrite must draw the
// same from default.css, so these goldens are the safety net.
//
// Record the goldens with:
//
//	LIVEMARK_GOLDEN=1 go test -run TestAppearanceBaseline
//
// and review the changes before committing them: every difference must be
// intended.
const goldenEnv = "LIVEMARK_GOLDEN"

type appearanceCaret struct {
	name string
	// marker is a string that occurs once in the document; the caret goes
	// at its first byte plus delta.
	marker string
	delta  int
}

var appearanceDocs = []struct {
	name   string
	src    string
	carets []appearanceCaret
}{
	{
		name: "prose",
		src: "# Heading one\n" +
			"\n" +
			"A paragraph of **bold**, *italic*, ~~struck~~, `inline code`, $x^2 + y^2$,\n" +
			"a [link](https://example.com \"title\"), an autolink <https://example.com>, a\n" +
			"footnote[^note], an escape \\* and an entity &amp;, plus <u>underline</u>,\n" +
			"<mark>mark</mark>, <kbd>Ctrl</kbd>, H<sub>2</sub>O and x<sup>2</sup>.\n" +
			"\n" +
			"## Heading two\n",
		carets: []appearanceCaret{
			{"bold", "**bold**", 2},
			{"link", "[link]", 1},
			{"kbd", "Ctrl", 2},
		},
	},
	{
		name: "structure",
		src: "> A quote with **bold**.\n" +
			">\n" +
			"> > Nested *quote*.\n" +
			"\n" +
			"- bullet one\n" +
			"- bullet two\n" +
			"  - nested\n" +
			"- [x] task done\n" +
			"- [ ] task open\n" +
			"\n" +
			"3. ordered one\n" +
			"4. ordered two\n" +
			"\n" +
			"Setext\n" +
			"======\n",
		carets: []appearanceCaret{
			{"quote", "**bold**", 2},
			{"task", "[x] task done", 1},
			{"setext", "Setext", 2},
		},
	},
	{
		name: "blocks",
		src: "~~~go\n" +
			"package main\n" +
			"\n" +
			"func main() { fmt.Println(\"hi\") }\n" +
			"~~~\n" +
			"\n" +
			"~~~mermaid\n" +
			"graph TD\n" +
			"  A --> B\n" +
			"~~~\n" +
			"\n" +
			"    indented code\n" +
			"    two lines\n" +
			"\n" +
			"$$\n" +
			"\\frac{a}{b} = c\n" +
			"$$\n" +
			"\n" +
			"---\n" +
			"\n" +
			"[^note]: The note's text.\n" +
			"\n" +
			"A reference[^note] to the note.\n" +
			"\n" +
			"[ref]: https://example.com \"Title\"\n",
		carets: []appearanceCaret{
			{"code", "package main", 3},
			{"math", "\\frac{a}{b}", 2},
			{"footnote", "The note's text.", 4},
			{"ref", "reference[^note]", 9},
		},
	},
	{
		name: "table",
		src: "| Left | Center | Right |\n" +
			"|:-----|:------:|------:|\n" +
			"| a    | b      | c     |\n" +
			"| **d**| `e`    | f     |\n",
		carets: []appearanceCaret{
			{"header", "Left", 1},
			{"strong", "**d**", 2},
		},
	},
	{
		name: "html",
		src: "<div class=\"box\">\n" +
			"  <span>inline html</span>\n" +
			"</div>\n" +
			"\n" +
			"![alt text](missing.png)\n" +
			"\n" +
			"<u>underlined</u> <span>plain</span> <br> tail\n",
		carets: []appearanceCaret{
			{"block", "<div", 2},
			{"image", "missing.png", 1},
		},
	},
	{
		name: "front",
		src: "---\n" +
			"title: Baseline\n" +
			"---\n" +
			"\n" +
			"[ref]: https://example.com\n" +
			"\n" +
			"# Top\n",
		carets: []appearanceCaret{
			{"matter", "title: Baseline", 2},
			{"def", "https://example.com", 2},
			{"heading", "# Top", 2},
		},
	},
}

// appearanceCase is one drawing of one document: a theme, and a caret to
// show the syntax around, or none to hide it.
type appearanceCase struct {
	name  string
	src   string
	dark  bool
	caret int
	focus bool
}

func appearanceCases() []appearanceCase {
	var cases []appearanceCase
	for _, d := range appearanceDocs {
		seen := map[string]bool{}
		for _, c := range d.carets {
			if seen[c.name] {
				panic("duplicate caret " + c.name)
			}
			seen[c.name] = true
			if strings.Count(d.src, c.marker) != 1 {
				panic(fmt.Sprintf("%s: marker %q occurs %d times", d.name, c.marker, strings.Count(d.src, c.marker)))
			}
		}
		for _, dark := range []bool{false, true} {
			theme := "light"
			if dark {
				theme = "dark"
			}
			cases = append(cases, appearanceCase{name: d.name + "-" + theme + "-hidden", src: d.src, dark: dark})
			for _, c := range d.carets {
				off := strings.Index(d.src, c.marker) + c.delta
				cases = append(cases, appearanceCase{
					name: d.name + "-" + theme + "-" + c.name,
					src:  d.src, dark: dark, caret: off, focus: true,
				})
			}
		}
	}
	return cases
}

// TestAppearanceBaseline records and checks the appearance goldens.
func TestAppearanceBaseline(t *testing.T) {
	update := os.Getenv(goldenEnv) != ""
	dir := filepath.Join("testdata", "appearance")
	if update {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cases := appearanceCases()
	if len(cases) < 40 {
		t.Fatalf("only %d cases", len(cases))
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			e, tt := newBaselineEditor(c)
			snap := appearanceSnapshot(e)
			checkGolden(t, filepath.Join(dir, c.name+".txt"), []byte(snap), update)
			checkGoldenPNG(t, filepath.Join(dir, c.name+".png"), tt.Image(), update)
		})
	}
}

// newBaselineEditor lays the document out in a fixed window, focused with
// the caret where the case says.
func newBaselineEditor(c appearanceCase) (*Editor, *ui.Tester) {
	e := New(c.src, Options{FontSize: 16})
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			e.View(c).Grow(1)
		})
	}, 720, 1200)
	tt.SetDark(c.dark)
	tt.Frame()
	tt.Frame()
	if c.focus {
		e.Focus()
		e.dropPhantom()
		e.anchor, e.caret = c.caret, c.caret
		// The caret blinks from the wall clock; pin its phase so the
		// pixels do not depend on how long the test has run.
		e.blinkStart = tt.Now()
		tt.Frame()
		tt.Frame()
	}
	return e, tt
}

// appearanceSnapshot writes what the editor laid out: each leaf's box and
// view, and each run of displayed text with its style.
func appearanceSnapshot(e *Editor) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "column %.2f %.2f %.2f\n", e.colX, e.colW, e.padTop)
	for _, l := range e.leaves {
		snapshotLeaf(&sb, e, l, "")
		if l.kind == kTable && l.tab != nil {
			for r, row := range l.tab.rows {
				for c, cell := range row {
					snapshotLeaf(&sb, e, cell, fmt.Sprintf("r%dc%d ", r, c))
				}
			}
		}
	}
	return sb.String()
}

func snapshotLeaf(sb *strings.Builder, e *Editor, l *leaf, cell string) {
	fmt.Fprintf(sb, "%s%s [%d,%d) line=%d syntax=%d box=%.2f,%.2f,%.2f,%.2f",
		cell, kindName[l.kind], l.start, l.end, l.lineStart, l.syntaxStart, l.x, l.y, l.w, l.h)
	if l.id < 0 {
		sb.WriteString(" raw")
	}
	if l.pending {
		sb.WriteString(" pending")
	}
	if l.setext {
		sb.WriteString(" setext")
	}
	if l.frontMatter {
		sb.WriteString(" frontmatter")
	}
	if l.whole {
		sb.WriteString(" whole")
	}
	for _, c := range l.conts {
		switch c.kind {
		case cQuote:
			sb.WriteString(" Q")
		case cItem:
			fmt.Fprintf(sb, " I%c", c.marker)
			if c.ordered {
				fmt.Fprintf(sb, "%d", c.num)
			}
			if c.task != 0 {
				fmt.Fprintf(sb, " task=%d", c.task)
			}
			if !c.tight {
				sb.WriteString(" loose")
			}
		case cFootnote:
			fmt.Fprintf(sb, " F:%s", c.label)
		}
	}
	if l.kind == kCode {
		fmt.Fprintf(sb, " lang=%q fenced=%v", l.lang, l.fenced)
	}
	if l.kind == kHeading {
		fmt.Fprintf(sb, " h%d", l.level)
	}
	sb.WriteString("\n")
	if l.view == nil {
		return
	}
	v := l.view
	fmt.Fprintf(sb, "  view panel=%v reveal=%v label=%q tx=%.2f ty=%.2f h=%.2f",
		v.panel, v.reveal, v.label, v.tx, v.ty, v.h)
	if v.block != nil {
		fmt.Fprintf(sb, " block obj %.2f,%.2f %.2fx%.2f descent=%.2f alone=%v alt=%q loaded=%v",
			v.bx, v.by, v.block.w, v.block.h, v.block.descent, v.block.block, v.block.alt, v.block.bmp != nil)
	}
	sb.WriteString("\n")
	if len(l.lines) > 0 {
		for _, ln := range l.lines {
			fmt.Fprintf(sb, "  line %d %q\n", ln.off, ln.text)
		}
	}
	for _, p := range v.disp.pieces {
		snapshotPiece(sb, p)
	}
}

func snapshotPiece(sb *strings.Builder, p piece) {
	kind := "text"
	switch p.kind {
	case pAtom:
		kind = "atom"
	case pDeco:
		kind = "deco"
	}
	txt := p.text
	if p.obj != nil {
		fmt.Fprintf(sb, "  %s s[%d,%d) d[%d,%d) obj alt=%q %.2fx%.2f descent=%.2f alone=%v",
			kind, p.s0, p.s1, p.d0, p.d1, p.obj.alt, p.obj.w, p.obj.h, p.obj.descent, p.obj.block)
	} else {
		fmt.Fprintf(sb, "  %s s[%d,%d) d[%d,%d) %q", kind, p.s0, p.s1, p.d0, p.d1, txt)
	}
	sp := p.span
	fmt.Fprintf(sb, " font=%q size=%.2f weight=%d italic=%v underline=%v wavy=%v strike=%v"+
		" color=%s decoration=%s:%.2f bg=%s radius=%.2f spacing=%.2f shift=%.2f features=%q lang=%q"+
		" marker=%v",
		sp.Font, sp.Size, sp.Weight, sp.Italic, sp.Underline, sp.WavyUnderline, sp.Strikethrough,
		hexColor(sp.Color), hexColor(sp.DecorationColor), sp.DecorationThickness,
		hexColor(sp.Background), sp.Radius, sp.LetterSpacing, sp.BaselineShift, sp.Features, sp.Lang,
		p.marker)
	if sp.ObjectWidth != 0 {
		fmt.Fprintf(sb, " object=%.2fx%.2f descent=%.2f", sp.ObjectWidth, sp.ObjectHeight, sp.ObjectDescent)
	}
	sb.WriteString("\n")
}

func hexColor(c ui.Color) string {
	return fmt.Sprintf("%02x%02x%02x%02x", c.R, c.G, c.B, c.A)
}

func checkGolden(t *testing.T, path string, got []byte, update bool) {
	t.Helper()
	if update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no golden %s (record with %s=1): %v", path, goldenEnv, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs (record with %s=1 to update)\n%s", path, goldenEnv, firstDiff(want, got))
	}
}

// firstDiff describes where two snapshots first differ.
func firstDiff(want, got []byte) string {
	wl := strings.Split(string(want), "\n")
	gl := strings.Split(string(got), "\n")
	for i := 0; i < len(wl) && i < len(gl); i++ {
		if wl[i] != gl[i] {
			return fmt.Sprintf("line %d:\n  want %s\n  got  %s", i+1, wl[i], gl[i])
		}
	}
	return fmt.Sprintf("want %d lines, got %d", len(wl), len(gl))
}

func checkGoldenPNG(t *testing.T, path string, got *image.RGBA, update bool) {
	t.Helper()
	if update {
		var buf bytes.Buffer
		if err := png.Encode(&buf, got); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("no golden %s (record with %s=1): %v", path, goldenEnv, err)
	}
	defer f.Close()
	dec, err := png.Decode(f)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	want := image.NewRGBA(dec.Bounds())
	draw.Draw(want, want.Bounds(), dec, dec.Bounds().Min, draw.Src)
	if want.Bounds().Size() != got.Bounds().Size() {
		t.Fatalf("%s is %v, frame is %v", path, want.Bounds().Size(), got.Bounds().Size())
	}
	diff, first := 0, image.Point{-1, -1}
	for y := 0; y < want.Bounds().Dy(); y++ {
		for x := 0; x < want.Bounds().Dx(); x++ {
			if want.RGBAAt(x, y) != got.RGBAAt(x, y) {
				if diff == 0 {
					first = image.Point{x, y}
				}
				diff++
			}
		}
	}
	if diff > 0 {
		t.Errorf("%s: %d pixels differ, first at %v (record with %s=1 to update)", path, diff, first, goldenEnv)
	}
}
