package mdoc

import "github.com/egoist/mygo/ui"

// Diagram is a Mermaid diagram mdoc draws natively: flowcharts, sequence,
// state, pie, class and Gantt subsets.
type Diagram struct {
	height func() float32
	draw   func(p *ui.Painter, r ui.Rect, t *ui.Theme)
}

// Height returns how tall the diagram is at a width.
func (d Diagram) Height(width float32) float32 { return d.height() }

// Draw paints the diagram in r, in the colors of t.
func (d Diagram) Draw(p *ui.Painter, r ui.Rect, t *ui.Theme) { d.draw(p, r, t) }

// ParseDiagram reads a Mermaid diagram, in the order the renderer tries
// its kinds, and reports false for one it cannot draw.
func ParseDiagram(code string) (Diagram, bool) {
	if g, ok := parseGantt(code); ok {
		return Diagram{height: g.height, draw: g.draw}, true
	}
	if g, ok := parseClass(code); ok {
		return Diagram{height: g.height, draw: g.draw}, true
	}
	if g, ok := parsePie(code); ok {
		return Diagram{height: func() float32 { return 286 }, draw: g.draw}, true
	}
	if g, ok := parseState(code); ok {
		return Diagram{height: g.height, draw: g.draw}, true
	}
	if g, ok := parseSequence(code); ok {
		return Diagram{height: g.height, draw: g.draw}, true
	}
	if g, ok := parseMermaid(code); ok {
		return Diagram{height: g.height, draw: g.draw}, true
	}
	return Diagram{}, false
}
