package theme

import (
	"math"
	"strconv"
	"strings"
)

// Color is a color of 8 bits a channel, not premultiplied, as MyGo's
// ui.Color is.
type Color struct{ R, G, B, A uint8 }

// Alpha returns the color with its alpha times a, rounded as MyGo's
// ui.Color.Alpha rounds it.
func (c Color) Alpha(a float32) Color {
	c.A = alphaByte(float32(c.A) / 255 * a)
	return c
}

// alphaByte returns an alpha between 0 and 1 as a byte, as MyGo rounds it.
func alphaByte(a float32) uint8 {
	if a <= 0 {
		return 0
	}
	if a >= 1 {
		return 255
	}
	return uint8(a*255 + 0.5)
}

// colorValue is a color as written, which currentColor, the element's
// color, may be part of.
type colorValue struct {
	c       Color
	current bool
	mix     *colorMix
}

// colorMix is color-mix(in srgb, a pa, b pb), its alpha times mult.
type colorMix struct {
	a, b   colorValue
	pa, pb float32
	mult   float32
}

// resolve returns the color, with cur as currentColor.
func (v colorValue) resolve(cur Color) Color {
	switch {
	case v.current:
		return cur
	case v.mix != nil:
		m := v.mix
		return mix(m.a.resolve(cur), m.pa, m.b.resolve(cur), m.pb).Alpha(m.mult)
	}
	return v.c
}

// mix mixes two colors in sRGB with premultiplied alpha, as color-mix()
// does.
func mix(a Color, pa float32, b Color, pb float32) Color {
	aa := float32(a.A)/255*pa + float32(b.A)/255*pb
	if aa <= 0 {
		return Color{}
	}
	ch := func(x, y uint8) uint8 {
		v := (float32(x)*float32(a.A)/255*pa + float32(y)*float32(b.A)/255*pb) / aa
		return uint8(min(max(v+0.5, 0), 255))
	}
	return Color{R: ch(a.R, b.R), G: ch(a.G, b.G), B: ch(a.B, b.B), A: alphaByte(aa)}
}

// parseColor reads a color at the parser.
func parseColor(p *vparser) (colorValue, bool) {
	t := p.next()
	switch t.kind {
	case tHash:
		c, ok := hexColor(t.val)
		return colorValue{c: c}, ok
	case tIdent:
		name := strings.ToLower(t.val)
		switch name {
		case "currentcolor":
			return colorValue{current: true}, true
		case "transparent":
			return colorValue{}, true
		}
		c, ok := namedColors[name]
		return colorValue{c: c}, ok
	case tFunction:
		args, ok := p.args()
		if !ok {
			return colorValue{}, false
		}
		switch t.val {
		case "rgb", "rgba":
			return rgbColor(args)
		case "hsl", "hsla":
			return hslColor(args)
		case "color-mix":
			return colorMixValue(args)
		}
	}
	return colorValue{}, false
}

// hexColor reads #rgb, #rgba, #rrggbb or #rrggbbaa, without its "#".
func hexColor(h string) (Color, bool) {
	switch len(h) {
	case 3, 4:
		var b strings.Builder
		for i := range len(h) {
			b.WriteByte(h[i])
			b.WriteByte(h[i])
		}
		h = b.String()
	case 6, 8:
	default:
		return Color{}, false
	}
	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return Color{}, false
	}
	if len(h) == 6 {
		v = v<<8 | 0xff
	}
	return Color{R: uint8(v >> 24), G: uint8(v >> 16), B: uint8(v >> 8), A: uint8(v)}, true
}

// channelArgs splits the arguments of rgb() or hsl(), written with commas
// or with spaces and a "/" before the alpha, into three channels and an
// alpha, nil when there is none.
func channelArgs(args [][]token) (ch [3]token, alpha *token, ok bool) {
	var vals []token
	slash := -1
	if len(args) == 1 {
		for _, t := range args[0] {
			switch {
			case t.kind == tWhitespace:
			case t.is('/'):
				slash = len(vals)
			default:
				vals = append(vals, t)
			}
		}
	} else {
		for _, a := range args {
			a = trimWS(a)
			if len(a) != 1 {
				return ch, nil, false
			}
			vals = append(vals, a[0])
		}
		if len(vals) == 4 {
			slash = 3
		}
	}
	switch {
	case len(vals) == 3 && slash < 0:
	case len(vals) == 4 && slash == 3:
		alpha = &vals[3]
	default:
		return ch, nil, false
	}
	copy(ch[:], vals)
	return ch, alpha, true
}

