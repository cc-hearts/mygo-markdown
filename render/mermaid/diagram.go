// Package mermaid draws the Mermaid diagrams a code block holds, natively:
// flowcharts, sequence, state, pie, class and Gantt subsets.
package mermaid

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/livemark/render"
)

// Diagram is a Mermaid diagram drawn natively: flowcharts, sequence,
// state, pie, class and Gantt subsets.
type Diagram struct {
	height func() float32
	draw   func(p *ui.Painter, r ui.Rect, s render.DiagramStyle)
}

// Height returns how tall the diagram is at a width.
func (d Diagram) Height(width float32) float32 { return d.height() }

// Draw paints the diagram in r, in style s.
func (d Diagram) Draw(p *ui.Painter, r ui.Rect, s render.DiagramStyle) { d.draw(p, r, s) }

// Renderer draws the code blocks of the language mermaid.
type Renderer struct{}

// New returns a renderer of Mermaid diagrams.
func New() *Renderer { return &Renderer{} }

// Diagram returns the diagram code holds, when lang is mermaid's.
func (r *Renderer) Diagram(lang, code string) (render.Diagram, bool) {
	if !strings.EqualFold(strings.TrimSpace(lang), "mermaid") {
		return nil, false
	}
	d, ok := ParseDiagram(code)
	if !ok {
		return nil, false
	}
	return d, true
}

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
