package theme

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// propID is a longhand property.
type propID uint8

const (
	pColor propID = iota
	pFontFamily
	pFontSize
	pFontWeight
	pFontStyle
	pFontFeatureSettings
	pFontVariantNumeric
	pLetterSpacing
	pLineHeight
	pTextAlign
	pCaretColor
	pListStyleType
	pTextDecorationLine
	pTextDecorationStyle
	pTextDecorationColor
	pTextDecorationThickness
	pVerticalAlign
	pOpacity
	pBackgroundColor
	pBorderRadius
	pPaddingTop
	pPaddingRight
	pPaddingBottom
	pPaddingLeft
	pMarginTop
	pMarginRight
	pMarginBottom
	pMarginLeft
	pBorderTopWidth
	pBorderRightWidth
	pBorderBottomWidth
	pBorderLeftWidth
	pBorderTopStyle
	pBorderRightStyle
	pBorderBottomStyle
	pBorderLeftStyle
	pBorderTopColor
	pBorderRightColor
	pBorderBottomColor
	pBorderLeftColor
	pMaxWidth
	pWidth
	pHeight
	numProps
)

// cssWide is inherit, initial or unset, which every property takes.
type cssWide uint8

const (
	kwInherit cssWide = iota + 1
	kwInitial
	kwUnset
)

// catNowhere marks a property no element uses.
const catNowhere elemCat = 1 << 7

// propDef is a longhand property: whether it inherits, its initial value,
// how its value reads, and where it applies (everywhere when 0).
type propDef struct {
	name      string
	inherited bool
	initial   string
	parse     parseFn
	cats      elemCat
	init      any
}

// parseFn reads a value at the parser. It returns nil for a value it
// cannot read, with why when it can say; a value it reads only in part
// comes with a note.
type parseFn func(p *vparser) (v any, msg string)

// shortDef is a shorthand property: the longhands it sets.
type shortDef struct {
	subs   []propID
	expand func(p *vparser) (map[propID]any, string)
}

// setting is what a declaration sets: a longhand and its value.
type setting struct {
	id propID
	v  any
}

// Values of properties.
type (
	// lineHeight is normal, a multiple of the font size, or a length.
	lineHeight struct {
		normal bool
		l      length
	}
	// decoLine is the lines of text-decoration-line.
	decoLine struct{ under, through bool }
	// autoLength is auto, none or a length.
	autoLength struct {
		auto bool
		l    length
	}
	// vAlign is baseline, super, sub or a length.
	vAlign struct {
		kind byte // 'b', 'p' for super, 's' for sub, 'l' for a length
		l    length
	}
	// borderStyle is whether a border shows.
	borderStyle bool
	// italic is font-style.
	italic bool
	// tabular is font-variant-numeric: tabular-nums.
	tabular bool
	// wavy is text-decoration-style: wavy.
	wavy bool
	// opacity is opacity.
	opacity float32
	// weight is font-weight.
	weight int
	// features is font-feature-settings, as MyGo takes it.
	features string
	// families is a font-family list.
	families []string
)

// Align is how text aligns.
type Align uint8

const (
	AlignStart Align = iota
	AlignLeft
	AlignCenter
	AlignRight
	AlignEnd
)

// ListStyle is the marker of list items.
type ListStyle struct {
	Kind ListKind
	// Text is the marker of KindString.
	Text string
}

// ListKind is a sort of list marker.
type ListKind uint8

const (
	// ListAuto turns bullets from discs to circles to squares as lists
	// nest, and numbers ordered lists.
	ListAuto ListKind = iota
	ListDisc
	ListCircle
	ListSquare
	ListDecimal
	ListNone
	ListString
)

var props [numProps]propDef

var propIDs = map[string]propID{}

var shorthands = map[string]shortDef{}

// propNames are the names of every property, for suggestions.
var propNames []string

