//go:build js

package main

// desktop reports that the playground can read and write theme files.
const desktop = false

// openTheme does nothing in a browser, which reads no local files.
func (a *app) openTheme() {}

// saveTheme does nothing in a browser: a theme leaves it by copying the
// CSS.
func (a *app) saveTheme() {}

// watchFile does nothing in a browser.
func (a *app) watchFile() {}
