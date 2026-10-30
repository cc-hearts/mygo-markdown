package theme

import (
	"maps"
	"slices"
	"strings"
)

// Theme is sheets in layers, the built-in default.css first: a later layer
// wins over an earlier one, and an !important declaration over every one
// that is not. A Theme is a value: it does not change.
type Theme struct {
	layers []*Sheet
}

// New returns the theme of sheets on top of default.css.
func New(sheets ...*Sheet) *Theme {
	return &Theme{layers: append([]*Sheet{Default()}, sheets...)}
}

// With returns the theme with one more sheet on top, as the overrides of
// an app's settings.
func (t *Theme) With(s *Sheet) *Theme {
	return &Theme{layers: append(slices.Clip(t.layers), s)}
}

// FontFaces returns the fonts the theme's sheets bring.
func (t *Theme) FontFaces() []FontFace {
	var out []FontFace
	for _, s := range t.layers {
		out = append(out, s.faces...)
	}
	return out
}

// Env is what a theme takes from the window: whether it is dark, and the
// variables --ui-* of its colors and fonts.
type Env struct {
	Dark bool
	Vars map[string]string
}

// Resolver computes the styles of the nodes of documents in a theme and
// an environment. The nodes it hands out keep their styles, so that a
// path styles once: a new theme or environment takes a new Resolver.
type Resolver struct {
	byType map[string][]*crule
	any    []*crule
	vars   varMap
	root   *Node
}

// crule is a selector of a rule, with the rule's declarations and its place
// in the cascade.
type crule struct {
	sel   *selector
	decls []*decl
	layer int
	order int
}

// Resolver returns a resolver of the theme in env.
func (t *Theme) Resolver(env Env) *Resolver {
	r := &Resolver{byType: map[string][]*crule{}, vars: varMap{}}
	order := 0
	for layer, s := range t.layers {
		for _, ru := range s.rules {
			if !ru.media.applies(env.Dark) {
				continue
			}
			for _, sel := range ru.sels {
				cr := &crule{sel: sel, decls: ru.decls, layer: layer, order: order}
				if typ := sel.parts[len(sel.parts)-1].typ; typ != "" {
					r.byType[typ] = append(r.byType[typ], cr)
				} else {
					r.any = append(r.any, cr)
				}
			}
			order++
		}
	}
	for name, v := range env.Vars {
		r.vars[name] = trimWS(tokenize(v, &diagnostics{}))
	}
	r.root = &Node{r: r, el: Element{Type: "document"}}
	return r
}

// Root returns the node of the document.
func (r *Resolver) Root() *Node { return r.root }

// Node is a node of a document, as a path of elements from the document:
// the same path is the same Node, which computes its style once.
type Node struct {
	r       *Resolver
	parent  *Node
	el      Element
	kids    map[string]*Node
	style   *Computed
	pseudos map[pseudoKey]*Computed
}

type pseudoKey struct {
	name   string
	states States
}

// Element returns the node's element.
func (n *Node) Element() Element { return n.el }

// Parent returns the node's parent, nil for the document.
func (n *Node) Parent() *Node { return n.parent }

// Child returns the node of the element el inside n.
func (n *Node) Child(el Element) *Node {
	k := el.String()
	if c, ok := n.kids[k]; ok {
		return c
	}
	if n.kids == nil {
		n.kids = map[string]*Node{}
	}
	c := &Node{r: n.r, parent: n, el: el}
	n.kids[k] = c
	return c
}

// Path returns the elements from the document to n, as an inspector
// shows them.
func (n *Node) Path() string {
	var parts []string
	for a := n; a != nil; a = a.parent {
		parts = append(parts, a.el.String())
	}
	slices.Reverse(parts)
	return strings.Join(parts, " › ")
}

// Style returns the node's style.
func (n *Node) Style() *Computed {
	if n.style == nil {
		var parent *Computed
		if n.parent != nil {
			parent = n.parent.Style()
		}
		n.style = n.r.compute(n, "", 0, parent)
	}
	return n.style
}