func init() {
	colorFn := func(p *vparser) (any, string) {
		if c, ok := parseColor(p); ok {
			return c, ""
		}
		return nil, ""
	}
	lengthFn := func(pct, negative bool) parseFn {
		return func(p *vparser) (any, string) {
			l, ok := parseLength(p, pct, false)
			if !ok {
				return nil, ""
			}
			if !negative && l.op == 0 && l.num < 0 {
				return nil, "it cannot be negative"
			}
			return l, ""
		}
	}
	boxCats := catRoot | catBlock | catContainer
	borderCats := catBlock | catContainer | catPseudo | catObject
	def := func(id propID, name string, inherited bool, initial string, parse parseFn, cats elemCat) {
		props[id] = propDef{name: name, inherited: inherited, initial: initial, parse: parse, cats: cats}
		propIDs[name] = id
	}
	def(pColor, "color", true, "#000", colorFn, 0)
	def(pFontFamily, "font-family", true, "system-ui", parseFamilies, 0)
	def(pFontSize, "font-size", true, "16px", lengthFn(true, false), 0)
	def(pFontWeight, "font-weight", true, "400", parseWeight, 0)
	def(pFontStyle, "font-style", true, "normal", keywords(map[string]any{"normal": italic(false), "italic": italic(true), "oblique": italic(true)}), 0)
	def(pFontFeatureSettings, "font-feature-settings", true, "normal", parseFeatures, 0)
	def(pFontVariantNumeric, "font-variant-numeric", true, "normal", parseNumeric, 0)
	def(pLetterSpacing, "letter-spacing", true, "normal", parseLetterSpacing, 0)
	def(pLineHeight, "line-height", true, "1.75", parseLineHeight, 0)
	def(pTextAlign, "text-align", true, "start", keywords(map[string]any{"start": AlignStart, "left": AlignLeft, "center": AlignCenter, "right": AlignRight, "end": AlignEnd}), 0)
	def(pCaretColor, "caret-color", true, "auto", func(p *vparser) (any, string) {
		if p.peek().isIdent("auto") {
			p.next()
			return colorValue{current: true}, ""
		}
		return colorFn(p)
	}, 0)
	def(pListStyleType, "list-style-type", true, "auto", parseListStyle, catContainer)
	def(pTextDecorationLine, "text-decoration-line", false, "none", parseDecoLine, 0)
	def(pTextDecorationStyle, "text-decoration-style", false, "solid", keywords(map[string]any{"solid": wavy(false), "wavy": wavy(true)}), 0)
	def(pTextDecorationColor, "text-decoration-color", false, "currentcolor", colorFn, 0)
	def(pTextDecorationThickness, "text-decoration-thickness", false, "auto", autoLengthFn("auto"), 0)
	def(pVerticalAlign, "vertical-align", false, "baseline", parseVAlign, catInline)
	def(pOpacity, "opacity", false, "1", parseOpacity, 0)
	def(pBackgroundColor, "background-color", false, "transparent", colorFn, 0)
	def(pBorderRadius, "border-radius", false, "0", lengthFn(false, false), 0)
	for i, side := range [4]string{"top", "right", "bottom", "left"} {
		def(pPaddingTop+propID(i), "padding-"+side, false, "0", lengthFn(false, false), boxCats)
		marginCats := catBlock | catContainer
		switch side {
		case "right":
			marginCats = catPseudo
		case "left":
			marginCats = catNowhere
		}
		def(pMarginTop+propID(i), "margin-"+side, false, "0", parseMargin, marginCats)
		def(pBorderTopWidth+propID(i), "border-"+side+"-width", false, "medium", parseBorderWidth, borderCats)
		def(pBorderTopStyle+propID(i), "border-"+side+"-style", false, "none", parseBorderStyle, borderCats)
		def(pBorderTopColor+propID(i), "border-"+side+"-color", false, "currentcolor", colorFn, borderCats)
	}
	def(pMaxWidth, "max-width", false, "820px", autoLengthFn("none"), catRoot)
	def(pWidth, "width", false, "auto", autoLengthFn("auto"), catBlock|catPseudo)
	def(pHeight, "height", false, "auto", autoLengthFn("auto"), catBlock|catPseudo)

	for i := range props {
		d := &props[i]
		v, msg := d.parse(&vparser{ts: tokenize(d.initial, &diagnostics{})})
		if v == nil {
			panic(fmt.Sprintf("theme: initial value of %s: %s", d.name, msg))
		}
		d.init = v
	}

	sides := func(first propID) []propID { return []propID{first, first + 1, first + 2, first + 3} }
	shorthands["background"] = shortDef{subs: []propID{pBackgroundColor}, expand: func(p *vparser) (map[propID]any, string) {
		if p.peek().isIdent("none") {
			p.next()
			return map[propID]any{pBackgroundColor: colorValue{}}, ""
		}
		c, ok := parseColor(p)
		if !ok {
			return nil, "background takes a color alone"
		}
		return map[propID]any{pBackgroundColor: c}, ""
	}}
	shorthands["padding"] = shortDef{subs: sides(pPaddingTop), expand: boxSides(sides(pPaddingTop), props[pPaddingTop].parse)}
	shorthands["margin"] = shortDef{subs: sides(pMarginTop), expand: boxSides(sides(pMarginTop), parseMargin)}
	shorthands["border-width"] = shortDef{subs: sides(pBorderTopWidth), expand: boxSides(sides(pBorderTopWidth), parseBorderWidth)}
	shorthands["border-style"] = shortDef{subs: sides(pBorderTopStyle), expand: boxSides(sides(pBorderTopStyle), parseBorderStyle)}
	shorthands["border-color"] = shortDef{subs: sides(pBorderTopColor), expand: boxSides(sides(pBorderTopColor), props[pBorderTopColor].parse)}
	all := slices.Concat(sides(pBorderTopWidth), sides(pBorderTopStyle), sides(pBorderTopColor))
	shorthands["border"] = shortDef{subs: all, expand: borderSides(0, 1, 2, 3)}
	for i, side := range [4]string{"top", "right", "bottom", "left"} {
		shorthands["border-"+side] = shortDef{
			subs:   []propID{pBorderTopWidth + propID(i), pBorderTopStyle + propID(i), pBorderTopColor + propID(i)},
			expand: borderSides(i),
		}
	}
	shorthands["text-decoration"] = shortDef{
		subs:   []propID{pTextDecorationLine, pTextDecorationStyle, pTextDecorationColor, pTextDecorationThickness},
		expand: parseTextDecoration,
	}
	shorthands["list-style"] = shortDef{subs: []propID{pListStyleType}, expand: func(p *vparser) (map[propID]any, string) {
		v, msg := parseListStyle(p)
		if v == nil {
			return nil, msg
		}
		return map[propID]any{pListStyleType: v}, msg
	}}

	propNames = slices.Sorted(maps.Keys(propIDs))
	for name := range shorthands {
		propNames = append(propNames, name)
	}
	slices.Sort(propNames)
}

