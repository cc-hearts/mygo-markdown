package livemark

import (
	"slices"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"

	"github.com/CarlOpenLab/livemark/theme"
)

const blink = 530 * time.Millisecond

// paint draws the document: the leaves in view, the selection, what a
// search found, and the caret.
func (e *Editor) paint(p *ui.Painter, r ui.Rect) {
	e.elemRect = r
	if r.W != e.elemW && r.W > 0 {
		// The width changed since the layout: lay out again next frame.
		e.elemW = r.W
		e.invalidate()
	}
	t := p.Theme()
	vis := p.Visible()
	top, bottom := vis.Y-r.Y-40, vis.Y-r.Y+vis.H+40
	ox, oy := r.X, r.Y

	// The containers' chrome: quotation bars from their first leaf to their
	// last.
	type bar struct {
		x, y0, y1 float32
		w         float32
		color     ui.Color
	}
	bars := map[int]*bar{}
	var order []int
	for _, l := range e.leaves {
		if !hasQuote(l.conts) {
			continue
		}
		x := e.colX
		st := e.styledLeaf(l)
		for k, c := range l.conts {
			cs := st.contStyle[k]
			if c.kind == cQuote {
				b := bars[c.id]
				if b == nil {
					bw := cs.Border[3].Width
					if bw <= 0 {
						bw = 3
					}
					b = &bar{x: x, y0: l.y, y1: l.y + l.h, w: bw, color: uiColor(cs.Border[3].Color)}
					bars[c.id] = b
					order = append(order, c.id)
				}
				b.y1 = l.y + l.h
			}
			x += cs.Padding.Left
		}
	}
	for _, id := range order {
		b := bars[id]
		if b.y1 < top || b.y0 > bottom {
			continue
		}
		p.Fill(ui.Rect{X: ox + b.x + 1, Y: oy + b.y0 - 2, W: b.w, H: b.y1 - b.y0 + 4}, b.color, b.w/2)
	}

	a, z := e.selection()
	for _, l := range e.leaves {
		if l.y+l.h < top || l.y > bottom {
			continue
		}
		e.paintMarkers(p, l, ox, oy)
		switch l.kind {
		case kTable:
			e.paintTable(p, l, ox, oy, t)
			for _, cell := range l.tab.cells() {
				e.paintUnit(p, cell, ox, oy, a, z)
			}
		default:
			e.paintUnit(p, l, ox, oy, a, z)
		}
	}
	if len(e.leaves) == 1 && e.leaves[0].kind == kPhantom && e.opts.Placeholder != "" && e.compose == "" {
		l := e.leaves[0]
		sp := ui.Span{Text: e.opts.Placeholder, Color: e.lk.muted.Alpha(0.6), Size: e.lk.size}
		p.RichText(ox+l.x, oy+l.y+l.h/2-e.lk.size*0.7, l.w, sp)
	}
	e.paintCaret(p, ox, oy)
}

// hasQuote reports whether a leaf is inside a quotation.
func hasQuote(conts []cont) bool {
	for _, c := range conts {
		if c.kind == cQuote {
			return true
		}
	}
	return false
}

// paintUnit draws a leaf or a cell: its panel, what is found and selected
// in it, its text and its objects.
func (e *Editor) paintUnit(p *ui.Painter, u *leaf, ox, oy float32, a, z int) {
	v := u.view
	if v == nil {
		return
	}
	x, y := ox+u.x, oy+u.y
	box := ui.Rect{X: x, Y: y, W: u.w, H: u.h}
	if v.panel {
		p.Fill(box, uiColor(v.style.Background), v.style.Radius)
	}
	if u.kind == kRule && v.layout == nil {
		if b := v.style.Border[0]; b.Width > 0 {
			p.Fill(ui.Rect{X: x, Y: y + u.h/2 - b.Width/2, W: u.w, H: b.Width}, uiColor(b.Color), 0)
		}
	}
	if v.layout != nil {
		lx, ly := x+v.tx, y+v.ty
		for i, h := range e.highlights {
			col := ui.RGBA(250, 204, 21, 0.35)
			if i == e.current {
				col = ui.RGBA(250, 150, 20, 0.55)
			}
			e.fillRange(p, u, v, h.Start, h.End, lx, ly, col)
		}
		if a != z {
			sel := e.lk.selection
			if !e.focused {
				sel = sel.Alpha(0.6)
			}
			e.fillRange(p, u, v, a, z, lx, ly, sel)
		}
		if v.diagram == nil {
			p.TextLayout(v.layout, lx, ly, e.lk.text)
		}
		e.paintObjects(p, v, lx, ly)
	} else if a != z && a <= leafLo(u) && z >= u.end {
		p.Fill(box, e.lk.selection, 4)
	}
	if v.block != nil {
		e.paintObject(p, v.block, ui.Rect{X: x + v.bx, Y: y + v.by, W: v.block.w, H: v.block.h})
	}
	if v.diagram != nil {
		pad := v.style.Padding.Top
		v.diagram.Draw(p, ui.Rect{X: x, Y: y + pad, W: u.w, H: u.h - 2*pad}, p.Theme())
	}
	if v.label != "" && !v.reveal && v.panel && v.info != nil {
		sp := e.styleSpan(v.info)
		sp.Text = v.label
		w, _ := p.MeasureText(0, sp)
		p.RichText(x+u.w-w-e.lk.size*0.6, y+e.lk.size*0.35, 0, sp)
	}
}