// Pseudo returns the style of the node's pseudo-element name in states:
// item::marker, or ::selection of the document.
func (n *Node) Pseudo(name string, states States) *Computed {
	k := pseudoKey{name, states}
	if c, ok := n.pseudos[k]; ok {
		return c
	}
	if n.pseudos == nil {
		n.pseudos = map[pseudoKey]*Computed{}
	}
	c := n.r.compute(n, name, states, n.Style())
	n.pseudos[k] = c
	return c
}

// hit is a declaration that applies to a node, with its place in the
// cascade.
type hit struct {
	d     *decl
	layer int
	spec  specificity
	order int
}

func (a hit) less(b hit) bool {
	switch {
	case a.d.important != b.d.important:
		return b.d.important
	case a.layer != b.layer:
		return a.layer < b.layer
	case a.spec != b.spec:
		return a.spec.less(b.spec)
	}
	return a.order < b.order
}

// matching returns the declarations that apply to node n, or to its
// pseudo-element, weakest first.
func (r *Resolver) matching(n *Node, pseudo string, ps States) []hit {
	var hits []hit
	add := func(rules []*crule) {
		for _, cr := range rules {
			if cr.sel.matches(n, pseudo, ps) {
				for _, d := range cr.decls {
					hits = append(hits, hit{d: d, layer: cr.layer, spec: cr.sel.spec, order: cr.order})
				}
			}
		}
	}
	add(r.byType[n.el.Type])
	add(r.any)
	slices.SortStableFunc(hits, func(a, b hit) int {
		switch {
		case a.less(b):
			return -1
		case b.less(a):
			return 1
		}
		return 0
	})
	return hits
}

// compute returns the style of node n, or of its pseudo-element, inside
// parent, nil for the document.
func (r *Resolver) compute(n *Node, pseudo string, ps States, parent *Computed) *Computed {
	hits := r.matching(n, pseudo, ps)
	c := &Computed{}
	if parent != nil {
		c.vars = parent.vars
	} else {
		c.vars = r.vars
	}
	c.vars = c.vars.with(hits)
	var spec [numProps]any
	for _, h := range hits {
		d := h.d
		if d.custom() {
			continue
		}
		sets := d.sets
		if d.vars {
			ts, ok := substitute(d.val, c.vars.lookup)
			if ok {
				sets, _, ok = parseDecl(d.prop, ts)
			}
			if !ok {
				sets = unsetAll(d.prop)
			}
		}
		for _, s := range sets {
			spec[s.id] = s.v
			c.set |= 1 << s.id
		}
	}
	x := &computer{r: r, c: c, p: parent, spec: &spec, el: n.el, pseudo: pseudo}
	x.run()
	return c
}

// unsetAll returns the settings of unset for every longhand of a property,
// as a declaration its variables make invalid sets.
func unsetAll(prop string) []setting {
	if id, ok := propIDs[prop]; ok {
		return []setting{{id, kwUnset}}
	}
	var out []setting
	for _, id := range shorthands[prop].subs {
		out = append(out, setting{id, kwUnset})
	}
	return out
}

func (m varMap) lookup(name string) ([]token, bool) {
	v, ok := m[name]
	return v, ok
}

// with returns the variables with those the declarations set, the later
// winning, each with its var() replaced; a variable whose var()s refer to
// it again, or to none, is invalid and left out.
func (m varMap) with(hits []hit) varMap {
	own := map[string][]token{}
	for _, h := range hits {
		if h.d.custom() {
			own[h.d.prop] = h.d.val
		}
	}
	if len(own) == 0 {
		return m
	}
	out := maps.Clone(m)
	state := map[string]uint8{}
	var resolve func(name string) ([]token, bool)
	resolve = func(name string) ([]token, bool) {
		v, mine := own[name]
		if !mine {
			return m.lookup(name)
		}
		switch state[name] {
		case 1:
			return nil, false
		case 2:
			return out.lookup(name)
		}
		state[name] = 1
		ts, ok := v, true
		if len(v) == 1 && v[0].kind == tIdent {
			switch strings.ToLower(v[0].val) {
			case "inherit", "unset":
				ts, ok = m.lookup(name)
			case "initial":
				ok = false
			}
		}
		if ok {
			ts, ok = substitute(ts, resolve)
		}
		state[name] = 2
		if ok {
			out[name] = ts
		} else {
			delete(out, name)
		}
		return ts, ok
	}
	for name := range own {
		resolve(name)
	}
	return out
}

