package math

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"sync"

	"git.sr.ht/~sbinet/gg"
	"github.com/egoist/mygo/ui"
	"github.com/go-latex/latex/drawtex"
	"github.com/go-latex/latex/font/ttf"
	"github.com/go-latex/latex/mtex"
	"github.com/go-latex/latex/tex"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

// mathDPI is the rasterisation density; the bitmap is displayed at 72/DPI of
// its pixel size, so formulas stay sharp without being giant.
const mathDPI = 192.0

// typeset is a formula rasterized: its pixels, and its baseline in pixels
// from their top, which text around an inline formula lines up with.
type typeset struct {
	img      *image.RGBA
	baseline int
}

type mathRender struct {
	bmp      *ui.Bitmap
	baseline float32
}

var (
	mathCache sync.Map
	mathMu    sync.Mutex
)

// black is the ink of a formula rendered plain, as the tests check.
var black = ui.RGB(0, 0, 0)

// mathBitmap typesets a TeX expression with go-latex's engine (the same
// family of MathJax/KaTeX-style layout used by Typora), in color. A cache
// keeps a formula from being re-rasterised each frame.
func mathBitmap(expr string, size float64, color ui.Color) *ui.Bitmap {
	return mathRendered(expr, size, color).bmp
}

// mathRendered is mathBitmap with the formula's baseline.
func mathRendered(expr string, size float64, color ui.Color) *mathRender {
	key := fmt.Sprintf("%s|%.1f|%02x%02x%02x%02x", expr, size, color.R, color.G, color.B, color.A)
	if v, ok := mathCache.Load(key); ok {
		return v.(*mathRender)
	}
	mathMu.Lock()
	defer mathMu.Unlock()
	if v, ok := mathCache.Load(key); ok {
		return v.(*mathRender)
	}
	r := &mathRender{}
	var img image.Image
	var err error
	if m, ok := parseMatrix(expr); ok {
		img, err = m.render(size)
		if err == nil {
			r.baseline = float32(img.Bounds().Dy())/2 + float32(size*mathDPI/72)*0.3
		}
	} else {
		var ts typeset
		ts, err = typesetMath(expr, size)
		img, r.baseline = ts.img, float32(ts.baseline)
	}
	if err == nil {
		img = tintImage(img, color)
		r.bmp = ui.NewBitmap(img)
	}
	mathCache.Store(key, r)
	return r
}

// renderMathImage typesets expr at size (in points), as an image.
func renderMathImage(expr string, size float64) (image.Image, error) {
	ts, err := typesetMath(expr, size)
	if err != nil {
		return nil, err
	}
	return ts.img, nil
}

// typesetMath typesets expr at size (in points): with go-latex, or where it
// cannot lay out scripts, as on \int_0^1, with Muse's own composition of
// the base and its scripts.
func typesetMath(expr string, size float64) (typeset, error) {
	expr = normalizeTeX(expr)
	ts, err := typesetTeX(expr, size)
	if err == nil {
		return ts, nil
	}
	if strings.ContainsAny(expr, "^_") {
		if script, serr := typesetScripts(expr, size); serr == nil {
			return script, nil
		}
	}
	return typeset{}, err
}

