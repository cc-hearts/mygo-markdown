package livemark

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/livemark/internal/mdoc"
)

// mathScale turns the pixels of a typeset formula into DIPs: the renderer
// rasterizes at 192 DPI, twice the density of a DIP.
const mathScale = 72.0 / 192.0

// mathObject returns the typeset formula of tex at the size of the text
// around it, as an inline object, or nil when it cannot be typeset.
func (e *Editor) mathObject(tex string, size float32, display bool, color ui.Color) *object {
	tex = strings.TrimSpace(tex)
	if tex == "" || e.opts.Math == nil {
		return nil
	}
	if display {
		size *= 1.15
	}
	bmp, baseline, ok := e.opts.Math.Math(tex, display, size, color)
	if !ok || bmp == nil {
		return nil
	}
	w, h := bmp.Size()
	fw, fh := float32(w)*mathScale, float32(h)*mathScale
	// The formula's baseline sits on the text's.
	ascent := min(baseline*mathScale, fh)
	return &object{bmp: bmp, w: fw + 2, h: ascent, descent: fh - ascent}
}

// mathBlockObject returns the typeset formula of a block of math.
func (e *Editor) mathBlockObject(tex string, width, size float32, color ui.Color) *object {
	tex = strings.TrimSpace(tex)
	if tex == "" || e.opts.Math == nil {
		return nil
	}
	bmp, _, ok := e.opts.Math.Math(tex, true, size, color)
	if !ok || bmp == nil {
		return nil
	}
	w, h := bmp.Size()
	fw, fh := float32(w)*mathScale, float32(h)*mathScale
	if width > 0 && fw > width {
		fh = fh * width / fw
		fw = width
	}
	return &object{bmp: bmp, w: fw, h: fh, block: true}
}

// loadImage returns the image src names, with the app's loader or the
// built-in one.
func (e *Editor) loadImage(src string, onLoad func()) *ui.Bitmap {
	if e.opts.LoadImage != nil {
		return e.opts.LoadImage(src, onLoad)
	}
	return mdoc.LoadImage(src, e.opts.BaseDir, onLoad)
}

// imageObject returns the image at dest as an inline object, scaled down
// to the width; a placeholder showing alt until it loads. alone makes it a
// line of its own: a paragraph of just an image.
func (e *Editor) imageObject(dest, alt string, width float32, alone bool) *object {
	bmp := e.loadImage(dest, e.assetLoaded)
	obj := &object{bmp: bmp, src: dest, alt: alt, block: alone}
	if bmp == nil {
		obj.w = min(max(width, 80), 260)
		obj.h = e.baseSize() * 2.4
		obj.descent = obj.h * 0.25
		return obj
	}
	w, h := bmp.Size()
	fw, fh := float32(w), float32(h)
	if width > 0 && fw > width {
		fh = fh * width / fw
		fw = width
	}
	obj.w, obj.h = fw, fh
	return obj
}

// htmlImageObject returns the image an HTML block of just an image shows.
func (e *Editor) htmlImageObject(html string, width float32) *object {
	src, alt, _, iw, ih, ok := mdoc.HTMLOnlyImage(html)
	if !ok {
		return nil
	}
	obj := e.imageObject(src, alt, width, true)
	if obj.bmp != nil && (iw > 0 || ih > 0) {
		bw, bh := obj.bmp.Size()
		fw, fh := float32(bw), float32(bh)
		switch {
		case iw > 0 && ih > 0:
			fw, fh = iw, ih
		case iw > 0:
			fh = fh * iw / fw
			fw = iw
		default:
			fw = fw * ih / fh
			fh = ih
		}
		if width > 0 && fw > width {
			fh = fh * width / fw
			fw = width
		}
		obj.w, obj.h = fw, fh
	}
	return obj
}

// assetLoaded is called on another goroutine when an image has loaded:
// the leaves showing it lay out again with its size.
func (e *Editor) assetLoaded() {
	e.assetGen.Add(1)
	if e.opts.Invalidate != nil {
		e.opts.Invalidate()
	}
}
