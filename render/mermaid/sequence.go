package mermaid

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/livemark/render"
)

// seqGraph is a native sequenceDiagram subset: participants, messages,
// dashed replies and notes. Loops, alt blocks and activations are parsed
// without their frames, so the messages still read in order.
type seqGraph struct {
	parts  []*seqParticipant
	byID   map[string]*seqParticipant
	events []seqEvent
}

type seqParticipant struct {
	id, label string
	x, w      float32
}

type seqEvent struct {
	kind   string // message | note
	from   string
	to     string
	text   string
	dashed bool
}

func parseSequence(src string) (*seqGraph, bool) {
	lines := strings.Split(src, "\n")
	if len(lines) == 0 || !strings.EqualFold(strings.TrimSpace(lines[0]), "sequencediagram") {
		return nil, false
	}
	g := &seqGraph{byID: map[string]*seqParticipant{}}
	for _, raw := range lines[1:] {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "%%") {
			continue
		}
		low := strings.ToLower(line)
		switch {
		case strings.HasPrefix(low, "participant "), strings.HasPrefix(low, "actor "):
			rest := strings.TrimSpace(line[strings.IndexByte(line, ' ')+1:])
			id, label := rest, rest
			if at := strings.Index(low, " as "); at >= 0 {
				id = strings.TrimSpace(line[len("participant "):at])
				if strings.HasPrefix(low, "actor ") {
					id = strings.TrimSpace(line[len("actor "):at])
				}
				label = strings.TrimSpace(line[at+4:])
			}
			g.participant(id, label)
			continue
		case strings.HasPrefix(low, "note "):
			rest := line[len("note "):]
			colon := strings.Index(rest, ":")
			if colon < 0 {
				continue
			}
			whereRaw := strings.TrimSpace(rest[:colon])
			where := strings.ToLower(whereRaw)
			text := strings.TrimSpace(rest[colon+1:])
			ids := []string{}
			if strings.HasPrefix(where, "over ") {
				for _, id := range strings.Split(whereRaw[len("over "):], ",") {
					ids = append(ids, strings.TrimSpace(id))
				}
			} else if strings.HasPrefix(where, "left of ") {
				ids = append(ids, strings.TrimSpace(whereRaw[len("left of "):]))
			} else if strings.HasPrefix(where, "right of ") {
				ids = append(ids, strings.TrimSpace(whereRaw[len("right of "):]))
			}
			if len(ids) == 0 {
				continue
			}
			for _, id := range ids {
				g.participant(id, id)
			}
			g.events = append(g.events, seqEvent{kind: "note", from: ids[0], to: ids[len(ids)-1], text: text})
			continue
		case strings.HasPrefix(low, "loop "), strings.HasPrefix(low, "alt "),
			strings.HasPrefix(low, "else"), strings.HasPrefix(low, "end"),
			strings.HasPrefix(low, "opt "), strings.HasPrefix(low, "par "),
			strings.HasPrefix(low, "activate "), strings.HasPrefix(low, "deactivate "),
			strings.HasPrefix(low, "autonumber"), strings.HasPrefix(low, "title "),
			strings.HasPrefix(low, "rect "), strings.HasPrefix(low, "critical "),
			strings.HasPrefix(low, "break "):
			continue
		}
		colon := strings.Index(line, ":")
		if colon < 0 {
			continue
		}
		left := strings.TrimSpace(line[:colon])
		text := strings.TrimSpace(line[colon+1:])
		from, to, dashed, ok := splitSeqArrow(left)
		if !ok {
			continue
		}
		g.participant(from, from)
		g.participant(to, to)
		g.events = append(g.events, seqEvent{kind: "message", from: from, to: to, text: text, dashed: dashed})
	}
	if len(g.events) == 0 {
		return nil, false
	}
	return g, true
}

func (g *seqGraph) participant(id, label string) *seqParticipant {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	if p, ok := g.byID[id]; ok {
		if label != "" && label != id {
			p.label = label
		}
		return p
	}
	p := &seqParticipant{id: id, label: label}
	g.byID[id] = p
	g.parts = append(g.parts, p)
	return p
}

func splitSeqArrow(left string) (from, to string, dashed, ok bool) {
	for _, arrow := range []string{"-->>", "->>", "-->", "->", "--x", "-x", "--)"} {
		if i := strings.Index(left, arrow); i >= 0 {
			from = strings.TrimRight(strings.TrimSpace(left[:i]), "-+")
			to = strings.TrimLeft(strings.TrimSpace(left[i+len(arrow):]), "+-")
			to = strings.TrimRight(to, "-+")
			if from == "" || to == "" {
				return "", "", false, false
			}
			return from, to, strings.HasPrefix(arrow, "--"), true
		}
	}
	return "", "", false, false
}

