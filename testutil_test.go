package livemark

import (
	"github.com/CarlOpenLab/livemark/render"
	mathrender "github.com/CarlOpenLab/livemark/render/math"
	mermaidrender "github.com/CarlOpenLab/livemark/render/mermaid"
)

// testOptions returns the options the appearance baseline was drawn with:
// formulas typeset and Mermaid diagrams drawn.
func testOptions() Options {
	return Options{
		Math:     mathrender.New(),
		Diagrams: []render.DiagramRenderer{mermaidrender.New()},
	}
}