// fillRange fills the part of the source range [a, z) that shows in a
// unit.
func (e *Editor) fillRange(p *ui.Painter, u *leaf, v *leafView, a, z int, lx, ly float32, col ui.Color) {
	lo, hi := u.lineStart, u.end
	if z < lo || a > hi || v.empty {
		return
	}
	da := v.disp.looseDisp(max(a, lo) - u.lineStart)
	dz := v.disp.looseDisp(min(z, hi) - u.lineStart)
	if dz <= da {
		return
	}
	for _, r := range v.layout.SelectionRects(ui.TextInputRange{Start: da, End: dz}) {
		r.X += lx
		r.Y += ly
		p.Fill(r, col, 2)
	}
}

// looseDisp returns the display offset of any source offset (relative): of
// the piece holding it, or the edge of the next piece.
func (d *display) looseDisp(rel int) int {
	for i := range d.pieces {
		pc := &d.pieces[i]
		if rel > pc.s1 {
			continue
		}
		if rel <= pc.s0 {
			return pc.d0
		}
		if pc.kind == pText {
			return pc.d0 + ui.UTF16Len(pc.text[:min(rel-pc.s0, len(pc.text))])
		}
		return pc.d1
	}
	if len(d.pieces) == 0 {
		return 0
	}
	return d.pieces[len(d.pieces)-1].d1
}

// paintObjects draws the images and formulas inside a layout's lines.
func (e *Editor) paintObjects(p *ui.Painter, v *leafView, lx, ly float32) {
	if !v.disp.hasObjects {
		return
	}
	for _, o := range v.layout.Objects() {
		for i := range v.disp.pieces {
			pc := &v.disp.pieces[i]
			if pc.obj != nil && pc.d0 == o.Index {
				r := o.Rect
				r.X += lx
				r.Y += ly
				e.paintObject(p, pc.obj, r)
				break
			}
		}
	}
}

// paintObject draws an image or a formula in r, or a placeholder for an
// image not loaded.
func (e *Editor) paintObject(p *ui.Painter, o *object, r ui.Rect) {
	if o.bmp != nil {
		p.Image(o.bmp, r, ui.Contain)
		return
	}
	p.Fill(r, e.lk.panel, 6)
	p.Stroke(r, e.lk.border, 6, 1)
	label := "🖼 " + o.alt
	if o.alt == "" {
		label = "🖼 图片"
	}
	sp := ui.Span{Text: label, Size: e.lk.size * 0.8, Color: e.lk.muted}
	w, h := p.MeasureText(r.W-16, sp)
	p.Clip(r, 6, func() {
		p.RichText(r.X+max((r.W-w)/2, 8), r.Y+(r.H-h)/2, r.W-16, sp)
	})
}

// paintMarkers draws the markers of the items a leaf opens: bullets,
// numbers, check boxes, and a footnote's label.
func (e *Editor) paintMarkers(p *ui.Painter, l *leaf, ox, oy float32) {
	if len(l.firstOf) == 0 {
		return
	}
	baseline := l.y + e.lk.size*1.1
	if v := l.view; v != nil && v.layout != nil {
		if lines := v.layout.Lines(); len(lines) > 0 {
			baseline = l.y + v.ty + lines[0].Baseline
		}
	}
	x := e.colX
	st := e.styledLeaf(l)
	for k, c := range l.conts {
		cs := st.contStyle[k]
		if slices.Contains(l.firstOf, k) {
			switch c.kind {
			case cItem:
				e.paintItemMarker(p, c, st.conts[k], ox+x, oy+baseline, cs.Padding.Left)
			case cFootnote:
				sp := e.styleSpan(st.conts[k].Pseudo("label", 0))
				sp.Text = "[^" + c.label + "]"
				fm := ui.Font{Family: sp.Font, Size: sp.Size}.Metrics()
				p.RichText(ox+x, oy+baseline-fm.Ascent, cs.Padding.Left, sp)
			}
		}
		x += cs.Padding.Left
	}
}

