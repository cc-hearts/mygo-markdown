package livemark

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The benchmarks run the editor offscreen on a fixed corpus, for comparing
// the theme rewrite against this baseline with benchstat:
//
//	go test -run '^$' -bench . -count 10 . > old.txt
//	# rewrite the editor
//	go test -run '^$' -bench . -count 10 . > new.txt
//	benchstat old.txt new.txt

// benchSource returns a document of a few hundred lines covering the
// constructs the editor lays out: headings, prose with inline syntax,
// lists, quotes, code, math and tables.
func benchSource(reps int) string {
	const block = "## Section\n" +
		"\n" +
		"Some **bold** and *emphasis*, `code`, a [link](https://example.com),\n" +
		"$x + y$ and a note[^n].\n" +
		"\n" +
		"- item one\n" +
		"- item two\n" +
		"\n" +
		"> quoted text\n" +
		"\n" +
		"~~~go\n" +
		"func main() { println(\"hi\") }\n" +
		"~~~\n" +
		"\n" +
		"| a | b |\n" +
		"|---|---|\n" +
		"| 1 | 2 |\n" +
		"\n"
	return strings.Repeat(block, reps) + "[^n]: note\n"
}

// newBenchEditor lays src out in a fixed window.
func newBenchEditor(b *testing.B, src string) (*Editor, *ui.Tester) {
	b.Helper()
	e := New(src, Options{FontSize: 16})
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			e.View(c).Grow(1)
		})
	}, 900, 700)
	tt.Frame()
	tt.Frame()
	return e, tt
}

// BenchmarkKeystroke measures a typed character and the frame that shows
// it, with the caret in prose.
func BenchmarkKeystroke(b *testing.B) {
	src := benchSource(120)
	e, tt := newBenchEditor(b, src)
	e.Focus()
	tt.Frame()
	off := strings.Index(src, "Some **bold**") + len("Some ")
	for i := 0; i < b.N; i++ {
		// Put the document back as it was, outside the measurement.
		b.StopTimer()
		e.ReplaceAll(src)
		e.dropPhantom()
		e.anchor, e.caret = off, off
		tt.Frame()
		b.StartTimer()
		tt.Type("x")
		tt.Frame()
	}
}

// BenchmarkCaretMove measures moving the caret a line and drawing the
// frame that reveals or hides syntax around it.
func BenchmarkCaretMove(b *testing.B) {
	_, tt := newBenchEditor(b, benchSource(120))
	tt.Frame()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%2 == 0 {
			tt.Key(0, ui.KeyDown)
		} else {
			tt.Key(0, ui.KeyUp)
		}
		tt.Frame()
	}
}

// BenchmarkRelayout measures laying the whole document out again, as a
// window resized by a pixel does.
func BenchmarkRelayout(b *testing.B) {
	_, tt := newBenchEditor(b, benchSource(120))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%2 == 0 {
			tt.SetSize(901, 700)
		} else {
			tt.SetSize(900, 700)
		}
	}
}

// BenchmarkThemeSwitch measures switching between the light and the dark
// appearance, which drops every shaped view.
func BenchmarkThemeSwitch(b *testing.B) {
	_, tt := newBenchEditor(b, benchSource(120))
	dark := false
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dark = !dark
		tt.SetDark(dark)
	}
}
