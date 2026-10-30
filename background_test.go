package livemark

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/livemark/theme"
)

// TestRootBackground checks that a theme's :root background-color fills
// the page the document is drawn on.
func TestRootBackground(t *testing.T) {
	sheet, ds := theme.Parse("page", ":root { background-color: #123456; }")
	for _, d := range ds {
		t.Fatalf("theme: %s", d)
	}
	e := New("hello", Options{Theme: theme.New(sheet)})
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			e.View(c).Grow(1)
		})
	}, 300, 200)
	tt.Frame()
	tt.Frame()
	r, g, b, a := tt.Image().At(150, 190).RGBA()
	if r != 0x12*0x101 || g != 0x34*0x101 || b != 0x56*0x101 || a != 0xffff {
		t.Fatalf("pixel = %04x %04x %04x %04x, want #123456", r, g, b, a)
	}
}
