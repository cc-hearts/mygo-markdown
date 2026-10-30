package mdoc

import "testing"

func TestNormalizeTeX(t *testing.T) {
	cases := map[string]string{
		`x^2 + x_i^2 \le y \ge z \ne w`: `x^2 + x_i^2 \leq y \geq z \neq w`,
		`\text{if } x>0`:                `if  x>0`,
		`\left( x \right)`:              `( x )`,
		`\mathbb{R}`:                    `R`,
		`\mathrm{abc}`:                  `abc`,
	}
	for in, want := range cases {
		if got := normalizeTeX(in); got != want {
			t.Fatalf("normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWrapperCommandsRender(t *testing.T) {
	for _, e := range []string{`\text{if } x>0`, `\left( x \right)`, `\mathbb{R}`, `\mathrm{abc}`} {
		img, err := renderMathImage(e, 18)
		if img == nil {
			t.Fatalf("render %q: %v", e, err)
		}
	}
}