// keywords returns a parseFn of the identifiers of m.
func keywords(m map[string]any) parseFn {
	return func(p *vparser) (any, string) {
		t := p.next()
		if t.kind == tIdent {
			if v, ok := m[strings.ToLower(t.val)]; ok {
				return v, ""
			}
		}
		return nil, ""
	}
}

// autoLengthFn returns a parseFn of a length, or of the keyword kw.
func autoLengthFn(kw string) parseFn {
	return func(p *vparser) (any, string) {
		if p.peek().isIdent(kw) {
			p.next()
			return autoLength{auto: true}, ""
		}
		l, ok := parseLength(p, false, false)
		if !ok {
			return nil, ""
		}
		return autoLength{l: l}, ""
	}
}

var genericFamilies = map[string]bool{
	"serif": true, "sans-serif": true, "monospace": true, "system-ui": true, "cursive": true,
	"fantasy": true, "ui-monospace": true, "ui-sans-serif": true, "ui-serif": true, "ui-rounded": true,
}

func parseFamilies(p *vparser) (any, string) {
	var out families
	for _, part := range splitTop(p.ts[p.i:], tComma) {
		part = trimWS(part)
		switch {
		case len(part) == 1 && part[0].kind == tString:
			out = append(out, part[0].val)
		case len(part) > 0:
			var words []string
			for _, t := range part {
				switch t.kind {
				case tWhitespace:
				case tIdent:
					words = append(words, t.val)
				default:
					return nil, "quote a font name that is not plain words"
				}
			}
			name := strings.Join(words, " ")
			if genericFamilies[strings.ToLower(name)] {
				name = strings.ToLower(name)
			}
			out = append(out, name)
		default:
			return nil, "empty font name"
		}
	}
	p.i = len(p.ts)
	if len(out) == 0 {
		return nil, ""
	}
	return out, ""
}

