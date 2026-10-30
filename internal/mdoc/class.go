package mdoc

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// classGraph is a native classDiagram subset: classes with members,
// inheritance/realization, composition/aggregation, association and
// dependency. Notes, namespaces and styles are ignored.
type classGraph struct {
	classes   []*classNode
	byName    map[string]*classNode
	relations []classRel
	rowH      []float32
}

type classNode struct {
	name    string
	members []string
	level   int
	rect    ui.Rect
	w, h    float32
}

type classRel struct {
	from, to *classNode
	kind     string
	label    string
}

func parseClass(src string) (*classGraph, bool) {
	lines := strings.Split(src, "\n")
	if len(lines) == 0 || !strings.EqualFold(strings.TrimSpace(lines[0]), "classdiagram") {
		return nil, false
	}
	g := &classGraph{byName: map[string]*classNode{}}
	inBody := false
	for _, raw := range lines[1:] {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "%%") {
			continue
		}
		if inBody {
			if line == "}" {
				inBody = false
				continue
			}
			if len(g.classes) > 0 {
				g.classes[len(g.classes)-1].members = append(g.classes[len(g.classes)-1].members, line)
			}
			continue
		}
		low := strings.ToLower(line)
		if strings.HasPrefix(low, "direction ") || strings.HasPrefix(low, "namespace ") ||
			strings.HasPrefix(low, "note ") || strings.HasPrefix(low, "style ") ||
			strings.HasPrefix(low, "classdef") || strings.HasPrefix(low, "cssclass") ||
			strings.HasPrefix(low, "<<") {
			continue
		}
		if strings.HasPrefix(low, "class ") {
			rest := strings.TrimSpace(line[len("class "):])
			if strings.HasSuffix(rest, "{") {
				rest = strings.TrimSpace(strings.TrimSuffix(rest, "{"))
				inBody = true
			}
			g.class(rest)
			continue
		}
		if arrow, at := findClassArrow(line); at >= 0 {
			left := strings.TrimSpace(line[:at])
			right := strings.TrimSpace(line[at+len(arrow):])
			label := ""
			if colon := strings.Index(right, ":"); colon >= 0 {
				label = strings.TrimSpace(right[colon+1:])
				right = strings.TrimSpace(right[:colon])
			}
			fromName := cleanClassName(left)
			toName := cleanClassName(right)
			from, to := g.class(fromName), g.class(toName)
			if from == nil || to == nil || from == to {
				continue
			}
			kind := "assoc"
			reverse := false
			switch arrow {
			case "<|--":
				kind = "inherit"
			case "<|..":
				kind, reverse = "realize", false
			case "*--":
				kind = "compose"
			case "o--":
				kind = "aggregate"
			case "-->":
				kind = "assoc"
			case "<--":
				kind, reverse = "assoc", true
			case "..>":
				kind = "depend"
			case "<..":
				kind, reverse = "depend", true
			case "..|>":
				kind = "realize"
			}
			if reverse {
				from, to = to, from
			}
			g.relations = append(g.relations, classRel{from: from, to: to, kind: kind, label: label})
			continue
		}
		if colon := strings.Index(line, ":"); colon >= 0 {
			name := cleanClassName(strings.TrimSpace(line[:colon]))
			if c := g.class(name); c != nil {
				c.members = append(c.members, strings.TrimSpace(line[colon+1:]))
			}
		}
	}
	if len(g.classes) == 0 {
		return nil, false
	}
	g.levels()
	g.sizes()
	return g, true
}

func (g *classGraph) class(name string) *classNode {
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, " ") && strings.Contains(name, `"`) {
		// Cardinalities such as "1" are not class names.
		return nil
	}
	if c, ok := g.byName[name]; ok {
		return c
	}
	c := &classNode{name: name}
	g.byName[name] = c
	g.classes = append(g.classes, c)
	return c
}

func findClassArrow(s string) (string, int) {
	for _, a := range []string{"<|--", "<|..", "..|>", "..>", "<..", "*--", "o--", "-->", "<--", "--", ".."} {
		if i := strings.Index(s, a); i >= 0 {
			return a, i
		}
	}
	return "", -1
}

func cleanClassName(s string) string {
	s = strings.TrimSpace(s)
	for {
		if strings.HasPrefix(s, `"`) {
			if end := strings.Index(s[1:], `"`); end >= 0 {
				s = strings.TrimSpace(s[end+2:])
				continue
			}
		}
		if strings.HasSuffix(s, `"`) {
			if start := strings.LastIndex(s[:len(s)-1], `"`); start >= 0 {
				s = strings.TrimSpace(s[:start])
				continue
			}
		}
		break
	}
	return strings.TrimSpace(strings.TrimSuffix(s, "{"))
}

func (g *classGraph) levels() {
	for _, c := range g.classes {
		c.level = -1
	}
	indeg := map[*classNode]int{}
	out := map[*classNode][]*classNode{}
	for _, r := range g.relations {
		indeg[r.to]++
		out[r.from] = append(out[r.from], r.to)
	}
	var queue []*classNode
	for _, c := range g.classes {
		if indeg[c] == 0 {
			c.level = 0
			queue = append(queue, c)
		}
	}
	if len(queue) == 0 && len(g.classes) > 0 {
		g.classes[0].level = 0
		queue = append(queue, g.classes[0])
	}
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		for _, v := range out[u] {
			if v.level < 0 {
				v.level = u.level + 1
				queue = append(queue, v)
			}
		}
	}
	for _, c := range g.classes {
		if c.level < 0 {
			c.level = 0
		}
	}
}

