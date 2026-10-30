package mdoc

import "testing"

func TestCasesAndAligned(t *testing.T) {
	for _, expr := range []string{
		`\begin{cases} x^2 & x>0 \\ -x & x\le 0 \end{cases}`,
		`\begin{aligned} a &= b + c \\ d &= e \end{aligned}`,
		`\begin{array}{cc} 1 & 2 \\ 3 & 4 \end{array}`,
	} {
		spec, ok := parseMatrix(expr)
		if !ok || len(spec.rows) != 2 {
			t.Fatalf("parse %q = %+v", expr, spec)
		}
		if bmp := mathBitmap(expr, 18, false); bmp == nil {
			t.Fatalf("bitmap for %q is nil", expr)
		}
	}
}
