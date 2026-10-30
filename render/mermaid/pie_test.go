package mermaid

import "testing"

func TestParsePie(t *testing.T) {
	g, ok := parsePie(`pie title 流量
"Dogs" : 386
"Cats" : 85
"Rats" : 15
`)
	if !ok || g.title != "流量" || len(g.slices) != 3 || g.slices[0].label != "Dogs" || g.slices[0].value != 386 {
		t.Fatalf("pie = %+v", g)
	}
}