// computer computes a node's style from its specified values and its
// parent's style.
type computer struct {
	r      *Resolver
	c      *Computed
	p      *Computed
	spec   *[numProps]any
	el     Element
	pseudo string
}

// pick returns the value of a property: specified, initial, or inherit
// for the parent's.
func (x *computer) pick(id propID) (v any, inherit bool) {
	v = x.spec[id]
	switch v {
	case kwInherit:
		inherit = true
	case kwInitial:
		v = props[id].init
	case kwUnset, nil:
		if props[id].inherited {
			inherit = true
		} else {
			v = props[id].init
		}
	}
	if inherit && x.p == nil {
		return props[id].init, false
	}
	return v, inherit
}

// rem returns the font size rems are of: the document's.
func (x *computer) rem() float64 {
	if x.p == nil {
		return 16
	}
	return float64(x.r.root.Style().FontSize)
}

// lengthOf returns a property's length, ems being of the node's font size.
func (x *computer) lengthOf(v any) float32 {
	l, _ := v.(length)
	return float32(l.eval(lenCtx{em: float64(x.c.FontSize), rem: x.rem()}))
}

func (x *computer) run() {
	c, p := x.c, x.p
	parentSize := float32(16)
	if p != nil {
		parentSize = p.FontSize
	}

	if v, inh := x.pick(pFontSize); inh {
		c.FontSize = p.FontSize
	} else {
		l := v.(length)
		c.FontSize = float32(l.eval(lenCtx{em: float64(parentSize), rem: x.rem(), pct: float64(parentSize)}))
	}

	if v, inh := x.pick(pOpacity); inh {
		c.raw.opacity = p.raw.opacity
	} else {
		c.raw.opacity = float32(v.(opacity))
	}
	c.Opacity = c.raw.opacity
	if p != nil {
		c.Opacity *= p.Opacity
	}

	if v, inh := x.pick(pColor); inh {
		c.raw.color = p.raw.color
	} else {
		cur := Color{A: 255}
		if p != nil {
			cur = p.raw.color
		}
		c.raw.color = v.(colorValue).resolve(cur)
	}
	c.Color = x.final(c.raw.color)
	// colorOf resolves a color property that does not inherit,
	// currentColor being the node's.
	colorOf := func(id propID) Color {
		return x.final(x.nonInherited(id).(colorValue).resolve(c.raw.color))
	}

	if v, inh := x.pick(pFontFamily); inh {
		c.FontFamily = p.FontFamily
	} else {
		c.FontFamily = v.(families)
	}
	if v, inh := x.pick(pFontWeight); inh {
		c.FontWeight = p.FontWeight
	} else {
		c.FontWeight = int(v.(weight))
	}
	if v, inh := x.pick(pFontStyle); inh {
		c.Italic = p.Italic
	} else {
		c.Italic = bool(v.(italic))
	}
	if v, inh := x.pick(pFontFeatureSettings); inh {
		c.raw.features = p.raw.features
	} else {
		c.raw.features = string(v.(features))
	}
	if v, inh := x.pick(pFontVariantNumeric); inh {
		c.raw.tabular = p.raw.tabular
	} else {
		c.raw.tabular = bool(v.(tabular))
	}
	c.FontFeatures = c.raw.features
	if c.raw.tabular && !strings.Contains(","+c.FontFeatures+",", ",tnum,") {
		if c.FontFeatures != "" {
			c.FontFeatures += ","
		}
		c.FontFeatures += "tnum"
	}
	if v, inh := x.pick(pLetterSpacing); inh {
		c.LetterSpacing = p.LetterSpacing
	} else {
		c.LetterSpacing = x.lengthOf(v)
	}
	if v, inh := x.pick(pLineHeight); inh {
		c.raw.lhNormal, c.raw.lhMul, c.raw.lhPx = p.raw.lhNormal, p.raw.lhMul, p.raw.lhPx
	} else {
		lh := v.(lineHeight)
		switch {
		case lh.normal:
			c.raw.lhNormal = true
		case lh.l.numeric():
			c.raw.lhMul = float32(lh.l.eval(lenCtx{}))
		default:
			c.raw.lhPx = float32(lh.l.eval(lenCtx{em: float64(c.FontSize), rem: x.rem(), pct: float64(c.FontSize)}))
		}
	}
	switch {
	case c.raw.lhNormal:
		c.LineHeight = 0
	case c.raw.lhPx > 0 && c.FontSize > 0:
		c.LineHeight = c.raw.lhPx / c.FontSize
	default:
		c.LineHeight = c.raw.lhMul
	}
	if v, inh := x.pick(pTextAlign); inh {
		c.TextAlign = p.TextAlign
	} else {
		c.TextAlign = v.(Align)
	}
	if v, inh := x.pick(pCaretColor); inh {
		c.raw.caret = p.raw.caret
	} else {
		c.raw.caret = v.(colorValue)
	}
	c.CaretColor = x.final(c.raw.caret.resolve(c.raw.color))
	if v, inh := x.pick(pListStyleType); inh {
		c.ListStyle = p.ListStyle
	} else {
		c.ListStyle = v.(ListStyle)
	}

	// The decoration, the background of text and the shift of the
	// baseline pass from inline elements to those inside them, and the
	// decoration from blocks to their text, but not to the syntax shown
	// at the caret, which shows plain, nor to pseudo-elements.
	syntax := x.pseudo == "" && x.el.Type == "syntax"
	inline := x.pseudo == "" && x.el.inline()
	if p != nil && x.pseudo == "" && !syntax {
		c.Decoration = p.Decoration
	}
	if line := x.nonInherited(pTextDecorationLine).(decoLine); line.under || line.through {
		c.Decoration.Underline = c.Decoration.Underline || line.under
		c.Decoration.LineThrough = c.Decoration.LineThrough || line.through
		c.Decoration.Wavy = bool(x.nonInherited(pTextDecorationStyle).(wavy))
		c.Decoration.Color = colorOf(pTextDecorationColor)
		if t := x.nonInherited(pTextDecorationThickness).(autoLength); !t.auto {
			c.Decoration.Thickness = x.lengthOf(t.l)
		} else {
			c.Decoration.Thickness = 0
		}
	}

	c.Background = colorOf(pBackgroundColor)
	c.Radius = x.lengthOf(x.nonInherited(pBorderRadius))
	switch {
	case inline && c.Background.A != 0:
		c.TextBackground, c.TextRadius = c.Background, c.Radius
	case inline && !syntax && p != nil:
		c.TextBackground, c.TextRadius = p.TextBackground, p.TextRadius
	}

	if inline {
		var own float32
		switch va := x.nonInherited(pVerticalAlign).(vAlign); va.kind {
		case 'p':
			own = parentSize * 0.35
		case 's':
			own = -parentSize * 0.2
		case 'l':
			own = x.lengthOf(va.l)
		}
		c.Shift = own
		if p != nil {
			c.Shift += p.Shift
		}
	}

	c.Padding = Edges{
		Top: x.lengthOf(x.nonInherited(pPaddingTop)), Right: x.lengthOf(x.nonInherited(pPaddingRight)),
		Bottom: x.lengthOf(x.nonInherited(pPaddingBottom)), Left: x.lengthOf(x.nonInherited(pPaddingLeft)),
	}
	c.Margin = Edges{
		Top: x.lengthOf(x.nonInherited(pMarginTop)), Right: x.lengthOf(x.nonInherited(pMarginRight)),
		Bottom: x.lengthOf(x.nonInherited(pMarginBottom)), Left: x.lengthOf(x.nonInherited(pMarginLeft)),
	}
	for i := range 4 {
		if bool(x.nonInherited(pBorderTopStyle + propID(i)).(borderStyle)) {
			c.Border[i] = Border{
				Width: x.lengthOf(x.nonInherited(pBorderTopWidth + propID(i))),
				Color: colorOf(pBorderTopColor + propID(i)),
			}
		}
	}
	if m := x.nonInherited(pMaxWidth).(autoLength); !m.auto {
		c.MaxWidth = x.lengthOf(m.l)
	}
	if w := x.nonInherited(pWidth).(autoLength); !w.auto {
		c.Width = x.lengthOf(w.l)
	}
	if h := x.nonInherited(pHeight).(autoLength); !h.auto {
		c.Height = x.lengthOf(h.l)
	}
}

