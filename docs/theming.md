# Writing a theme

livemark draws a document the way a theme says: every color, font, length
and box in the editor comes from the document's computed styles, and a
theme is CSS. This guide is the reference for that CSS: the values it
reads, the selectors that match, and the elements and properties a theme
may name.

The built-in look is `theme/default.css`, which every theme is layered on
top of, so a theme writes only what it changes. The sample theme
`cmd/playground/themes/paper.css` is a worked example, and the playground
shows one next to the document it styles:

```bash
go run ./cmd/playground
```

## Using a theme

```go
import (
	"github.com/CarlOpenLab/livemark"
	"github.com/CarlOpenLab/livemark/theme"
)

sheet, diagnostics := theme.Parse("my-theme", css)
for _, d := range diagnostics {
	log.Print(d) // "12:3: warning: heading has no property ..."
}
ed := livemark.New(src, livemark.Options{Theme: theme.New(sheet)})
```

`theme.Parse` never fails: it skips what it cannot read, as a browser does,
and reports it as diagnostics with a line and a column. `theme.New` stacks
sheets over `default.css` in the order given; `Theme.With` adds one more
layer, as an app's settings do. The environment decides dark and light:
`theme.Resolver(theme.Env{Dark: true, Vars: ...})` is what the editor makes
each time the window's appearance or colors change, and it supplies the
`--ui-*` variables. An app that renders documents without a window can
resolve styles the same way.

## The CSS subset

### Values

- **Lengths**: `px`, `em` (of the element's font size) and `rem` (of the
  document's), with `calc()`, `min()`, `max()`, `clamp()` and parentheses.
  `font-size` and `line-height` also take percentages; `line-height` takes
  a number, a multiple of the font size.
- **Colors**: `#rgb`, `#rgba`, `#rrggbb`, `#rrggbbaa`, `rgb()` and `rgba()`
  (comma or space syntax, with `/` alpha), `hsl()` and `hsla()`, the named
  colors, `transparent`, `currentcolor`, and
  `color-mix(in srgb, a 30%, b)`.
- **Variables**: declare `--name: value` on any element; read it with
  `var(--name, fallback)`. Variables cascade; a variable's `var()`s are
  read where it is declared. The window supplies `--ui-text`,
  `--ui-text-muted`, `--ui-background`, `--ui-surface`, `--ui-border`,
  `--ui-accent`, `--ui-accent-text`, `--ui-selection`, `--ui-danger`,
  `--ui-warning`, `--ui-success`, `--ui-font`, `--ui-font-size` and
  `--ui-radius`.
- **Font families**: a comma-separated list of names, quoted when they are
  not plain words; the generic families `serif`, `sans-serif`, `monospace`,
  `system-ui`, `ui-serif`, `ui-sans-serif`, `ui-monospace`, `ui-rounded`,
  `cursive` and `fantasy` choose the window's own fonts.

### Selectors

A selector names an element by its type, its classes and attributes, with
the combinators of CSS:

```css
quote strong { color: #a5392b; }       /* a descendant */
item.checked > paragraph { color: gray; } /* a child */
heading.h1, heading.h2 { font-weight: 700; }
code.block[lang="go"] { /* ... */ }
cell[align="right"] { text-align: right; }
:root { font-family: serif; }
* { }
```

- Classes are `.name`; ids are not supported, as nodes have none.
- Attribute selectors: `[name]`, `[name="value"]`, `~=`, `^=`, `$=`, `*=`,
  with an optional `i` for case-insensitive values.
- Pseudo-classes: `:root` (the document), `:first-child`, `:last-child`,
  `:revealed` (the syntax is showing at the caret), `:broken` (an image
  that did not load, or a formula that did not typeset) and `:not(...)`,
  which takes one compound selector without pseudo-elements.
- Pseudo-elements: `::marker` and `::checkbox` (an item's), `::info` (a
  fenced block's info string), `::label` (a footnote's), and the
  document's `::selection`, `::search-text` (with `:current` on the chosen
  match) and `::placeholder`.
- Hover, focus, `::before`, `::after` and `:nth-child()` are reported and
  match nothing: the document's nodes have no such states, and generated
  content has no place.

### Cascade and at-rules

A theme is a stack of sheets. The built-in `default.css` is the first
layer; the layers of `theme.New` follow in order. Within the stack, the
usual cascade decides: specificity first (classes, attributes and
pseudo-classes, then types and pseudo-elements), then order, and
`!important` beats everything not important. `inherit`, `initial` and
`unset` work as in CSS.

- `@media (prefers-color-scheme: dark)` and `(prefers-color-scheme: light)`
  choose the appearance; `and` combines them, and no other media feature is
  read.
