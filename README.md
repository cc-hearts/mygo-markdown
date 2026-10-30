# livemark

A Typora-style WYSIWYG Markdown editor for [MyGo](https://mygo.egoist.dev/)'s native UI. The document is the rendering: the caret moves through headings, lists, quotes, tables, code and math as they read; inline syntax (`**`, `` ` ``, `[]()`, `$`) appears only around the caret, and block syntax never shows. Everything is drawn by Go on the GPU — no WebView.

The syntax is fixed and the tree behind it is public, so a theme is a CSS file written against the document's elements, not Go code.

> **Status:** the editor, public syntax tree, CSS themes, renderers, the desktop playground and the web playground are in place (M0–M7).

```go
import (
	"github.com/CarlOpenLab/livemark"
	"github.com/CarlOpenLab/livemark/render"
	"github.com/CarlOpenLab/livemark/render/mathrender"
	"github.com/CarlOpenLab/livemark/render/mermaidrender"
)

// Kept by the app across frames.
ed := livemark.New("# Hello\n", livemark.Options{
	Placeholder: "Write…",
	Math:        mathrender.New(),
	Diagrams:    []render.DiagramRenderer{mermaidrender.New()},
})

// In the view.
ed.View(c)

// The Markdown, exactly as written.
src := ed.Source()

// The style path at the caret, e.g. document › quote › paragraph › strong:revealed.
path := ed.StylePath(ed.Caret())
```

Formulas and diagrams are renderers the app passes in: without them the
editor shows their source, and an app that needs neither links neither.

## Themes

A theme parses to a sheet of rules over the document's elements — `heading`,
`quote`, `code.block`, `item.checked`, `syntax:revealed`, `token.keyword` —
and the editor draws with each node's computed style:

```go
sheet, diagnostics := theme.Parse("mine", css) // never fails; CSS it cannot read is a diagnostic
ed.SetOptions(livemark.Options{Theme: theme.New(sheet)})
```

[Writing a theme](docs/theming.md) is the reference: the CSS subset, the
element vocabulary and the properties that draw. A sample theme, 纸张, lives
in the playground. [写主题](docs/theming.zh-CN.md) is the same document in
Chinese.

## Playground

`cmd/playground` edits a theme beside the document it styles: live CSS editor
with 默认/纸张 presets, diagnostics that jump to their line, a node-path
inspector that inserts selectors, the public syntax tree, light/dark, and —
on the desktop — opening, saving and hot-reloading theme files.

```bash
go run ./cmd/playground
```

The same program compiles to WebAssembly for browsers, with the desktop-only
file features left out:

```bash
./scripts/build-web.sh                      # writes dist/web, a static site
python3 -m http.server 8123 --directory dist/web
```

The loader page MyGo writes shows a spinner until the app is up. Every push to
`main` builds it and publishes it to GitHub Pages
(`.github/workflows/pages.yml`; with Pages set to deploy from GitHub Actions).

Live: https://cc-hearts.github.io/mygo-markdown/

## Layout

```
*.go               the editor: parse, layout, paint, caret, input methods, undo
syntax/            the public syntax tree: nodes, ranges, style paths, label strings
theme/             the CSS theme engine: parser, selectors, cascade, vocabulary
render/            the interfaces through which the editor draws formulas and diagrams
render/math/       TeX typesetting on go-latex
render/mermaid/    Mermaid diagrams
internal/mdoc/     image loading and the helpers shared with Muse's renderer
cmd/playground/    the theme playground, its sample document and 纸张
scripts/           build-web.sh: the playground as a static site for browsers
mygo.json          the playground app, as MyGo's CLI builds it
docs/              how the editor works, how to write a theme, and design notes
```

## Develop

```bash
go test ./...
go run ./cmd/playground
```

Requires Go 1.27.3+ and MyGo v0.7.3.

## Origin

Forked on 2026-10-30 from Muse at `0c7dda1`, with the changes then in progress in its working tree:

- `native/mdedit` → the root package, unchanged but for its name. It took along uncommitted changes to `doc.go`, `display.go` and `editor.go`, the untracked `cjk.go` (CJK emphasis rules, still being written) and the `zz_*_test.go` harnesses.
- `native/mdoc` → `internal/mdoc`: its formula and Mermaid code with their tests, and the image and heading helpers of its chat renderer (`image.go`). The chat renderer itself, and goldmark with it, stayed in Muse.

## License

[MIT](LICENSE)