// nonInherited returns the value of a property that does not inherit: as
// specified or initial, or, for inherit, as the parent specified it.
func (x *computer) nonInherited(id propID) any {
	v, inh := x.pick(id)
	if !inh {
		return v
	}
	return x.parentValue(id)
}

// parentValue returns the value the parent's computed style holds of a
// property that does not inherit, as inherit asks for it. The parent's
// computed lengths and colors are turned back into values.
func (x *computer) parentValue(id propID) any {
	p := x.p
	px := func(f float32) length { return length{num: float64(f), unit: "px"} }
	col := func(c Color) colorValue { return colorValue{c: c} }
	switch id {
	case pTextDecorationLine:
		return decoLine{under: p.Decoration.Underline, through: p.Decoration.LineThrough}
	case pTextDecorationStyle:
		return wavy(p.Decoration.Wavy)
	case pTextDecorationColor:
		return col(p.Decoration.Color)
	case pTextDecorationThickness:
		return autoLength{auto: p.Decoration.Thickness == 0, l: px(p.Decoration.Thickness)}
	case pVerticalAlign:
		return vAlign{kind: 'b'}
	case pOpacity:
		return opacity(p.raw.opacity)
	case pBackgroundColor:
		return col(p.Background)
	case pBorderRadius:
		return px(p.Radius)
	case pPaddingTop:
		return px(p.Padding.Top)
	case pPaddingRight:
		return px(p.Padding.Right)
	case pPaddingBottom:
		return px(p.Padding.Bottom)
	case pPaddingLeft:
		return px(p.Padding.Left)
	case pMarginTop:
		return px(p.Margin.Top)
	case pMarginRight:
		return px(p.Margin.Right)
	case pMarginBottom:
		return px(p.Margin.Bottom)
	case pMarginLeft:
		return px(p.Margin.Left)
	case pMaxWidth:
		return autoLength{auto: p.MaxWidth == 0, l: px(p.MaxWidth)}
	case pWidth:
		return autoLength{auto: p.Width == 0, l: px(p.Width)}
	case pHeight:
		return autoLength{auto: p.Height == 0, l: px(p.Height)}
	}
	switch {
	case id >= pBorderTopWidth && id <= pBorderLeftWidth:
		return px(p.Border[id-pBorderTopWidth].Width)
	case id >= pBorderTopStyle && id <= pBorderLeftStyle:
		return borderStyle(p.Border[id-pBorderTopStyle].Width > 0)
	case id >= pBorderTopColor && id <= pBorderLeftColor:
		return col(p.Border[id-pBorderTopColor].Color)
	}
	return props[id].init
}

// final returns a color with the node's opacity.
func (x *computer) final(c Color) Color {
	if x.c.Opacity >= 1 {
		return c
	}
	return c.Alpha(x.c.Opacity)
}