- `@font-face` brings a TrueType or OpenType font: `font-family`, `src`,
  `font-weight` and `font-style`; `src` may list files and picks the first
  the format supports.
- `@import` is reported and skipped: put the rules in the sheet.

Every problem is a diagnostic: an error is CSS the parser skipped, a
warning is CSS that reads but does nothing here, as a property that does
not apply to the elements its selector matches. The playground lists them
and jumps to the line.

## Elements

Each node of a document is an element, named as package `syntax` names its
nodes. A theme selectors these:

| Element | What it is | Classes | Attributes | Pseudo-elements |
|---|---|---|---|---|
| `document` | The whole document: `:root` | | | `::selection`, `::search-text`, `::placeholder` |
| `paragraph` | A paragraph | | | |
| `heading` | A heading | `h1`–`h6`, `setext` | `level` | |
| `quote` | A quotation | | | |
| `list` | A list | `ordered`, `bullet`, `tight`, `loose` | `start`, `depth` | |
| `item` | A list item | `task`, `checked` | `number`, `depth` | `::marker`, `::checkbox` |
| `code` | A code span or block | `block`, `fenced`, `indented`, `inline`, `html` | `lang`, `info` | `::info` |
| `math` | A formula | `block`, `inline` | | |
| `html` | Raw HTML | `block`, `inline` | `tag` | |
| `table` | A table | | | |
| `row` | A table row | `header`, `odd`, `even` | | |
| `cell` | A table cell | `header` | `align` | |
| `rule` | A thematic break | | | |
| `footnote` | A footnote's text | | `label` | `::label` |
| `front-matter` | The YAML front matter | | | |
| `link-def` | A link reference definition | | | |
| `strong` | Bold text | `html` | | |
| `emphasis` | Italic text | `html` | | |
| `strike` | Struck text | `html` | | |
| `underline` | Underlined text | | | |
| `mark` | Highlighted text | | | |
| `sup`, `sub` | Superscript, subscript | | | |
| `kbd` | A key | | | |
| `link` | A link | `auto` | `href`, `title` | |
| `image` | An image | `alone` | `src`, `alt`, `title` | |
| `footnote-ref` | A footnote reference | | `label` | |
| `syntax` | The Markdown syntax showing at the caret | see below | | |
| `token` | A token of highlighted code | see below | | |

`syntax` names the construct the markers belong to: `strong`, `emphasis`,
`strike`, `code`, `link`, `image`, `math`, `html`, `escape`, `entity`,
`footnote-ref`, `fence`, `info` and `rule`. Block syntax never shows: a
heading's `#`, a fence and a bullet are replaced by what they mean. Inline
syntax shows only around the caret, and while it does, the construct is in
the `:revealed` state, as a block's source is while the caret is in it.

`token` is a token of a fenced or indented code block whose language
livemark highlights: `keyword`, `type`, `function`, `variable`, `property`,
`constant`, `number`, `string`, `escape`, `regexp`, `comment`, `operator`,
`punctuation`, `tag`, `attribute`, `meta`, `heading`, `emphasis`, `strong`,
`link`, `code`, `inserted`, `deleted` and `invalid`. Tokens nest inside the
block's `code` element, so `code.block token.keyword` colors them there and
nothing else.

An element's classes and attributes are the nodes it actually has: only a
task item has `task` and `checked`, only a table's first row has `header`,
only rows after it have `odd` and `even`, and only an image alone in its
paragraph has `alone`. A path at any offset is what the playground's
inspector shows, e.g. `document › quote › paragraph › strong:revealed`.

## Properties

These are the properties a theme reads, with their initial value,
whether they inherit, and where they apply. A value on an element they do
not apply to is a warning, not a silent no-op.

