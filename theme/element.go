package theme

import (
	"slices"
	"strconv"
	"strings"
)

// Element is a node of a document as a theme sees it, which selectors
// match: its type ("heading"), its classes ("h2"), its attributes
// (level="2") and the states it is in (:revealed).
type Element struct {
	Type    string
	Classes []string
	Attrs   []Attr
	States  States
}

// Attr is an attribute of an element.
type Attr struct{ Name, Value string }

// States are the pseudo-classes an element is in.
type States uint8

const (
	// FirstChild and LastChild tell an element's place among its
	// siblings.
	FirstChild States = 1 << iota
	LastChild
	// Revealed tells a construct whose syntax shows, as the caret is in
	// it.
	Revealed
	// Broken tells an image that did not load, or a formula that could
	// not be typeset.
	Broken
	// Current tells the chosen match of a search, of ::search-text.
	Current
)

var stateNames = [...]struct {
	s    States
	name string
}{
	{FirstChild, "first-child"},
	{LastChild, "last-child"},
	{Revealed, "revealed"},
	{Broken, "broken"},
	{Current, "current"},
}

// HasClass reports whether the element has class c.
func (e Element) HasClass(c string) bool { return slices.Contains(e.Classes, c) }

// Attr returns the value of the element's attribute name.
func (e Element) Attr(name string) (string, bool) {
	for _, a := range e.Attrs {
		if a.Name == name {
			return a.Value, true
		}
	}
	return "", false
}

// String returns the element as a selector of it, as an inspector shows
// it: code.block[lang="go"]:revealed.
func (e Element) String() string {
	var sb strings.Builder
	sb.WriteString(e.Type)
	for _, c := range e.Classes {
		sb.WriteByte('.')
		sb.WriteString(c)
	}
	for _, a := range e.Attrs {
		sb.WriteByte('[')
		sb.WriteString(a.Name)
		sb.WriteByte('=')
		sb.WriteString(strconv.Quote(a.Value))
		sb.WriteByte(']')
	}
	for _, st := range stateNames {
		if e.States&st.s != 0 {
			sb.WriteByte(':')
			sb.WriteString(st.name)
		}
	}
	return sb.String()
}

// inline reports whether the element is inline content, whose background
// and baseline its inline descendants share.
func (e Element) inline() bool {
	t, ok := elementTypes[e.Type]
	if !ok {
		return false
	}
	if t.cat&catBlock != 0 && t.cat&catInline != 0 {
		return e.HasClass("inline")
	}
	return t.cat&catInline != 0
}
