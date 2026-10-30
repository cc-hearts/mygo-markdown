// Package parse prepares Markdown source as the editor reads it, and
// parses it with the extensions the editor reads. The editor and the
// public syntax tree both parse through it, so both see the same document:
// the YAML front matter blanked out, and the characters beside CJK
// emphasis delimiters swapped so the parser reads emphasis as Typora does.
package parse

import "github.com/egoist/mygo/ui/markdown/parser"

// Exts is the Markdown the editor reads: GitHub's flavor, footnotes and
// math, with $…$ as inline math, as Typora writes it.
const Exts = parser.Default | parser.ExtDollarMath

// Text returns the source to give the parser for src, and where the YAML
// front matter at its top ends, after its closing line, or 0 when it has
// none. The result is as long as src, and every byte where it was.
func Text(src string) (string, int) {
	fm := frontMatterEnd(src)
	return CJKFriendly(maskFrontMatter(src, fm)), fm
}

// Parse parses src as the editor reads it, and returns the document and
// where its front matter ends.
func Parse(src string) (*parser.Document, int) {
	text, fm := Text(src)
	d := parser.New(Exts)
	d.Set(text)
	return d, fm
}
