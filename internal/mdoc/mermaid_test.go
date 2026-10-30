package mdoc

import "testing"

func TestParseMermaidFlowchart(t *testing.T) {
	g, ok := parseMermaid(`graph TD
A[Start] --> B{Ready?}
B -->|yes| C[Ship]
B -.-> D[Retry]
`)
	if !ok {
		t.Fatal("parse failed")
	}
	if g.dir != "TD" || len(g.nodes) != 4 || len(g.edges) != 3 {
		t.Fatalf("graph = dir %s nodes %d edges %d", g.dir, len(g.nodes), len(g.edges))
	}
	if g.byID["A"].label != "Start" || g.byID["B"].label != "Ready?" {
		t.Fatalf("labels = %q %q", g.byID["A"].label, g.byID["B"].label)
	}
	if g.edges[1].label != "yes" || !g.edges[2].dashed {
		t.Fatalf("edges = %+v", g.edges)
	}
}
