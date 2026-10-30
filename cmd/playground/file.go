//go:build !js

package main

import (
	"os"
	"time"

	"github.com/egoist/mygo"
)

// desktop reports that the playground can read and write theme files.
const desktop = true

// themeFilters are what a theme file dialog shows.
var themeFilters = []mygo.FileFilter{{Name: "CSS", Extensions: []string{"css"}}}

// openTheme asks for a CSS file and opens it.
func (a *app) openTheme() {
	win := a.window()
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:  win,
			Title:   "打开主题",
			Filters: themeFilters,
		})
		if err != nil {
			mygo.Dialog.Error("打开主题失败", err.Error())
			return
		}
		if len(paths) == 0 {
			return
		}
		data, err := os.ReadFile(paths[0])
		if err != nil {
			mygo.Dialog.Error("打开主题失败", err.Error())
			return
		}
		css := string(data)
		mygo.RunOnMain(func() {
			a.setCSS(css)
			a.css.Buffer.MarkSaved()
			a.setFile(paths[0], modTime(paths[0]))
			a.preset = -1
			a.parse()
			a.invalidate()
		})
	}()
}

// saveTheme saves the CSS to its file, asking for one the first time.
func (a *app) saveTheme() {
	css := a.css.Buffer.String()
	if path := a.filePath(); path != "" {
		go a.writeTheme(path, css)
		return
	}
	win := a.window()
	go func() {
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent:      win,
			Title:       "保存主题",
			DefaultPath: "theme.css",
			Filters:     themeFilters,
		})
		if err != nil {
			mygo.Dialog.Error("保存主题失败", err.Error())
			return
		}
		if path == "" {
			return
		}
		a.writeTheme(path, css)
	}()
}

// writeTheme writes the CSS and makes the file the one saved to.
func (a *app) writeTheme(path, css string) {
	if err := os.WriteFile(path, []byte(css), 0o644); err != nil {
		mygo.Dialog.Error("保存主题失败", err.Error())
		return
	}
	mygo.RunOnMain(func() {
		a.css.Buffer.MarkSaved()
		a.setFile(path, modTime(path))
		a.invalidate()
	})
}

// watchFile reloads the open theme file when another program changes it,
// as long as the playground has no edits of its own.
func (a *app) watchFile() {
	go func() {
		for {
			time.Sleep(400 * time.Millisecond)
			path, mod := a.filePath2()
			if path == "" {
				continue
			}
			fi, err := os.Stat(path)
			if err != nil || !fi.ModTime().After(mod) {
				continue
			}
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			css, at := string(data), fi.ModTime()
			mygo.RunOnMain(func() { a.reload(path, css, at) })
		}
	}()
}

// reload shows a file changed elsewhere, unless the editor has edits.
func (a *app) reload(path, css string, mod time.Time) {
	if a.filePath() != path || a.css.Buffer.Modified() {
		return
	}
	a.setCSS(css)
	a.css.Buffer.MarkSaved()
	a.setFile(path, mod)
	a.preset = -1
	a.parse()
	a.invalidate()
}

// filePath returns the open file's path, "" for none.
func (a *app) filePath() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.file
}

// filePath2 returns the open file's path and the time it was last read.
func (a *app) filePath2() (string, time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.file, a.fileMod
}

// setFile makes path the open file, as of mod.
func (a *app) setFile(path string, mod time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.file, a.fileMod = path, mod
}

// modTime returns when a file was last changed, zero when it cannot be
// read.
func modTime(path string) time.Time {
	fi, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return fi.ModTime()
}