func parseWeight(p *vparser) (any, string) {
	t := p.next()
	switch {
	case t.isIdent("normal"):
		return weight(400), ""
	case t.isIdent("bold"):
		return weight(700), ""
	case t.isIdent("bolder") || t.isIdent("lighter"):
		return nil, "bolder and lighter are not supported: give a weight"
	case t.kind == tNumber && t.num >= 1 && t.num <= 1000:
		return weight(int(t.num)), ""
	}
	return nil, ""
}

func parseFeatures(p *vparser) (any, string) {
	if p.peek().isIdent("normal") {
		p.next()
		return features(""), ""
	}
	var out []string
	for _, part := range splitTop(p.ts[p.i:], tComma) {
		q := &vparser{ts: part}
		tag := q.next()
		if tag.kind != tString || len(tag.val) != 4 {
			return nil, "a feature is a quoted tag of 4 letters, as \"tnum\""
		}
		v := 1
		switch t := q.next(); {
		case t.kind == tEOF, t.isIdent("on"):
		case t.isIdent("off"):
			v = 0
		case t.kind == tNumber && t.num >= 0 && t.num == float64(int(t.num)):
			v = int(t.num)
		default:
			return nil, ""
		}
		if !q.done() {
			return nil, ""
		}
		switch v {
		case 0:
			out = append(out, "-"+tag.val)
		case 1:
			out = append(out, tag.val)
		default:
			out = append(out, tag.val+"="+strconv.Itoa(v))
		}
	}
	p.i = len(p.ts)
	return features(strings.Join(out, ",")), ""
}

func parseNumeric(p *vparser) (any, string) {
	t := p.next()
	switch {
	case t.isIdent("normal"), t.isIdent("proportional-nums"):
		return tabular(false), ""
	case t.isIdent("tabular-nums"):
		return tabular(true), ""
	case t.kind == tIdent:
		return nil, "font-variant-numeric takes normal or tabular-nums"
	}
	return nil, ""
}

func parseLetterSpacing(p *vparser) (any, string) {
	if p.peek().isIdent("normal") {
		p.next()
		return length{}, ""
	}
	if l, ok := parseLength(p, false, false); ok {
		return l, ""
	}
	return nil, ""
}

func parseLineHeight(p *vparser) (any, string) {
	if p.peek().isIdent("normal") {
		p.next()
		return lineHeight{normal: true}, ""
	}
	l, ok := parseLength(p, true, true)
	if !ok || l.op == 0 && l.num < 0 {
		return nil, ""
	}
	return lineHeight{l: l}, ""
}

func parseListStyle(p *vparser) (any, string) {
	t := p.next()
	if t.kind == tString {
		return ListStyle{Kind: ListString, Text: t.val}, ""
	}
	kinds := map[string]ListKind{"auto": ListAuto, "disc": ListDisc, "circle": ListCircle, "square": ListSquare, "decimal": ListDecimal, "none": ListNone}
	if t.kind == tIdent {
		if k, ok := kinds[strings.ToLower(t.val)]; ok {
			return ListStyle{Kind: k}, ""
		}
		return nil, "list markers are auto, disc, circle, square, decimal, none or a string"
	}
	return nil, ""
}

