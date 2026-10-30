package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/CarlOpenLab/livemark/syntax"
)

// dumpTree writes a syntax tree as the panel shows it: one line a node,
// indented under its parent, with its element, its source range, and the
// syntax it hides.
func dumpTree(t *syntax.Tree) string {
	var b strings.Builder
	var walk func(n syntax.Node, depth int)
	walk = func(n syntax.Node, depth int) {
		r := n.Range()
		fmt.Fprintf(&b, "%s%s [%d, %d)", strings.Repeat("  ", depth), n.Element().String(), r.Start, r.End)
		if syn := n.Syntax(); len(syn) > 0 {
			b.WriteString("  syntax=")
			for i, s := range syn {
				if i > 0 {
					b.WriteString(",")
				}
				b.WriteString(quoted(t.Source()[s.Start:s.End]))
			}
		}
		b.WriteByte('\n')
		for c := range n.Children() {
			walk(c, depth+1)
		}
	}
	walk(t.Root(), 0)
	return b.String()
}

// quoted writes src as a string literal, cut to at most 24 characters.
func quoted(src string) string {
	r := []rune(src)
	if len(r) > 24 {
		return strconv.Quote(string(r[:24]) + "…")
	}
	return strconv.Quote(src)
}
