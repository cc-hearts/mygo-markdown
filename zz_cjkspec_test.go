package livemark

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/egoist/mygo/ui/markdown/parser"
)

// TestZZMaskSpec checks that the mask changes no spec example's parse.
func TestZZMaskSpec(t *testing.T) {
	path := os.Getenv("MDSPEC")
	if path == "" {
		t.Skip()
	}
	b, _ := os.ReadFile(path)
	var ex []zzSpec
	json.Unmarshal(b, &ex)
	n := 0
	for _, x := range ex {
		d1 := parser.New(exts)
		d1.Set(x.Markdown)
		d2 := parser.New(exts)
		d2.Set(cjkMask(x.Markdown))
		if h1, h2 := d1.HTML(), d2.HTML(); h1 != h2 {
			n++
			fmt.Printf("#%d %q\n  plain %q\n  mask  %q\n", x.Example, x.Markdown, h1, h2)
		}
	}
	fmt.Println("diffs:", n)
}
