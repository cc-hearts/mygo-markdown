package theme

import (
	"maps"
	"slices"
)

// elemCat is what sort of element a type is: which properties apply to it.
type elemCat uint8

const (
	catRoot elemCat = 1 << iota
	// catBlock is a block of text or of something drawn: a paragraph, a
	// heading, a block of code, a table and its rows and cells.
	catBlock
	// catContainer is a block holding blocks: a quotation, a list, an
	// item, a footnote.
	catContainer
	catInline
	catPseudo
	// catObject is an inline drawn as a box of its own: an image.
	catObject
)

// elemType is a type of element: its classes, attributes and
// pseudo-elements.
type elemType struct {
	cat     elemCat
	classes []string
	attrs   []string
	pseudos []string
}

// elementTypes are the types of the nodes of a document, as package syntax
// names them. Selectors naming others match nothing, and warn.
var elementTypes = map[string]elemType{
	"document":     {cat: catRoot},
	"paragraph":    {cat: catBlock},
	"heading":      {cat: catBlock, classes: []string{"h1", "h2", "h3", "h4", "h5", "h6", "setext"}, attrs: []string{"level"}},
	"quote":        {cat: catContainer},
	"list":         {cat: catContainer, classes: []string{"ordered", "bullet", "tight", "loose"}, attrs: []string{"start", "depth"}},
	"item":         {cat: catContainer, classes: []string{"task", "checked"}, attrs: []string{"number", "depth"}, pseudos: []string{"marker", "checkbox"}},
	"code":         {cat: catBlock | catInline, classes: []string{"block", "fenced", "indented", "inline", "html"}, attrs: []string{"lang", "info"}, pseudos: []string{"info"}},
	"math":         {cat: catBlock | catInline, classes: []string{"block", "inline", "display"}},
	"html":         {cat: catBlock | catInline, classes: []string{"block", "inline"}, attrs: []string{"tag"}},
	"table":        {cat: catBlock},
	"row":          {cat: catBlock, classes: []string{"header", "odd", "even"}},
	"cell":         {cat: catBlock, classes: []string{"header"}, attrs: []string{"align"}},
	"rule":         {cat: catBlock},
	"footnote":     {cat: catContainer, attrs: []string{"label"}, pseudos: []string{"label"}},
	"front-matter": {cat: catBlock},
	"link-def":     {cat: catBlock},
	"strong":       {cat: catInline, classes: []string{"html"}},
	"emphasis":     {cat: catInline, classes: []string{"html"}},
	"strike":       {cat: catInline, classes: []string{"html"}},
	"underline":    {cat: catInline},
	"mark":         {cat: catInline},
	"sup":          {cat: catInline},
	"sub":          {cat: catInline},
	"kbd":          {cat: catInline},
	"link":         {cat: catInline, classes: []string{"auto"}, attrs: []string{"href", "title"}},
	"image":        {cat: catInline | catObject, classes: []string{"alone"}, attrs: []string{"src", "alt", "title"}},
	"footnote-ref": {cat: catInline, attrs: []string{"label"}},
	"syntax":       {cat: catInline, classes: syntaxClasses},
	"token":        {cat: catInline, classes: tokenClasses},
}

// syntaxClasses name the construct a syntax element belongs to.
var syntaxClasses = []string{
	"strong", "emphasis", "strike", "code", "link", "image", "math", "html",
	"escape", "entity", "footnote-ref", "fence", "info", "rule",
}

// tokenClasses are the kinds of the tokens of highlighted code.
var tokenClasses = []string{
	"keyword", "type", "function", "variable", "property", "constant",
	"number", "string", "escape", "regexp", "comment", "operator",
	"punctuation", "tag", "attribute", "meta", "heading", "emphasis",
	"strong", "link", "code", "inserted", "deleted", "invalid",
}

// globalPseudos are the pseudo-elements of the document as a whole.
var globalPseudos = []string{"selection", "search-text", "placeholder"}

// pseudoClassNames are the pseudo-classes of elements; "current" goes
// after ::search-text alone.
var pseudoClassNames = []string{"first-child", "last-child", "revealed", "broken", "root", "not"}

// unsupportedPseudos are pseudo-classes and pseudo-elements of CSS that a
// document of the editor has no use for, with why.
var unsupportedPseudos = map[string]string{
	"hover":         "nodes have no hover state",
	"active":        "nodes have no active state",
	"focus":         "nodes have no focus state",
	"focus-within":  "nodes have no focus state",
	"visited":       "links have no visited state",
	"nth-child":     ":nth-child is not supported: use .odd or .even on rows",
	"nth-of-type":   ":nth-of-type is not supported",
	"first-of-type": ":first-of-type is not supported: use :first-child",
	"last-of-type":  ":last-of-type is not supported: use :last-child",
	"is":            ":is() is not supported: list the selectors",
	"where":         ":where() is not supported: list the selectors",
	"has":           ":has() is not supported",
	"before":        "::before is not supported: nodes get no generated content",
	"after":         "::after is not supported: nodes get no generated content",
	"first-line":    "::first-line is not supported",
	"first-letter":  "::first-letter is not supported",
}

var typeNames = slices.Sorted(maps.Keys(elementTypes))

// allClasses are the classes of every type, for selectors of no type.
var allClasses = func() []string {
	var out []string
	for _, t := range elementTypes {
		for _, c := range t.classes {
			if !slices.Contains(out, c) {
				out = append(out, c)
			}
		}
	}
	slices.Sort(out)
	return out
}()

var allAttrs = func() []string {
	var out []string
	for _, t := range elementTypes {
		for _, a := range t.attrs {
			if !slices.Contains(out, a) {
				out = append(out, a)
			}
		}
	}
	slices.Sort(out)
	return out
}()

var allPseudos = func() []string {
	out := slices.Clone(globalPseudos)
	for _, t := range elementTypes {
		for _, p := range t.pseudos {
			if !slices.Contains(out, p) {
				out = append(out, p)
			}
		}
	}
	slices.Sort(out)
	return out
}()