// alphaValue reads an alpha: a number from 0 to 1, or a percentage.
func alphaValue(t *token) (float32, bool) {
	if t == nil {
		return 1, true
	}
	switch t.kind {
	case tNumber:
		return float32(min(max(t.num, 0), 1)), true
	case tPercentage:
		return float32(min(max(t.num/100, 0), 1)), true
	}
	return 0, false
}

func rgbColor(args [][]token) (colorValue, bool) {
	ch, at, ok := channelArgs(args)
	if !ok {
		return colorValue{}, false
	}
	var c [3]uint8
	for i, t := range ch {
		var v float64
		switch t.kind {
		case tNumber:
			v = t.num
		case tPercentage:
			v = t.num * 255 / 100
		default:
			return colorValue{}, false
		}
		c[i] = uint8(math.Round(min(max(v, 0), 255)))
	}
	a, ok := alphaValue(at)
	if !ok {
		return colorValue{}, false
	}
	return colorValue{c: Color{R: c[0], G: c[1], B: c[2], A: alphaByte(a)}}, true
}

func hslColor(args [][]token) (colorValue, bool) {
	ch, at, ok := channelArgs(args)
	if !ok {
		return colorValue{}, false
	}
	var h float64
	switch t := ch[0]; {
	case t.kind == tNumber:
		h = t.num
	case t.kind == tDimension && t.val == "deg":
		h = t.num
	case t.kind == tDimension && t.val == "turn":
		h = t.num * 360
	case t.kind == tDimension && t.val == "rad":
		h = t.num * 180 / math.Pi
	default:
		return colorValue{}, false
	}
	pct := func(t token) (float64, bool) {
		if t.kind != tPercentage && t.kind != tNumber {
			return 0, false
		}
		return min(max(t.num/100, 0), 1), true
	}
	s, ok1 := pct(ch[1])
	l, ok2 := pct(ch[2])
	a, ok3 := alphaValue(at)
	if !ok1 || !ok2 || !ok3 {
		return colorValue{}, false
	}
	h = math.Mod(math.Mod(h, 360)+360, 360) / 360
	f := func(n float64) uint8 {
		k := math.Mod(n+h*12, 12)
		v := l - s*min(l, 1-l)*max(-1, min(k-3, 9-k, 1))
		return uint8(math.Round(v * 255))
	}
	return colorValue{c: Color{R: f(0), G: f(8), B: f(4), A: alphaByte(a)}}, true
}

// colorMixValue reads the arguments of color-mix(in srgb, a [p], b [p]).
func colorMixValue(args [][]token) (colorValue, bool) {
	if len(args) != 3 {
		return colorValue{}, false
	}
	in := trimWS(args[0])
	if len(in) != 3 || !in[0].isIdent("in") || !in[2].isIdent("srgb") {
		return colorValue{}, false
	}
	side := func(ts []token) (colorValue, float32, bool, bool) {
		p := &vparser{ts: trimWS(ts)}
		var pct float32
		has := false
		if t := p.peek(); t.kind == tPercentage {
			pct, has = float32(t.num/100), true
			p.next()
		}
		c, ok := parseColor(p)
		if !ok {
			return c, 0, false, false
		}
		if t := p.peek(); !has && t.kind == tPercentage {
			pct, has = float32(t.num/100), true
			p.next()
		}
		return c, pct, has, p.done()
	}
	a, pa, hasA, okA := side(args[1])
	b, pb, hasB, okB := side(args[2])
	if !okA || !okB || pa < 0 || pb < 0 || pa > 1 || pb > 1 {
		return colorValue{}, false
	}
	switch {
	case !hasA && !hasB:
		pa, pb = 0.5, 0.5
	case !hasB:
		pb = 1 - pa
	case !hasA:
		pa = 1 - pb
	}
	sum := pa + pb
	if sum <= 0 {
		return colorValue{}, false
	}
	m := &colorMix{a: a, b: b, pa: pa, pb: pb, mult: 1}
	if sum != 1 {
		m.pa, m.pb = pa/sum, pb/sum
		if sum < 1 {
			m.mult = sum
		}
	}
	if !a.current && a.mix == nil && !b.current && b.mix == nil {
		return colorValue{c: mix(a.c, m.pa, b.c, m.pb).Alpha(m.mult)}, true
	}
	return colorValue{mix: m}, true
}
