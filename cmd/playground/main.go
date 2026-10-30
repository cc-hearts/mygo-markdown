// Command playground is livemark's theme playground: a CSS editor on the
// left, the document it styles on the right. A change shows at once, the
// theme's diagnostics list below, and the status bar names the nodes at
// the caret, to write selectors against.
//
// It runs on the desktop:
//
//	go run ./cmd/playground
package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func main() {
	a := newApp()
	mygo.App.WhenReady(func() {
		a.win.Store(mygo.NewWindow(mygo.WindowOptions{
			Title:     "livemark · 主题 Playground",
			Width:     1320,
			Height:    860,
			MinWidth:  960,
			MinHeight: 600,
			StateKey:  "playground",
			Content:   ui.View(a.view),
		}))
		a.watchFile()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
