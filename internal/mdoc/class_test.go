package mdoc

import "testing"

func TestParseClassDiagram(t *testing.T) {
	g, ok := parseClass(`classDiagram
class Animal {
  +String name
  +makeSound()
}
class Dog
Animal <|-- Dog
Dog --> Owner : feeds
`)
	if !ok || len(g.classes) != 3 || len(g.relations) != 2 {
		t.Fatalf("graph = %+v", g)
	}
	if g.byName["Animal"].members[1] != "+makeSound()" {
		t.Fatalf("members = %+v", g.byName["Animal"].members)
	}
	if g.relations[0].kind != "inherit" || g.relations[1].label != "feeds" {
		t.Fatalf("relations = %+v", g.relations)
	}
}