| Property | Initial | Inherited | Applies to | Notes |
|---|---|---|---|---|
| `color` | `#000` | yes | everything | |
| `font-family` | `system-ui` | yes | everything | a list |
| `font-size` | `16px` | yes | everything | length or percentage |
| `font-weight` | `400` | yes | everything | `normal`, `bold` or 1–1000 |
| `font-style` | `normal` | yes | everything | `normal`, `italic`, `oblique` |
| `font-feature-settings` | `normal` | yes | everything | quoted 4-letter tags |
| `font-variant-numeric` | `normal` | yes | everything | `normal`, `proportional-nums`, `tabular-nums` |
| `letter-spacing` | `normal` | yes | everything | length |
| `line-height` | `1.75` | yes | everything | number, length or percentage |
| `text-align` | `start` | yes | everything | `start`, `left`, `center`, `right`, `end` |
| `caret-color` | `auto` | yes | everything | a color, or the text's |
| `list-style-type` | `auto` | yes | `list` | `auto`, `disc`, `circle`, `square`, `decimal`, `none`, `"string"` |
| `text-decoration-line` | `none` | no | everything | `underline`, `line-through` |
| `text-decoration-style` | `solid` | no | everything | `solid`, `wavy` |
| `text-decoration-color` | `currentcolor` | no | everything | |
| `text-decoration-thickness` | `auto` | no | everything | length or `auto` |
| `vertical-align` | `baseline` | no | inlines | `baseline`, `super`, `sub`, length |
| `opacity` | `1` | no | everything | a number or percentage; multiplies the colors |
| `background-color` | `transparent` | no | everything | |
| `border-radius` | `0` | no | everything | |
| `padding-top` … `padding-left` | `0` | no | `document`, blocks, containers | |
| `margin-top`, `margin-bottom` | `0` | no | blocks and containers | |
| `margin-right` | `0` | no | pseudo-elements | between a marker and its text |
| `margin-left` | — | — | nothing | use `padding-left` |
| `border-top-width` … `border-left-width` | `medium` | no | blocks, containers, pseudo-elements, images | `thin`, `medium`, `thick` or a length |
| `border-top-style` … `border-left-style` | `none` | no | as widths | `none`, `solid` |
| `border-top-color` … `border-left-color` | `currentcolor` | no | as widths | |
| `max-width` | `820px` | no | `document` | the column of text |
| `width`, `height` | `auto` | no | blocks, pseudo-elements | a rule's height, a check box's size |

The shorthands are `background`, `padding`, `margin`, `border`,
`border-top`, `border-right`, `border-bottom`, `border-left`,
`border-width`, `border-style`, `border-color`, `text-decoration` and
`list-style`.

### How a style draws

- `:root` sets the page: its `background-color` fills the editing surface,
  its `max-width` and `padding` size the column, and its font, color,
  `line-height` and `caret-color` are the document's.
- A block's `background-color` and `border-radius` fill its box;
  `padding` separates its content from that box.
- Borders draw the top edge of blocks, a quotation's left edge, and the
  grid of a table, whose `border` draws its lines. Other edges are read
  but not drawn: a heading's underline is `text-decoration`, not
  `border-bottom`.
- `text-decoration` draws on the text itself, so it follows headings,
  links and paragraphs; its color and thickness are the properties above.
- `::marker` and `::checkbox` size and color list markers; `::info` colors
  a fenced block's language label; `::label` colors a footnote's `[^1]`.
- `code` sets the font of code, `token.*` its colors, and `math` the ink
  of a typeset formula. A diagram draws in the colors of its code block and
  the `--diagram-muted`, `--diagram-line`, `--diagram-fill`,
  `--diagram-accent` and `--ui-background` variables.
- `image:broken` styles the placeholder of an image that did not load, and
  `image:revealed` its source while the caret is at it.

## A worked example

`cmd/playground/themes/paper.css` is the sample theme 纸张: serif body
text, warm ink, a red seal color, headings underlined with a hairline, and
zebra-striped tables. Its start:

```css
:root {
  --ink: #3a3226;
  --muted: #8a7d66;
  --rule: #ddd0b4;
  --seal: #a5392b;

  font-family: "Songti SC", "Noto Serif CJK SC", "Times New Roman", serif;
  font-size: 16.5px;
  line-height: 1.9;
  color: var(--ink);
  background-color: #faf6ec;
  caret-color: var(--seal);
  max-width: 740px;
  padding: 3rem 44px 0;
}

@media (prefers-color-scheme: dark) {
  :root {
    --ink: #e6ddc9;
    --seal: #d97b6a;
    background-color: #221f1a;
  }
}

heading {
  text-decoration: underline;
  text-decoration-color: var(--rule);
  text-decoration-thickness: 1px;
}
row.even { background-color: rgba(120, 96, 48, .06); }
```

The rest of the file styles quotes, list markers, check boxes, links,
code, tables and the broken-image placeholder the same way: a few
selectors, the variables above, and nothing else.

## The playground

`cmd/playground` is the theme author's workbench:

- The CSS editor on the left offers the presets 默认 (an empty sheet over
  `default.css`) and 纸张 (the sample theme); what is typed applies about
  150 ms after typing rests.
- The document on the right covers every element in the table above.
- The bar below the editor lists the theme's diagnostics; a click goes to
  the line they are about.
- The status bar names the path at the caret and inserts it as a selector
  at the caret of the CSS, with a rule's braces ready.
- 语法树 shows the public syntax tree of the document, with each node's
  element, range and hidden syntax.
- 深色 switches the window's appearance, which `@media
  (prefers-color-scheme)` follows.
- On the desktop, 打开 and 保存 read and write a theme file, and the
  playground reloads it when another program changes it.
