package syntax

import "testing"

func FuzzParse(f *testing.F) {
	seeds := []string{
		"",
		"a",
		"# h\n",
		"**a** *b* ~~c~~ `d` $e$\n",
		"```go\nx\n```\n",
		"| a | b |\n|---|--:|\n| 1 | 2 |\n",
		"<b>x</b> <u>y</u> <span>z</span>\n",
		"> - [x] **a**\n",
		"---\na: 1\n---\nbody\n",
		"[x]: http://a\n\n[x]\n",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		if len(src) > 4096 {
			t.Skip()
		}
		tr := Parse(src)
		if tr.Source() != src {
			t.Fatalf("source: %q", tr.Source())
		}
		for n := range tr.Walk() {
			r := n.Range()
			if r.Start < 0 || r.End > len(src) || r.End < r.Start {
				t.Fatalf("%q: %v out of the source", src, n)
			}
			if p, ok := n.Parent(); ok {
				pr := p.Range()
				if r.Start < pr.Start || r.End > pr.End {
					t.Fatalf("%q: %v out of parent %v", src, n, p)
				}
			}
		}
		for _, off := range []int{0, len(src) / 2, len(src)} {
			ps := tr.PathAt(off)
			if len(ps) == 0 || ps[0].Kind() != Document {
				t.Fatalf("%q: path at %d: %v", src, off, ps)
			}
			for _, n := range ps {
				r := n.Range()
				inside := off >= r.Start && off < r.End
				if n.Kind() != Syntax {
					inside = off >= r.Start && off <= r.End
				}
				if !inside {
					t.Fatalf("%q: path at %d holds %v", src, off, n)
				}
			}
		}
	})
}
