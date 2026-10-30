package main

import _ "embed"

// sampleDoc covers every node of the theme vocabulary: the document the
// playground opens, so that a selector's effect shows at once.
//
//go:embed sample.md
var sampleDoc string

// paperCSS is the sample theme 纸张, which the docs also walk through.
//
//go:embed themes/paper.css
var paperCSS string

// preset is a theme the CSS editor starts from.
type preset struct{ name, css string }

// presets are the themes the row above the editor offers: the built-in
// default.css alone, and the sample.
var presets = []preset{
	{"默认", ""},
	{"纸张", paperCSS},
}
