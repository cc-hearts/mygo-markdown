# How the editor works

livemark edits Markdown as Typora does: the document is one surface the user
writes on, rendered as it reads, with no WebView and no separate source
mode. This note records how the editor is built; the code and its tests
(`go test ./...`) are the reference.

## The source is the only state

The editor keeps the Markdown string and nothing else that the user could
lose: the caret, the selection, undo, the input method's composition and
every command work on byte offsets of that string. Nothing is ever
serialized from a model, so a file saves exactly as it was written, and an
edit from outside (an AI tool, a reload) is one more edit of the same
string — undoable like typing.

After each edit the source is parsed again with MyGo's CommonMark/GFM
parser (`ui/markdown/parser`), which reparses only the blocks an edit
touches and keeps the source range of every block and inline. Those ranges
are what lets the editor hide syntax and still put every caret and every
keystroke at an exact offset.

## Leaves

`doc.go` turns the parse into a flat list of *leaves*, the units laid out
one below another: paragraphs, headings, blocks of code, math and HTML,
tables (whose cells are leaves of their own), rules, raw lines no block
holds (link reference definitions, front matter) and the phantom (below).
Each leaf carries the containers around it — quotations, list items,
footnotes — which draw as chrome (bars, bullets, numbers, check boxes) and
set the prefix a new line in them needs.

## Display and reveal

`display.go` builds a leaf's display: pieces of text, each standing for a
range of the source.

- *Text* pieces are the source character for character.
- *Atoms* stand for a range as a whole: a line break and the container
  prefix after it, an escape or an entity, a formula or an image drawn as
  an inline object.
- *Decorations* stand for nothing, as a fence's backticks beside an info
  string, or an input method's composition.

Block syntax is never a piece. Inline syntax — emphasis markers, code
backticks, link brackets and destinations, `$`, `![]()`, formatting HTML
tags — is a piece only while the caret is inside or at the edge of its
construct; elsewhere it is skipped and the construct renders. Mapping a
display offset back to the source picks the candidate nearest the caret's
direction of travel, so arrows walk into revealed markers one character at
a time, and a click lands next to the character it hit.

A heading's `#`, an item's `-` or a quotation's `>` typed alone, without
the space after it, is *pending*: it shows as text, so typing `#hashtag`
or `-1` never flashes a heading or a bullet.

## Layout and painting

`layout.go` shapes each leaf with `ui.ShapeRichTextWith` (formulas and
images are inline objects: `Span.ObjectWidth/Height/Descent`), caches the
views by their content, width, computed style and reveal state, and stacks
the leaves with the margins the theme gives them. Tables size their columns
from their cells' natural widths. `paint.go` draws only what
`Painter.Visible` shows: panels, chrome, search matches, the selection,
text, objects, Mermaid diagrams and the caret, which is as tall as the text
rather than its leading — each in its node's computed style from the
theme.

## Editing

`edit.go` writes the Markdown that each structural key means, in the
containers the caret is in:

- Enter splits a paragraph, continues a list (incrementing numbers,
  unchecked boxes for tasks) or a quotation, leaves a list or quotation on
  an empty item, moves through table rows, and keeps indentation in code.
- Backspace at a block's start turns a heading into a paragraph, takes an
  item out of its list (or up a level), lifts a paragraph out of its
  quotation, joins paragraphs, and removes an empty code or math block.
- Tab and Shift+Tab nest and lift list items, walk table cells, indent
  code.
- ```` ``` ````, `~~~` and `$$` typed alone on a line open a block with its
  closing fence at once; `| a | b |` + Enter makes a table; `|` in a cell
  is escaped; brackets and quotes pair.

Markdown has no empty paragraphs, so the paragraph Enter opens is a
*phantom*: an empty leaf in the editor's state, with the separators its
first character brings into the source. Deleting a paragraph's last
character turns it back into one, tidying the blank lines it leaves.

`history.go` records changes as (offset, old, new) with the selections
around them, coalescing typing a word at a time.

## Input methods

`ime.go` makes the editor a `ui.TextInputClient`: the composition is a
virtual insertion at the caret, shown underlined in the caret's leaf, in
UTF-16 units of the source with it; the commit goes in as typed text, one
step of undo.

## Themes and the tree

The parser's output is also public: `syntax.Parse` returns a tree whose
nodes carry an element — the type, classes and attributes a theme selects —
and the ranges of syntax hidden inside them. The editor takes each node's
style from `theme`, a CSS engine over that vocabulary; the packages'
doc comments have the details, and [theming](theming.md) is the author's
guide.

## What is not done

- Formulas are typeset by go-latex, which lays out a subset of TeX;
  `render/math` composes the scripts it cannot (as `\int_0^1`) itself.
  Diagrams come from `render/mermaid`, a renderer the app passes in.
- Resizing a very long document lays every leaf out again in one frame
  (about 150 ms for 340 KB); typing in it costs a few milliseconds.