// typesetTeX lays expr out with go-latex and rasterizes what it draws. The
// image is as large as the ink of every glyph and rule, rather than the
// box go-latex reports, which leaves out parts that hang below it.
func typesetTeX(expr string, size float64) (ts typeset, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("math panic: %v", r)
		}
	}()
	canvas := drawtex.New()
	box, err := mtex.Parse("$"+expr+"$", size, 72, ttf.New(canvas))
	if err != nil {
		return typeset{}, err
	}
	tree, ok := box.(tex.Tree)
	if !ok {
		return typeset{}, fmt.Errorf("math: %q laid out no tree", expr)
	}
	var sh tex.Ship
	sh.Call(0, 0, tree)

	// The ink, in points, with the box's own extent as the least of it.
	h := box.Height()
	minX, minY := 0.0, 0.0
	maxX, maxY := box.Width(), h+math.Max(box.Depth(), 0)
	for _, op := range canvas.Ops() {
		switch op := op.(type) {
		case drawtex.GlyphOp:
			m := op.Glyph.Metrics
			minX, maxX = math.Min(minX, op.X+m.XMin), math.Max(maxX, op.X+m.XMax)
			minY, maxY = math.Min(minY, op.Y-m.YMax), math.Max(maxY, op.Y-m.YMin)
		case drawtex.RectOp:
			minX, maxX = math.Min(minX, math.Min(op.X1, op.X2)), math.Max(maxX, math.Max(op.X1, op.X2))
			minY, maxY = math.Min(minY, math.Min(op.Y1, op.Y2)), math.Max(maxY, math.Max(op.Y1, op.Y2))
		}
	}
	const pad = 1.0
	s := mathDPI / 72
	w := int(math.Ceil((maxX - minX + 2*pad) * s))
	hpx := int(math.Ceil((maxY - minY + 2*pad) * s))
	if w <= 0 || hpx <= 0 {
		return typeset{}, fmt.Errorf("math: %q is empty", expr)
	}
	ctx := gg.NewContext(w, hpx)
	ctx.SetColor(color.Black)
	dx, dy := pad-minX, pad-minY
	for _, op := range canvas.Ops() {
		switch op := op.(type) {
		case drawtex.GlyphOp:
			face, err := opentype.NewFace(op.Glyph.Font, &opentype.FaceOptions{
				DPI:     mathDPI,
				Size:    op.Glyph.Size,
				Hinting: font.HintingNone,
			})
			if err != nil {
				return typeset{}, err
			}
			ctx.SetFontFace(face)
			ctx.DrawString(op.Glyph.Symbol, (op.X+dx)*s, (op.Y+dy)*s)
			face.Close()
		case drawtex.RectOp:
			x0, x1 := math.Min(op.X1, op.X2), math.Max(op.X1, op.X2)
			y0, y1 := math.Min(op.Y1, op.Y2), math.Max(op.Y1, op.Y2)
			ctx.DrawRectangle((x0+dx)*s, (y0+dy)*s, (x1-x0)*s, math.Max(y1-y0, 0.5)*s)
			ctx.Fill()
		}
	}
	img, ok := ctx.Image().(*image.RGBA)
	if !ok {
		return typeset{}, fmt.Errorf("math: unexpected image %T", ctx.Image())
	}
	return typeset{img: img, baseline: int(math.Round((h + dy) * s))}, nil
}

// tintImage draws the black ink of src in c, over the alpha its glyphs
// were rasterized with: the antialiased edges take the color properly,
// premultiplied.
func tintImage(src image.Image, c ui.Color) image.Image {
	if c.R == 0 && c.G == 0 && c.B == 0 && c.A == 255 {
		return src
	}
	b := src.Bounds()
	dst := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := src.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			al := uint32(a>>8) * uint32(c.A) / 255
			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(uint32(c.R) * al / 255),
				G: uint8(uint32(c.G) * al / 255),
				B: uint8(uint32(c.B) * al / 255),
				A: uint8(al),
			})
		}
	}
	return dst
}

// Renderer typesets formulas, keeping each rasterizer output by its TeX,
// its size and its color.
type Renderer struct{}

// New returns a renderer of formulas.
func New() *Renderer { return &Renderer{} }

// Math typesets tex at size in color, as MathRenderer does. display tells
// a formula on its own lines, which the caller sizes.
func (r *Renderer) Math(tex string, display bool, size float32, color ui.Color) (*ui.Bitmap, float32, bool) {
	tex = strings.TrimSpace(tex)
	if tex == "" {
		return nil, 0, false
	}
	m := mathRendered(tex, float64(size), color)
	if m.bmp == nil {
		return nil, 0, false
	}
	return m.bmp, m.baseline, true
}
