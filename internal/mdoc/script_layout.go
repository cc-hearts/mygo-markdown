package mdoc

import (
	"image"
	"image/draw"
	"strings"
)

// go-latex parses the AST for sup/sub but does not lay out every case, as
// the limits of \int. When it fails on an expression containing ^ or _,
// Muse splits the expression at the first top-level script and composes the
// base, its scripts and the text after them itself, on one baseline.
func typesetScripts(expr string, size float64) (typeset, error) {
	base, sup, sub, rest, ok := splitScript(expr)
	if !ok || strings.TrimSpace(base) == "" {
		return typeset{}, errNoScripts
	}
	b, err := typesetMath(base, size)
	if err != nil {
		return typeset{}, err
	}
	em := size * mathDPI / 72
	var parts []placed
	parts = append(parts, placed{ts: b})
	x := b.img.Bounds().Dx()
	scriptW := 0
	ascent := b.baseline
	depth := b.img.Bounds().Dy() - b.baseline
	if sup != "" {
		s, err := typesetMath(sup, size*0.7)
		if err != nil {
			return typeset{}, err
		}
		// Raised by at least half an em, more for a tall base, as \int.
		shift := max(int(em*0.42), ascent-s.baseline*2/3)
		parts = append(parts, placed{ts: s, x: x + 1, rise: shift})
		scriptW = max(scriptW, s.img.Bounds().Dx()+1)
	}
	if sub != "" {
		s, err := typesetMath(sub, size*0.7)
		if err != nil {
			return typeset{}, err
		}
		// Lowered by a fifth of an em, more under a deep base.
		shift := max(int(em*0.22), depth-(s.img.Bounds().Dy()-s.baseline)/2)
		parts = append(parts, placed{ts: s, x: x + 1, rise: -shift})
		scriptW = max(scriptW, s.img.Bounds().Dx()+1)
	}
	x += scriptW
	if trimmed := strings.TrimLeft(rest, " "); strings.TrimSpace(trimmed) != "" {
		r, err := typesetMath(trimmed, size)
		if err != nil {
			return typeset{}, err
		}
		if len(trimmed) < len(rest) {
			x += int(em * 0.25)
		}
		parts = append(parts, placed{ts: r, x: x + int(em*0.1)})
	}
	return compose(parts), nil
}

// placed is a part of a formula composed by Muse: its typesetting, where it
// starts across, and how far its baseline is raised above the formula's.
type placed struct {
	ts   typeset
	x    int
	rise int
}

// compose draws parts on one baseline, into an image as large as all of
// them.
func compose(parts []placed) typeset {
	top, bottom, right := 0, 0, 0
	for _, p := range parts {
		b := p.ts.img.Bounds()
		// Relative to the common baseline at 0: the part's top and bottom.
		top = min(top, -p.rise-p.ts.baseline)
		bottom = max(bottom, -p.rise-p.ts.baseline+b.Dy())
		right = max(right, p.x+b.Dx())
	}
	dst := image.NewRGBA(image.Rect(0, 0, right, bottom-top))
	for _, p := range parts {
		b := p.ts.img.Bounds()
		y := -p.rise - p.ts.baseline - top
		draw.Draw(dst, image.Rect(p.x, y, p.x+b.Dx(), y+b.Dy()), p.ts.img, b.Min, draw.Over)
	}
	return typeset{img: dst, baseline: -top}
}

func splitScript(expr string) (base, sup, sub, rest string, ok bool) {
	depth := 0
	for i := 0; i < len(expr); i++ {
		switch expr[i] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case '^', '_':
			if depth != 0 {
				continue
			}
			base = expr[:i]
			j := i
			for j < len(expr) && (expr[j] == '^' || expr[j] == '_') {
				marker := expr[j]
				content, next := scriptContent(expr, j+1)
				if content == "" {
					return "", "", "", "", false
				}
				if marker == '^' {
					sup = content
				} else {
					sub = content
				}
				j = next
			}
			rest = expr[j:]
			return base, sup, sub, rest, true
		}
	}
	return "", "", "", "", false
}

func scriptContent(expr string, i int) (content string, next int) {
	if i >= len(expr) {
		return "", i
	}
	if expr[i] == '{' {
		depth := 0
		for j := i; j < len(expr); j++ {
			switch expr[j] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return expr[i+1 : j], j + 1
				}
			}
		}
		return "", i
	}
	return string(expr[i]), i + 1
}

type noScriptsError struct{}

func (noScriptsError) Error() string { return "no top-level scripts" }

var errNoScripts = noScriptsError{}