func parseDecoLine(p *vparser) (any, string) {
	var d decoLine
	if p.peek().isIdent("none") {
		p.next()
		return d, ""
	}
	any := false
	for !p.done() {
		t := p.peek()
		switch {
		case t.isIdent("underline") && !d.under:
			d.under = true
		case t.isIdent("line-through") && !d.through:
			d.through = true
		case t.isIdent("overline") || t.isIdent("blink"):
			return nil, t.val + " is not supported"
		default:
			if !any {
				return nil, ""
			}
			return d, ""
		}
		p.next()
		any = true
	}
	if !any {
		return nil, ""
	}
	return d, ""
}

func parseVAlign(p *vparser) (any, string) {
	t := p.peek()
	switch {
	case t.isIdent("baseline"):
		p.next()
		return vAlign{kind: 'b'}, ""
	case t.isIdent("super"):
		p.next()
		return vAlign{kind: 'p'}, ""
	case t.isIdent("sub"):
		p.next()
		return vAlign{kind: 's'}, ""
	case t.kind == tIdent:
		return nil, "vertical-align takes baseline, super, sub or a length"
	}
	l, ok := parseLength(p, false, false)
	if !ok {
		return nil, ""
	}
	return vAlign{kind: 'l', l: l}, ""
}

func parseOpacity(p *vparser) (any, string) {
	switch t := p.next(); t.kind {
	case tNumber:
		return opacity(min(max(t.num, 0), 1)), ""
	case tPercentage:
		return opacity(min(max(t.num/100, 0), 1)), ""
	}
	return nil, ""
}

func parseMargin(p *vparser) (any, string) {
	if p.peek().isIdent("auto") {
		return nil, "auto margins are not supported"
	}
	if l, ok := parseLength(p, false, false); ok {
		return l, ""
	}
	return nil, ""
}

func parseBorderWidth(p *vparser) (any, string) {
	if t := p.peek(); t.kind == tIdent {
		widths := map[string]float64{"thin": 1, "medium": 3, "thick": 5}
		if w, ok := widths[strings.ToLower(t.val)]; ok {
			p.next()
			return length{num: w, unit: "px"}, ""
		}
		return nil, ""
	}
	l, ok := parseLength(p, false, false)
	if !ok || l.op == 0 && l.num < 0 {
		return nil, ""
	}
	return l, ""
}

func parseBorderStyle(p *vparser) (any, string) {
	t := p.next()
	if t.kind != tIdent {
		return nil, ""
	}
	switch s := strings.ToLower(t.val); s {
	case "none", "hidden":
		return borderStyle(false), ""
	case "solid":
		return borderStyle(true), ""
	case "dashed", "dotted", "double", "groove", "ridge", "inset", "outset":
		return borderStyle(true), "borders are drawn solid: " + s + " is not supported"
	}
	return nil, ""
}

// boxSides returns the expander of a shorthand of one to four values, for
// the top, the right, the bottom and the left, as padding is.
func boxSides(ids []propID, parse parseFn) func(p *vparser) (map[propID]any, string) {
	return func(p *vparser) (map[propID]any, string) {
		var vals []any
		note := ""
		for !p.done() && len(vals) < 5 {
			v, msg := parse(p)
			if v == nil {
				return nil, msg
			}
			if msg != "" {
				note = msg
			}
			vals = append(vals, v)
		}
		var top, right, bottom, left any
		switch len(vals) {
		case 1:
			top, right, bottom, left = vals[0], vals[0], vals[0], vals[0]
		case 2:
			top, right, bottom, left = vals[0], vals[1], vals[0], vals[1]
		case 3:
			top, right, bottom, left = vals[0], vals[1], vals[2], vals[1]
		case 4:
			top, right, bottom, left = vals[0], vals[1], vals[2], vals[3]
		default:
			return nil, "it takes one to four values"
		}
		return map[propID]any{ids[0]: top, ids[1]: right, ids[2]: bottom, ids[3]: left}, note
	}
}

