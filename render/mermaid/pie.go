package mermaid

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/livemark/render"
)

type pieChart struct {
	title  string
	slices []pieSlice
}

type pieSlice struct {
	label string
	value float64
}

func parsePie(src string) (*pieChart, bool) {
	lines := strings.Split(src, "\n")
	if len(lines) == 0 {
		return nil, false
	}
	head := strings.ToLower(strings.TrimSpace(lines[0]))
	if head != "pie" && !strings.HasPrefix(head, "pie ") {
		return nil, false
	}
	g := &pieChart{}
	if strings.HasPrefix(head, "pie title ") {
		g.title = strings.TrimSpace(lines[0][len("pie title "):])
	}
	for _, raw := range lines[1:] {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "%%") {
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "title ") {
			g.title = strings.TrimSpace(line[len("title "):])
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "showdata") {
			continue
		}
		colon := strings.LastIndex(line, ":")
		if colon < 0 {
			continue
		}
		label := strings.Trim(strings.TrimSpace(line[:colon]), `"`)
		value, err := strconv.ParseFloat(strings.TrimSpace(line[colon+1:]), 64)
		if err != nil || value <= 0 || label == "" {
			continue
		}
		g.slices = append(g.slices, pieSlice{label: label, value: value})
	}
	if len(g.slices) == 0 {
		return nil, false
	}
	return g, true
}

var piePalette = []string{
	"#4f7cff", "#2bb5a0", "#f0a43c", "#9b6cff",
	"#e5679b", "#6aa84f", "#5aa9e6", "#d97757",
}

// draw paints the diagram in r.
func (g *pieChart) draw(p *ui.Painter, r ui.Rect, style render.DiagramStyle) {
	titleH := float32(0)
	if g.title != "" {
		titleH = 26
		sp := ui.Span{Text: g.title, Size: 13, Color: style.Text}
		tw, _ := p.MeasureText(0, sp)
		p.RichText(r.X+(r.W-tw)/2, r.Y+8, 0, sp)
	}
	total := 0.0
	for _, s := range g.slices {
		total += s.value
	}
	radius := float32(math.Min(92, float64((r.H-titleH-28)/2)))
	cx := r.X + radius + 44
	cy := r.Y + titleH + (r.H-titleH)/2
	start := float32(-math.Pi / 2)
	for i, s := range g.slices {
		sweep := float32(s.value/total) * 2 * float32(math.Pi)
		path := ui.Path{}
		path.MoveTo(cx, cy)
		path.LineTo(cx+float32(math.Cos(float64(start)))*radius, cy+float32(math.Sin(float64(start)))*radius)
		steps := int(math.Ceil(float64(sweep) / (math.Pi / 30)))
		if steps < 1 {
			steps = 1
		}
		for j := 1; j <= steps; j++ {
			a := float64(start) + float64(sweep)*float64(j)/float64(steps)
			path.LineTo(cx+float32(math.Cos(a))*radius, cy+float32(math.Sin(a))*radius)
		}
		path.Close()
		color := ui.Hex(piePalette[i%len(piePalette)])
		p.FillPath(&path, color)
		p.StrokePath(&path, 1.2, style.Background)
		start += sweep
	}

	// Legend.
	lx := cx + radius + 52
	ly := r.Y + titleH + 18
	for i, s := range g.slices {
		if ly+22 > r.Y+r.H-8 {
			break
		}
		color := ui.Hex(piePalette[i%len(piePalette)])
		p.Fill(ui.Rect{X: lx, Y: ly + 4, W: 10, H: 10}, color, 2)
		sp := ui.Span{Text: s.label, Size: 11.5, Color: style.Text}
		p.RichText(lx+16, ly, 150, sp)
		pct := ui.Span{Text: fmt.Sprintf("%.0f%%", s.value/total*100), Size: 11, Color: style.Muted}
		p.RichText(lx+180, ly, 60, pct)
		ly += 24
	}
}
