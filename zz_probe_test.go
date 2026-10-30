package livemark

import (
	"fmt"
	"os"
	"testing"

	"github.com/egoist/mygo/ui/markdown/parser"
)

func TestZZProbe(t *testing.T) {
	src := os.Getenv("MDPROBE")
	if src == "" {
		t.Skip()
	}
	d := parser.New(exts)
	d.Set(src)
	for i := 1; i < d.Len(); i++ {
		b := d.Block(i)
		fmt.Printf("block %d %v [%d,%d) parent=%d\n", i, b.Kind, b.Start, b.End, b.Parent)
		for k := 0; k < d.LineCount(i); k++ {
			txt, off := d.Line(i, k)
			fmt.Printf("   line %d off=%d %q\n", k, off, txt)
		}
		for j, in := range d.Inlines(i) {
			fmt.Printf("   in %d %v [%d,%d) next=%d text=%q src=%q\n", j, in.Kind, in.Start, in.End, in.Next, in.Text, src[in.Start:in.End])
		}
	}
}