// borderSides returns the expander of border, or of the border of one
// side: a width, a style and a color, in any order, each at most once.
func borderSides(sides ...int) func(p *vparser) (map[propID]any, string) {
	return func(p *vparser) (map[propID]any, string) {
		var w, s, c any
		note := ""
		for !p.done() {
			at := p.i
			if w == nil {
				if v, _ := parseBorderWidth(p); v != nil {
					w = v
					continue
				}
				p.i = at
			}
			if s == nil {
				if v, msg := parseBorderStyle(p); v != nil {
					s, note = v, msg
					continue
				}
				p.i = at
			}
			if c == nil {
				if v, ok := parseColor(p); ok {
					c = v
					continue
				}
				p.i = at
			}
			return nil, ""
		}
		if w == nil && s == nil && c == nil {
			return nil, ""
		}
		w, s, c = cmp(w, props[pBorderTopWidth].init), cmp(s, props[pBorderTopStyle].init), cmp(c, props[pBorderTopColor].init)
		out := map[propID]any{}
		for _, i := range sides {
			out[pBorderTopWidth+propID(i)] = w
			out[pBorderTopStyle+propID(i)] = s
			out[pBorderTopColor+propID(i)] = c
		}
		return out, note
	}
}

// cmp returns v, or def when v is nil: the initial value of a component a
// shorthand leaves out.
func cmp(v, def any) any {
	if v == nil {
		return def
	}
	return v
}

func parseTextDecoration(p *vparser) (map[propID]any, string) {
	var line, style, color, thick any
	for !p.done() {
		at := p.i
		if line == nil {
			if v, _ := parseDecoLine(p); v != nil {
				line = v
				continue
			}
			p.i = at
		}
		if style == nil {
			if v, _ := props[pTextDecorationStyle].parse(p); v != nil {
				style = v
				continue
			}
			p.i = at
		}
		if color == nil {
			if v, ok := parseColor(p); ok {
				color = v
				continue
			}
			p.i = at
		}
		if thick == nil {
			if v, _ := props[pTextDecorationThickness].parse(p); v != nil {
				thick = v
				continue
			}
			p.i = at
		}
		return nil, ""
	}
	return map[propID]any{
		pTextDecorationLine:      cmp(line, props[pTextDecorationLine].init),
		pTextDecorationStyle:     cmp(style, props[pTextDecorationStyle].init),
		pTextDecorationColor:     cmp(color, props[pTextDecorationColor].init),
		pTextDecorationThickness: cmp(thick, props[pTextDecorationThickness].init),
	}, ""
}

// parseDecl reads the value of a declaration of a property, a longhand or
// a shorthand, into what it sets; ok is false for a property or a value it
// cannot read, and msg says why, or notes what it reads only in part.
func parseDecl(prop string, val []token) (sets []setting, msg string, ok bool) {
	short, isShort := shorthands[prop]
	id, isLong := propIDs[prop]
	if !isShort && !isLong {
		return nil, "", false
	}
	if len(val) == 1 && val[0].kind == tIdent {
		kw := map[string]cssWide{"inherit": kwInherit, "initial": kwInitial, "unset": kwUnset, "revert": kwUnset}[strings.ToLower(val[0].val)]
		if kw != 0 {
			if isLong {
				return []setting{{id, kw}}, "", true
			}
			for _, sub := range short.subs {
				sets = append(sets, setting{sub, kw})
			}
			return sets, "", true
		}
	}
	p := &vparser{ts: val}
	if isLong {
		v, msg := props[id].parse(p)
		if v == nil || !p.done() {
			return nil, msg, false
		}
		return []setting{{id, v}}, msg, true
	}
	m, msg := short.expand(p)
	if m == nil || !p.done() {
		return nil, msg, false
	}
	for _, sub := range short.subs {
		if v, ok := m[sub]; ok {
			sets = append(sets, setting{sub, v})
		}
	}
	return sets, msg, true
}

// applies reports whether a property applies to an element of category
// cat: a property no element uses applies nowhere.
func (d *propDef) applies(cat elemCat) bool {
	if d.inherited || d.cats == 0 {
		return true
	}
	return d.cats&cat != 0
}
