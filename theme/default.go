package theme

import (
	_ "embed"
	"sync"
)

//go:embed default.css
var defaultCSS string

var defaultSheet = sync.OnceValue(func() *Sheet {
	s, _ := Parse("default.css", defaultCSS)
	return s
})

// Default returns default.css: the editor's own look, which every theme
// is layered on.
func Default() *Sheet { return defaultSheet() }

// DefaultCSS returns the source of default.css, for an app that shows it
// as a start for a theme.
func DefaultCSS() string { return defaultCSS }