func (g *classGraph) sizes() {
	maxLevel := 0
	for _, c := range g.classes {
		w := float32(len([]rune(c.name)))*8 + 32
		for _, m := range c.members {
			if mw := float32(len([]rune(m)))*6.4 + 24; mw > w {
				w = mw
			}
		}
		if w < 130 {
			w = 130
		}
		if w > 260 {
			w = 260
		}
		c.w = w
		c.h = 30
		if len(c.members) > 0 {
			c.h += float32(len(c.members))*17 + 8
		}
		if c.level > maxLevel {
			maxLevel = c.level
		}
	}
	g.rowH = make([]float32, maxLevel+1)
	for _, c := range g.classes {
		if c.h > g.rowH[c.level] {
			g.rowH[c.level] = c.h
		}
	}
}

func (g *classGraph) height() float32 {
	h := float32(20)
	for _, rh := range g.rowH {
		h += rh + 54
	}
	return h
}

// draw paints the diagram in r.
func (g *classGraph) draw(p *ui.Painter, r ui.Rect, t *ui.Theme) {
	g.layout(r)
	for _, rel := range g.relations {
		drawClassRel(p, t, rel)
	}
	for _, c := range g.classes {
		box := c.rect
		p.Fill(box, t.Surface, 6)
		p.Stroke(box, t.Border, 6, 1)
		name := ui.Span{Text: c.name, Size: 12.5, Weight: 650, Color: t.Text}
		nw, _ := p.MeasureText(box.W-16, name)
		p.RichText(box.X+(box.W-nw)/2, box.Y+7, box.W-16, name)
		if len(c.members) == 0 {
			continue
		}
		p.Line(box.X, box.Y+30, box.X+box.W, box.Y+30, 1, t.Border)
		y := box.Y + 35
		for _, m := range c.members {
			sp := ui.Span{Text: m, Size: 10.5, Color: t.TextMuted}
			p.RichText(box.X+9, y, box.W-18, sp)
			y += 17
		}
	}
}

func (g *classGraph) layout(r ui.Rect) {
	rows := map[int][]*classNode{}
	maxLevel := 0
	for _, c := range g.classes {
		rows[c.level] = append(rows[c.level], c)
		if c.level > maxLevel {
			maxLevel = c.level
		}
	}
	y := r.Y + 16
	for l := 0; l <= maxLevel; l++ {
		row := rows[l]
		if len(row) == 0 {
			continue
		}
		total := float32(0)
		for i, c := range row {
			total += c.w
			if i > 0 {
				total += 30
			}
		}
		x := r.X + (r.W-total)/2
		for _, c := range row {
			c.rect = ui.Rect{X: x, Y: y + (g.rowH[l]-c.h)/2, W: c.w, H: c.h}
			x += c.w + 30
		}
		y += g.rowH[l] + 54
	}
}

func drawClassRel(p *ui.Painter, t *ui.Theme, rel classRel) {
	a, b := rel.from.rect, rel.to.rect
	x1, y1 := a.X+a.W/2, a.Y+a.H
	x2, y2 := b.X+b.W/2, b.Y
	// A child above its parent (cycle) is routed around the right side.
	if y2 < y1 {
		x1 = a.X + a.W
		y1 = a.Y + a.H/2
		x2 = b.X + b.W
		y2 = b.Y + b.H/2
		mid := maxF(x1, x2) + 26
		p.Line(x1, y1, mid, y1, 1.3, t.TextMuted)
		p.Line(mid, y1, mid, y2, 1.3, t.TextMuted)
		p.Line(mid, y2, x2, y2, 1.3, t.TextMuted)
		return
	}
	dashed := rel.kind == "depend" || rel.kind == "realize"
	if dashed {
		drawDashedLine(p, x1, y1, x2, y2, t.TextMuted)
	} else {
		p.Line(x1, y1, x2, y2, 1.3, t.TextMuted)
	}
	switch rel.kind {
	case "inherit", "realize":
		// Hollow triangle at the parent end.
		path := ui.Path{}
		path.MoveTo(x1, y1)
		path.LineTo(x1-6, y1+11)
		path.LineTo(x1+6, y1+11)
		path.Close()
		p.FillPath(&path, t.Background)
		p.StrokePath(&path, 1.2, t.TextMuted)
	case "compose", "aggregate":
		// Diamond at the owner end.
		path := ui.Path{}
		path.MoveTo(x1, y1)
		path.LineTo(x1-6, y1-11)
		path.LineTo(x1, y1-22)
		path.LineTo(x1+6, y1-11)
		path.Close()
		if rel.kind == "compose" {
			p.FillPath(&path, t.TextMuted)
		} else {
			p.FillPath(&path, t.Background)
			p.StrokePath(&path, 1.2, t.TextMuted)
		}
	default:
		p.Line(x2, y2, x2-5, y2+8, 1.2, t.TextMuted)
		p.Line(x2, y2, x2+5, y2+8, 1.2, t.TextMuted)
	}
	if rel.label != "" {
		sp := ui.Span{Text: rel.label, Size: 10, Color: t.TextMuted}
		tw, _ := p.MeasureText(0, sp)
		p.RichText((x1+x2)/2-tw/2, (y1+y2)/2-12, 0, sp)
	}
}

func drawDashedLine(p *ui.Painter, x1, y1, x2, y2 float32, c ui.Color) {
	dx, dy := x2-x1, y2-y1
	length := float32(1)
	if dx != 0 || dy != 0 {
		length = float32(1)
	}
	// Simple dash walk, good enough for the mostly vertical class edges.
	steps := int((absF(dx) + absF(dy)) / 8)
	if steps < 1 {
		steps = 1
	}
	for i := 0; i < steps; i += 2 {
		t0 := float32(i) / float32(steps)
		t1 := float32(i+1) / float32(steps)
		p.Line(x1+dx*t0, y1+dy*t0, x1+dx*t1, y1+dy*t1, 1.2, c)
	}
	_ = length
}

func absF(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
