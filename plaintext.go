package livemark

import (
	"strings"

	"github.com/egoist/mygo/ui/markdown/parser"
)

// plainText returns the text of inlines, as parser.PlainText does, with
// the characters CJKFriendly swapped taken back: a Text inline that is the
// parsed source as it is reads the editor's source instead.
func (e *Editor) plainText(ins []parser.Inline) string {
	parsed := e.doc.Source()
	var b strings.Builder
	for k := range ins {
		switch in := &ins[k]; in.Kind {
		case parser.Text:
			if in.End <= len(parsed) && in.End <= len(e.src) && in.Text == parsed[in.Start:in.End] {
				b.WriteString(e.src[in.Start:in.End])
			} else {
				b.WriteString(in.Text)
			}
		case parser.Code, parser.HTML, parser.Math:
			b.WriteString(in.Text)
		case parser.SoftBreak, parser.LineBreak:
			b.WriteByte(' ')
		case parser.FootnoteRef:
			b.WriteString("[^" + in.Text + "]")
		}
	}
	return b.String()
}
