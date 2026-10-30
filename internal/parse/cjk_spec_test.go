package parse_test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/egoist/mygo/ui/markdown/parser"

	"github.com/CarlOpenLab/livemark/internal/parse"
)

// TestZZMaskSpec checks that the mask changes no spec example's parse.
func TestZZMaskSpec(t *testing.T) {
	path := os.Getenv("MDSPEC")
	if path == "" {
		t.Skip()
	}
	b, _ := os.ReadFile(path)
	var ex []cjkSpec
	json.Unmarshal(b, &ex)
	n := 0
	for _, x := range ex {
		d1 := parser.New(parse.Exts)
		d1.Set(x.Markdown)
		d2 := parser.New(parse.Exts)
		d2.Set(parse.CJKFriendly(x.Markdown))
		if h1, h2 := d1.HTML(), d2.HTML(); h1 != h2 {
			n++
			fmt.Printf("#%d %q\n  plain %q\n  mask  %q\n", x.Example, x.Markdown, h1, h2)
		}
	}
	fmt.Println("diffs:", n)
}

// cjkSpec is an example of the CommonMark specification.
type cjkSpec struct {
	Example  int
	Markdown string
}
