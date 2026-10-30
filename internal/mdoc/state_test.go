package mdoc

import "testing"

func TestParseStateDiagram(t *testing.T) {
	g, ok := parseState(`stateDiagram-v2
[*] --> Idle
Idle --> Running: start
Running --> [*]: stop
`)
	if !ok || len(g.nodes) != 4 || len(g.edges) != 3 {
		t.Fatalf("graph = %+v", g)
	}
	if g.byID["[*]start"].label != "开始" {
		t.Fatalf("start label = %q", g.byID["[*]start"].label)
	}
	if g.edges[1].label != "start" {
		t.Fatalf("edge = %+v", g.edges[1])
	}
}
