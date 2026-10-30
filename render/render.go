// Package render holds the interfaces through which the editor draws what
// Markdown alone cannot: formulas and diagrams. Implementations live apart
// from the editor, so that an app that needs none links none of them.
package render

import "github.com/egoist/mygo/ui"

// MathRenderer typesets TeX: the bitmap, and its baseline in DIPs from the
// bitmap's top, which the text around an inline formula lines up with. ok
// is false for TeX the renderer cannot read.
type MathRenderer interface {
	Math(tex string, display bool, size float32, color ui.Color) (img *ui.Bitmap, baseline float32, ok bool)
}

// DiagramRenderer draws the code blocks of the languages it knows.
type DiagramRenderer interface {
	Diagram(lang, code string) (Diagram, bool)
}

// Diagram is a diagram a code block shows instead of its text.
type Diagram interface {
	// Height returns how tall the diagram is at a width.
	Height(width float32) float32
	// Draw paints the diagram in r, in style s.
	Draw(p *ui.Painter, r ui.Rect, s DiagramStyle)
}

// DiagramStyle is what a diagram draws in: the colors of the code block's
// computed style and the theme's diagram variables, and its font size.
type DiagramStyle struct {
	Text, Muted, Line, Fill, Accent, Background ui.Color
	FontSize                                    float32
}
