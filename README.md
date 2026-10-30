# livemark

A Typora-style WYSIWYG Markdown editor for [MyGo](https://mygo.egoist.dev/)'s native UI. The document is the rendering: the caret moves through headings, lists, quotes, tables, code and math as they read; inline syntax (`**`, `` ` ``, `[]()`, `$`) appears only around the caret, and block syntax never shows. Everything is drawn by Go on the GPU — no WebView.

> **Status:** freshly forked from [Muse](https://github.com/CarlOpenLab/muse)'s editor, not yet reshaped. Next come a public syntax tree and themes written as CSS.

```go
import "github.com/CarlOpenLab/livemark"

// Kept by the app across frames.
ed := livemark.New("# Hello\n", livemark.Options{Placeholder: "Write…"})

// In the view.
ed.View(c)

// The Markdown, exactly as written.
src := ed.Source()
```

## Layout

```
*.go            the editor: parse, layout, paint, caret, input methods, undo
internal/mdoc/  formulas (go-latex), Mermaid diagrams, image loading
docs/           how the editor works, and design notes
```

## Develop

```bash
go test ./...
```

Requires Go 1.27.3+ and MyGo v0.7.3.

## Origin

Forked on 2026-10-30 from Muse at `0c7dda1`, with the changes then in progress in its working tree:

- `native/mdedit` → the root package, unchanged but for its name. It took along uncommitted changes to `doc.go`, `display.go` and `editor.go`, the untracked `cjk.go` (CJK emphasis rules, still being written) and the `zz_*_test.go` harnesses.
- `native/mdoc` → `internal/mdoc`: its formula and Mermaid code with their tests, and the image and heading helpers of its chat renderer (`image.go`). The chat renderer itself, and goldmark with it, stayed in Muse.

## License

[MIT](LICENSE)