func (g *seqGraph) height() float32 {
	return 84 + float32(len(g.events))*42 + 24
}

// draw paints the diagram in r.
func (g *seqGraph) draw(p *ui.Painter, r ui.Rect, s render.DiagramStyle) {
	g.layout(r)
	// Participant boxes and lifelines.
	top := r.Y + 12
	for _, part := range g.parts {
		box := ui.Rect{X: part.x - part.w/2, Y: top, W: part.w, H: 30}
		p.Fill(box, s.Fill, 6)
		p.Stroke(box, s.Line, 6, 1)
		sp := ui.Span{Text: part.label, Size: 12, Color: s.Text}
		tw, th := p.MeasureText(box.W-12, sp)
		p.RichText(box.X+(box.W-tw)/2, box.Y+(box.H-th)/2, box.W-12, sp)
		// Dotted lifeline.
		for y := box.Y + box.H + 6; y < r.Y+r.H-12; y += 7 {
			p.Line(part.x, y, part.x, y+3, 1, s.Line)
		}
	}
	y := top + 54
	for _, e := range g.events {
		if e.kind == "note" {
			a, b := g.byID[e.from], g.byID[e.to]
			if a == nil || b == nil {
				continue
			}
			x0, x1 := a.x, b.x
			if x0 > x1 {
				x0, x1 = x1, x0
			}
			box := ui.Rect{X: x0 - 40, Y: y, W: x1 - x0 + 80, H: 28}
			p.Fill(box, s.Fill, 5)
			p.Stroke(box, s.Line, 5, 1)
			sp := ui.Span{Text: e.text, Size: 10.5, Color: s.Text}
			tw, th := p.MeasureText(box.W-12, sp)
			p.RichText(box.X+(box.W-tw)/2, box.Y+(box.H-th)/2, box.W-12, sp)
			y += 34
			continue
		}
		a, b := g.byID[e.from], g.byID[e.to]
		if a == nil || b == nil {
			continue
		}
		drawSeqMessage(p, s, a.x, b.x, y, e.text, e.dashed)
		y += 42
	}
}

func (g *seqGraph) layout(r ui.Rect) {
	if len(g.parts) == 0 {
		return
	}
	w := r.W / float32(len(g.parts))
	if w > 180 {
		w = 180
	}
	if w < 70 {
		w = 70
	}
	total := w * float32(len(g.parts))
	x := r.X + (r.W-total)/2
	for _, p := range g.parts {
		p.w = w - 10
		p.x = x + w/2
		x += w
	}
}

func drawSeqMessage(p *ui.Painter, s render.DiagramStyle, x1, x2, y float32, text string, dashed bool) {
	color := s.Muted
	if x1 == x2 {
		// Self message: a small loop.
		p.Line(x1, y, x1+24, y, 1.3, color)
		p.Line(x1+24, y, x1+24, y+10, 1.3, color)
		p.Line(x2+11, y+10, x2, y+10, 1.3, color)
		arrowRight(p, x2, y+10, color)
		sp := ui.Span{Text: text, Size: 10.5, Color: s.Text}
		p.RichText(x1+30, y-14, 0, sp)
		return
	}
	dir := float32(1)
	if x2 < x1 {
		dir = -1
	}
	if dashed {
		for x := x1; (dir > 0 && x < x2) || (dir < 0 && x > x2); x += dir * 8 {
			end := x + dir*4
			p.Line(x, y, end, y, 1.3, color)
		}
	} else {
		p.Line(x1, y, x2, y, 1.3, color)
	}
	if dir > 0 {
		arrowRight(p, x2, y, color)
	} else {
		arrowLeft(p, x2, y, color)
	}
	sp := ui.Span{Text: text, Size: 10.5, Color: s.Text}
	tw, _ := p.MeasureText(0, sp)
	p.RichText((x1+x2)/2-tw/2, y-15, 0, sp)
}

func arrowRight(p *ui.Painter, x, y float32, c ui.Color) {
	p.Line(x, y, x-7, y-4, 1.3, c)
	p.Line(x, y, x-7, y+4, 1.3, c)
}

func arrowLeft(p *ui.Painter, x, y float32, c ui.Color) {
	p.Line(x, y, x+7, y-4, 1.3, c)
	p.Line(x, y, x+7, y+4, 1.3, c)
}