// paintItemMarker draws an item's marker in the column from x, w wide, on
// the baseline.
func (e *Editor) paintItemMarker(p *ui.Painter, c cont, item *theme.Node, x, baseline, w float32) {
	s := item.Style().FontSize
	if c.task != parser.NoTask {
		cs := item.Pseudo("checkbox", 0)
		box := cs.Width
		if box <= 0 {
			box = cs.Height
		}
		if box <= 0 {
			box = s * 0.95
		}
		r := ui.Rect{X: x + w - box - cs.Margin.Right, Y: baseline - s*0.82, W: box, H: box}
		if c.task == parser.Checked {
			p.Fill(r, uiColor(cs.Background), cs.Radius)
			col := uiColor(cs.Color)
			if col.A == 0 {
				col = ui.RGB(255, 255, 255)
			}
			path := ui.Path{}
			path.MoveTo(r.X+r.W*0.24, r.Y+r.H*0.52)
			path.LineTo(r.X+r.W*0.43, r.Y+r.H*0.7)
			path.LineTo(r.X+r.W*0.77, r.Y+r.H*0.32)
			p.StrokePath(&path, 1.8, col)
		} else if b := cs.Border[0]; b.Width > 0 {
			p.Stroke(r, uiColor(b.Color), cs.Radius, b.Width)
		}
		return
	}
	marker := item.Pseudo("marker", 0)
	text := "•"
	size := marker.FontSize
	if c.ordered {
		text = strconv.Itoa(c.num) + string(c.marker)
	} else {
		list := item.Parent().Style().ListStyle
		kind := list.Kind
		if kind == theme.ListAuto {
			switch c.depth % 3 {
			case 1:
				kind = theme.ListCircle
			case 2:
				kind = theme.ListSquare
			default:
				kind = theme.ListDisc
			}
		}
		switch kind {
		case theme.ListNone:
			return
		case theme.ListCircle:
			text = "◦"
		case theme.ListSquare:
			text = "▪"
			size *= 0.8
		case theme.ListString:
			if list.Text != "" {
				text = list.Text
			}
		}
	}
	sp := e.styleSpan(marker)
	sp.Text = text
	sp.Size = size
	fm := ui.Font{Family: sp.Font, Size: size}.Metrics()
	tw, _ := p.MeasureText(0, sp)
	p.RichText(x+w-tw-marker.Margin.Right, baseline-fm.Ascent, 0, sp)
}

// paintTable draws a table's grid and its header row's fill.
func (e *Editor) paintTable(p *ui.Painter, l *leaf, ox, oy float32, t *ui.Theme) {
	g := l.geom
	if g == nil {
		return
	}
	x, y := ox+l.x, oy+l.y
	if len(g.rowY) > 1 {
		if cs := e.tableRowNode(l, 0).Style(); cs.Background.A != 0 {
			p.Fill(ui.Rect{X: x, Y: y, W: g.w, H: g.rowY[1]}, uiColor(cs.Background), 0)
		}
	}
	cs := e.leafStyle(l)
	line := uiColor(cs.Border[0].Color)
	lw := cs.Border[0].Width
	if lw <= 0 {
		lw = 1
	}
	for _, ry := range g.rowY {
		p.Fill(ui.Rect{X: x, Y: y + ry - lw/2, W: g.w, H: lw}, line, 0)
	}
	h := g.rowY[len(g.rowY)-1]
	for _, cx := range g.colX {
		p.Fill(ui.Rect{X: x + cx - lw/2, Y: y, W: lw, H: h}, line, 0)
	}
}

// paintCaret draws the caret, blinking, while the editor has the focus.
func (e *Editor) paintCaret(p *ui.Painter, ox, oy float32) {
	if !e.focused || e.anchor != e.caret && e.compose == "" {
		return
	}
	phase := p.Now().Sub(e.blinkStart)
	if phase < 30*time.Second {
		p.After(blink - phase%blink)
	}
	if e.compose == "" && phase%(2*blink) >= blink {
		return
	}
	r, ok := e.composeCaretRect()
	if !ok {
		r, ok = e.caretRect(e.caret)
	}
	if !ok {
		return
	}
	h := r.H
	p.Fill(ui.Rect{X: ox + r.X - 0.75, Y: oy + r.Y, W: 1.75, H: h}, e.lk.caret, 0.5)
}

// composeCaretRect returns the caret's box inside the composition of an
// input method.
func (e *Editor) composeCaretRect() (ui.Rect, bool) {
	if e.compose == "" {
		return ui.Rect{}, false
	}
	us := e.units()
	i := e.unitIndex(us, e.caret)
	if i < 0 {
		return ui.Rect{}, false
	}
	u := us[i]
	v := u.view
	if v == nil || v.layout == nil {
		return ui.Rect{}, false
	}
	for _, pc := range v.disp.pieces {
		if pc.kind == pDeco && pc.text == e.compose && pc.s0+u.lineStart == e.caret {
			r := v.layout.Caret(pc.d0 + min(e.composeSel.End, pc.d1-pc.d0))
			r.X += u.x + v.tx
			r.Y += u.y + v.ty
			return r, true
		}
	}
	return ui.Rect{}, false
}
