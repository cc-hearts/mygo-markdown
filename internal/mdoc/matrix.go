package mdoc

import (
	"image"
	"image/color"
	"image/draw"
	"strings"
)

type matrixSpec struct {
	kind string
	rows [][]string
}

// parseMatrix recognises a whole display expression that is one TeX matrix
// environment. go-latex does not implement these, so Muse lays the cells out
// itself and draws the delimiters.
func parseMatrix(expr string) (matrixSpec, bool) {
	s := strings.TrimSpace(expr)
	for _, kind := range []string{"pmatrix", "bmatrix", "Bmatrix", "vmatrix", "Vmatrix", "matrix", "cases", "aligned", "align", "align*", "array", "gathered", "split"} {
		open := `\begin{` + kind + `}`
		close := `\end{` + kind + `}`
		if !strings.HasPrefix(s, open) || !strings.HasSuffix(s, close) {
			continue
		}
		body := strings.TrimSpace(s[len(open) : len(s)-len(close)])
		if kind == "array" && strings.HasPrefix(body, "{") {
			if end := strings.IndexByte(body, '}'); end >= 0 {
				body = strings.TrimSpace(body[end+1:])
			}
		}
		if body == "" {
			return matrixSpec{}, false
		}
		var rows [][]string
		for _, raw := range strings.Split(body, `\\`) {
			raw = strings.TrimSpace(raw)
			// Optional row spacing such as \\[2pt].
			if strings.HasPrefix(raw, "[") {
				if end := strings.IndexByte(raw, ']'); end >= 0 {
					raw = strings.TrimSpace(raw[end+1:])
				}
			}
			if raw == "" {
				continue
			}
			cells := strings.Split(raw, "&")
			for i := range cells {
				cells[i] = strings.TrimSpace(cells[i])
			}
			rows = append(rows, cells)
		}
		if len(rows) == 0 {
			return matrixSpec{}, false
		}
		return matrixSpec{kind: kind, rows: rows}, true
	}
	return matrixSpec{}, false
}

func (m matrixSpec) render(size float64) (image.Image, error) {
	type cell struct {
		img image.Image
	}
	cols := 0
	for _, row := range m.rows {
		if len(row) > cols {
			cols = len(row)
		}
	}
	grid := make([][]cell, len(m.rows))
	colW := make([]int, cols)
	rowH := make([]int, len(m.rows))
	for r, row := range m.rows {
		grid[r] = make([]cell, cols)
		for c := 0; c < cols; c++ {
			if c >= len(row) || row[c] == "" {
				continue
			}
			img, err := renderMathImage(row[c], size)
			if err != nil {
				continue
			}
			grid[r][c] = cell{img: img}
			if w := img.Bounds().Dx(); w > colW[c] {
				colW[c] = w
			}
			if h := img.Bounds().Dy(); h > rowH[r] {
				rowH[r] = h
			}
		}
	}

	const (
		margin   = 10
		gapX     = 16
		gapY     = 10
		bracketW = 10
	)
	width := 2*margin + 2*bracketW + gapX*max(0, cols-1)
	for _, w := range colW {
		width += w
	}
	height := 2*margin + gapY*max(0, len(m.rows)-1)
	for _, h := range rowH {
		height += h
	}
	if width < 24 {
		width = 24
	}
	if height < 24 {
		height = 24
	}
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	black := color.RGBA{A: 255}
	x := margin + bracketW
	for c := 0; c < cols; c++ {
		y := margin
		for r := range m.rows {
			if img := grid[r][c].img; img != nil {
				dx := x + (colW[c]-img.Bounds().Dx())/2
				if leftAligned(m.kind) {
					dx = x
				}
				dy := y + (rowH[r]-img.Bounds().Dy())/2
				draw.Draw(dst, image.Rect(dx, dy, dx+img.Bounds().Dx(), dy+img.Bounds().Dy()), img, image.Point{}, draw.Over)
			}
			y += rowH[r] + gapY
		}
		x += colW[c] + gapX
	}

	top, bottom := margin, height-margin
	left, right := margin, width-margin
	// Delimiters.
	switch m.kind {
	case "cases":
		drawBrace(dst, left, top, bottom, black)
	case "aligned", "align", "align*", "array", "gathered", "split":
		// aligned/array environments have no delimiters of their own.
	case "vmatrix":
		rect(dst, left, top, left+2, bottom, black)
		rect(dst, right-2, top, right, bottom, black)
	case "Vmatrix":
		rect(dst, left, top, left+2, bottom, black)
		rect(dst, left+5, top, left+7, bottom, black)
		rect(dst, right-7, top, right-5, bottom, black)
		rect(dst, right-2, top, right, bottom, black)
	case "matrix":
	default:
		// Parentheses / brackets: vertical rules plus short inward serifs.
		rect(dst, left+3, top, left+5, bottom, black)
		rect(dst, right-5, top, right-3, bottom, black)
		rect(dst, left+3, top, left+9, top+2, black)
		rect(dst, left+3, bottom-2, left+9, bottom, black)
		rect(dst, right-9, top, right-3, top+2, black)
		rect(dst, right-9, bottom-2, right-3, bottom, black)
	}
	return dst, nil
}

func leftAligned(kind string) bool {
	switch kind {
	case "cases", "aligned", "align", "align*", "array", "gathered", "split":
		return true
	}
	return false
}

func drawBrace(dst *image.RGBA, left, top, bottom int, c color.RGBA) {
	mid := (top + bottom) / 2
	rect(dst, left+4, top, left+6, bottom, c)
	rect(dst, left+4, top, left+10, top+2, c)
	rect(dst, left+4, bottom-2, left+10, bottom, c)
	rect(dst, left, mid-1, left+6, mid+1, c)
}

func rect(dst *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	draw.Draw(dst, image.Rect(x0, y0, x1, y1), &image.Uniform{C: c}, image.Point{}, draw.Src)
}
