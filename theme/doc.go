// Package theme reads the themes of the editor: style sheets in a subset of
// CSS whose selectors match the nodes of a Markdown document, as package
// syntax names them (heading.h1, quote, code.inline, item::marker), and
// computes the style of each node through the cascade and inheritance.
//
// A Theme stacks sheets in layers: the built-in default.css first, then
// the sheets of a theme, then an app's overrides, so that a theme writes
// only what it changes. Parse never fails: it skips what it cannot read, as
// a browser does, and reports it as diagnostics with their lines and
// columns.
//
// The package depends on nothing but the standard library: the window's
// colors come in as the variables --ui-*, and the editor picks the font of
// a font-family list, so that a theme can be checked without a window.
package theme
