package livemark

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unsafe"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"

	"github.com/CarlOpenLab/livemark/theme"
)

// The theme's styles take the place of the editor's hard-coded look: a
// leaf's block element, and the containers around it, are a path of nodes
// in the resolver, and every length and color the editor draws with is
// that node's computed style.

// uiVars returns the window's colors and fonts, which default.css reads as
// the --ui-* variables.
func uiVars(t *ui.Theme) map[string]string {
	return map[string]string{
		"--ui-text":        cssColor(t.Text),
		"--ui-text-muted":  cssColor(t.TextMuted),
		"--ui-background":  cssColor(t.Background),
		"--ui-surface":     cssColor(t.Surface),
		"--ui-border":      cssColor(t.Border),
		"--ui-accent":      cssColor(t.Accent),
		"--ui-accent-text": cssColor(t.AccentText),
		"--ui-selection":   cssColor(t.Selection),
		"--ui-danger":      cssColor(t.Danger),
		"--ui-warning":     cssColor(t.Warning),
		"--ui-success":     cssColor(t.Success),
		"--ui-font":        t.Font,
		"--ui-font-size":   strconv.FormatFloat(float64(t.FontSize), 'f', -1, 32),
		"--ui-radius":      strconv.FormatFloat(float64(t.Radius), 'f', -1, 32),
	}
}

// cssColor returns a color as CSS writes it.
func cssColor(c ui.Color) string {
	if c.A == 255 {
		return fmt.Sprintf("rgb(%d, %d, %d)", c.R, c.G, c.B)
	}
	return fmt.Sprintf("rgba(%d, %d, %d, %s)",
		c.R, c.G, c.B, strconv.FormatFloat(float64(c.A)/255, 'f', 3, 64))
}

// styleKey is what the computed styles depend on: the theme, whether the
// window is dark, and the colors and fonts it passes. A change makes a new
// resolver, and with it new styles.
type styleKey struct {
	theme                                                        *theme.Theme
	dark                                                         bool
	font                                                         string
	fontSize                                                     float32
	radius                                                       float32
	text, muted, background, surface, border, accent, accentText ui.Color
	selection, danger, warning, success                          ui.Color
}

// resolverGen is a resolver of one environment, kept so that a window
// that changes its appearance and changes it back finds its styles again.
type resolverGen struct {
	res *theme.Resolver
	gen uint64
}

// styleKeyOf returns what the computed styles depend on.
func (e *Editor) styleKeyOf(t *ui.Theme) styleKey {
	return styleKey{
		theme: e.theme, dark: t.Dark,
		font: t.Font, fontSize: t.FontSize, radius: t.Radius,
		text: t.Text, muted: t.TextMuted, background: t.Background,
		surface: t.Surface, border: t.Border, accent: t.Accent,
		accentText: t.AccentText, selection: t.Selection,
		danger: t.Danger, warning: t.Warning, success: t.Success,
	}
}

// ensureResolver returns the resolver of this frame, making it anew when
// what it was made of changed.
func (e *Editor) ensureResolver(t *ui.Theme) *theme.Resolver {
	k := e.styleKeyOf(t)
	if g, ok := e.resCache[k]; ok {
		e.res, e.styleGen = g.res, g.gen
		return g.res
	}
	if e.resCache == nil || len(e.resCache) >= 16 {
		e.resCache = map[styleKey]*resolverGen{}
	}
	g := &resolverGen{res: e.theme.Resolver(theme.Env{Dark: t.Dark, Vars: uiVars(t)}), gen: e.nextGen}
	e.nextGen++
	e.resCache[k] = g
	e.res, e.styleGen = g.res, g.gen
	return g.res
}

// rootStyle returns the document element's computed style.
func (e *Editor) rootStyle() *theme.Computed { return e.res.Root().Style() }

// pickFont returns the family of a computed list that the editor can use,
// the window's own for the system font.
func (e *Editor) pickFont(fams []string) string {
	for _, f := range fams {
		switch strings.ToLower(strings.TrimSpace(f)) {
		case "system-ui", "sans-serif", "-apple-system", "ui-sans-serif", "inherit":
			if e.lk.font != "" {
				return e.lk.font
			}
		case "monospace", "ui-monospace":
			if e.lk.mono != "" {
				return e.lk.mono
			}
			return "monospace"
		default:
			if f != "" {
				return f
			}
		}
	}
	return e.lk.font
}

// styleFont returns the ui font of a computed style.
func (e *Editor) styleFont(c *theme.Computed) ui.Font {
	w := c.FontWeight
	if w == 400 {
		w = 0
	}
	return ui.Font{
		Family:     e.pickFont(c.FontFamily),
		Size:       c.FontSize,
		Weight:     w,
		Italic:     c.Italic,
		LineHeight: c.LineHeight,
		Features:   c.FontFeatures,
	}
}

// uiColor returns a theme color as ui's.
func uiColor(c theme.Color) ui.Color {
	return ui.Color{R: c.R, G: c.G, B: c.B, A: c.A}
}

// styleSpan returns the ui span of a computed style, for text drawn in it.
func (e *Editor) styleSpan(c *theme.Computed) ui.Span {
	return ui.Span{
		Font:                e.pickFont(c.FontFamily),
		Size:                c.FontSize,
		Weight:              uiWeight(c.FontWeight),
		Italic:              c.Italic,
		Color:               uiColor(c.Color),
		Underline:           c.Decoration.Underline,
		WavyUnderline:       c.Decoration.Wavy,
		Strikethrough:       c.Decoration.LineThrough,
		DecorationColor:     uiColor(c.Decoration.Color),
		DecorationThickness: c.Decoration.Thickness,
		Background:          uiColor(c.TextBackground),
		Radius:              c.TextRadius,
		LetterSpacing:       c.LetterSpacing,
		Features:            c.FontFeatures,
		BaselineShift:       c.Shift,
	}
}

// uiWeight maps CSS's normal weight to ui's zero, the font's own.
func uiWeight(w int) int {
	if w == 400 {
		return 0
	}
	return w
}

// leafElement returns the block element of a leaf: what selectors match.
func (e *Editor) leafElement(l *leaf) theme.Element {
	var el theme.Element
	switch l.kind {
	case kPara:
		el = theme.Element{Type: "paragraph"}
	case kHeading:
		el = theme.Element{Type: "heading"}
		el.Classes = append(el.Classes, fmt.Sprintf("h%d", max(l.level, 1)))
		if l.setext {
			el.Classes = append(el.Classes, "setext")
		}
		el.Attrs = append(el.Attrs, theme.Attr{Name: "level", Value: strconv.Itoa(max(l.level, 1))})
	case kCode:
		el = theme.Element{Type: "code", Classes: []string{"block"}}
		if l.fenced {
			el.Classes = append(el.Classes, "fenced")
		} else {
			el.Classes = append(el.Classes, "indented")
		}
		if l.lang != "" {
			el.Attrs = append(el.Attrs, theme.Attr{Name: "lang", Value: l.lang})
		}
	case kMath:
		el = theme.Element{Type: "math", Classes: []string{"block"}}
	case kHTML:
		el = theme.Element{Type: "html", Classes: []string{"block"}}
	case kRule:
		el = theme.Element{Type: "rule"}
	case kRaw:
		if l.frontMatter {
			el = theme.Element{Type: "front-matter"}
		} else {
			el = theme.Element{Type: "link-def"}
		}
	case kTable:
		el = theme.Element{Type: "table"}
	default:
		el = theme.Element{Type: "paragraph"}
	}
	if e.revealAt(l) >= 0 {
		switch l.kind {
		case kCode, kMath, kHTML, kRule:
			el.States |= theme.Revealed
		}
	}
	return el
}

// leafSourceFirst and leafSourceLast tell whether the leaf holds the
// document's first or last source line among its siblings.
func (e *Editor) leafStates(l *leaf) theme.States {
	var st theme.States
	n := len(l.conts)
	if n > 0 {
		if slices.Contains(l.firstOf, n-1) {
			st |= theme.FirstChild
		}
		if slices.Contains(l.lastOf, n-1) {
			st |= theme.LastChild
		}
		return st
	}
	if len(e.leaves) == 0 {
		return st
	}
	if e.leaves[0] == l {
		st |= theme.FirstChild
	}
	if e.leaves[len(e.leaves)-1] == l {
		st |= theme.LastChild
	}
	return st
}

// contElements returns the elements a container adds to a style path: a
// list and its item for an item, and the container itself otherwise.
func contElements(c cont) []theme.Element {
	switch c.kind {
	case cQuote:
		return []theme.Element{{Type: "quote"}}
	case cFootnote:
		return []theme.Element{{Type: "footnote", Attrs: []theme.Attr{{Name: "label", Value: c.label}}}}
	}
	list := theme.Element{Type: "list"}
	if c.ordered {
		list.Classes = append(list.Classes, "ordered")
	} else {
		list.Classes = append(list.Classes, "bullet")
	}
	if c.tight {
		list.Classes = append(list.Classes, "tight")
	} else {
		list.Classes = append(list.Classes, "loose")
	}
	list.Attrs = append(list.Attrs, theme.Attr{Name: "depth", Value: strconv.Itoa(c.depth)})
	item := theme.Element{Type: "item"}
	if c.task != parser.NoTask {
		item.Classes = append(item.Classes, "task")
	}
	if c.task == parser.Checked {
		item.Classes = append(item.Classes, "checked")
	}
	if c.ordered {
		item.Attrs = append(item.Attrs, theme.Attr{Name: "number", Value: strconv.Itoa(c.num)})
	}
	item.Attrs = append(item.Attrs, theme.Attr{Name: "depth", Value: strconv.Itoa(c.depth)})
	return []theme.Element{list, item}
}

// appendContNode adds a container's elements to a path.
func appendContNode(n *theme.Node, c cont) *theme.Node {
	for _, el := range contElements(c) {
		n = n.Child(el)
	}
	return n
}

// styleSig is what a leaf's style path depends on, cheaply compared: two
// leaves of the same signature share their computed style, as the many
// paragraphs of a document do.
type styleSig struct {
	kind     leafKind
	level    int8
	flags    uint8
	states   theme.States
	revealed bool
	lang     string
	conts    string
	row, col int16
}

// leafStyled is a leaf's path of theme nodes and its block style: the
// nodes of its containers, the block's own, and the style of each.
type leafStyled struct {
	// id tells this path and style apart from another leaf's, for the
	// views that show them.
	id    uint64
	node  *theme.Node
	style *theme.Computed
	// conts are the nodes after each of the leaf's containers, contStyle
	// their styles.
	conts     []*theme.Node
	contStyle []*theme.Computed
}

// containerNode returns the node of the containers around a leaf: quotes,
// lists and their items, footnotes.
func (e *Editor) containerNode(l *leaf) *theme.Node {
	n := e.res.Root()
	for _, c := range l.conts {
		n = appendContNode(n, c)
	}
	return n
}

// contSig returns the signature of a path of containers, in a buffer the
// editor reuses: a leaf's signature is compared at once, then forgotten,
// and kept only when it is new.
func (e *Editor) contSig(conts []cont) string {
	if len(conts) == 0 {
		return ""
	}
	b := e.contBuf[:0]
	for _, c := range conts {
		b = append(b, byte(c.kind))
		if c.ordered {
			b = append(b, 'o')
		} else {
			b = append(b, 'b')
		}
		if c.tight {
			b = append(b, 't')
		} else {
			b = append(b, 'l')
		}
		b = append(b, byte(c.task), ':')
		b = strconv.AppendInt(b, int64(c.num), 10)
		b = append(b, ':')
		b = strconv.AppendInt(b, int64(c.depth), 10)
		b = append(b, ':')
		b = append(b, c.label...)
		b = append(b, ';')
	}
	e.contBuf = b
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// styleSigOf returns the signature of a leaf's style path.
func (e *Editor) styleSigOf(l *leaf) styleSig {
	s := styleSig{
		kind:   l.kind,
		level:  int8(min(l.level, 127)),
		states: e.leafStates(l),
		lang:   l.lang,
	}
	if l.pending {
		s.flags |= fPending
	}
	if l.setext {
		s.flags |= fSetext
	}
	if l.frontMatter {
		s.flags |= fFront
	}
	if l.fenced {
		s.flags |= fFenced
	}
	switch l.kind {
	case kCode, kMath, kHTML, kRule:
		s.revealed = e.revealAt(l) >= 0
	}
	if l.kind == kPara && l.table != nil {
		t := l.table
		row, col := l.rowCol()
		s.conts = e.contSig(t.conts)
		s.row, s.col = int16(row), int16(col)
		s.states |= e.leafStates(t)
	} else {
		s.conts = e.contSig(l.conts)
	}
	return s
}

// fastKey returns the key of a leaf whose path needs no container nodes,
// and whether it has one: the common paragraphs, headings, cells and code
// blocks of a document, whose style is a lookup of an integer.
func (e *Editor) fastKey(l *leaf, rev bool) (uint64, bool) {
	if len(l.conts) != 0 || l.lang != "" {
		return 0, false
	}
	var flags uint8
	if l.pending {
		flags |= fPending
	}
	if l.setext {
		flags |= fSetext
	}
	if l.frontMatter {
		flags |= fFront
	}
	if l.fenced {
		flags |= fFenced
	}
	if l.level < 0 || l.level >= 128 {
		return 0, false
	}
	var states theme.States
	var tableStates theme.States
	align := uint64(parser.AlignNone)
	row, col := 0, 0
	if l.kind == kPara && l.table != nil {
		if len(l.table.conts) != 0 {
			return 0, false
		}
		row, col = l.rowCol()
		if row < 0 || row >= 1<<16 || col < 0 || col >= 1<<16 {
			return 0, false
		}
		states = e.cellStates(l)
		tableStates = e.leafStates(l.table)
		align = uint64(alignAt(l.table.tab.align, col))
	} else {
		states = e.leafStates(l)
	}
	var shown uint64
	if rev && (l.kind == kCode || l.kind == kMath || l.kind == kHTML || l.kind == kRule) {
		shown = 1
	}
	return uint64(l.kind)<<60 | uint64(l.level)<<53 | uint64(flags)<<45 |
		uint64(states)<<37 | shown<<36 | align<<34 |
		uint64(tableStates&3)<<32 | uint64(row)<<16 | uint64(col), true
}

// styleID returns the id of the leaf's signature, interning signatures so
// that a leaf keeps its id across resolver changes.
func (e *Editor) styleID(l *leaf, rev bool) uint32 {
	if l.sigID != 0 && l.sigRev == rev {
		return l.sigID
	}
	sig := e.styleSigOf(l)
	if e.styleIDs == nil {
		e.styleIDs = map[styleSig]uint32{}
		e.styleSigs = []styleSig{{}}
	}
	id, ok := e.styleIDs[sig]
	if !ok {
		// The signature is kept, so its container signature must be its
		// own, not the editor's reusable buffer.
		if sig.conts != "" {
			sig.conts = strings.Clone(sig.conts)
		}
		e.styleSigs = append(e.styleSigs, sig)
		id = uint32(len(e.styleSigs) - 1)
		e.styleIDs[sig] = id
	}
	l.sigID, l.sigRev = id, rev
	return id
}

// buildStyled makes a leaf's path of nodes and computes its block style.
func (e *Editor) buildStyled(l *leaf) *leafStyled {
	e.nextStyleID++
	s := &leafStyled{id: e.nextStyleID}
	n := e.res.Root()
	if l.kind == kPara && l.table != nil {
		t := l.table
		for _, c := range t.conts {
			n = appendContNode(n, c)
			s.conts = append(s.conts, n)
			s.contStyle = append(s.contStyle, n.Style())
		}
		n = n.Child(e.tableElement(t))
		n = n.Child(e.rowElement(l))
		n = n.Child(e.cellElement(l))
	} else {
		for _, c := range l.conts {
			n = appendContNode(n, c)
			s.conts = append(s.conts, n)
			s.contStyle = append(s.contStyle, n.Style())
		}
		el := e.leafElement(l)
		if l.kind != kTable {
			el.States |= e.leafStates(l)
		}
		n = n.Child(el)
	}
	s.node = n
	s.style = n.Style()
	return s
}

// styledLeaf returns the leaf's path and block style, from the cache when
// the resolver and reveal state are the same.
func (e *Editor) styledLeaf(l *leaf) *leafStyled {
	rev := e.revealAt(l) >= 0
	if l.styled != nil && l.styleGen == e.styleGen && l.styleRev == rev {
		return l.styled
	}
	key, fast := e.fastKey(l, rev)
	if fast {
		if e.fastByGen == nil {
			e.fastByGen = map[uint64]map[uint64]*leafStyled{}
		}
		m := e.fastByGen[e.styleGen]
		if m == nil {
			m = map[uint64]*leafStyled{}
			e.fastByGen[e.styleGen] = m
		}
		if s, ok := m[key]; ok {
			l.styled, l.styleGen, l.styleRev = s, e.styleGen, rev
			return s
		}
		s := e.buildStyled(l)
		m[key] = s
		l.styled, l.styleGen, l.styleRev = s, e.styleGen, rev
		return s
	}
	id := e.styleID(l, rev)
	if e.styledByGen == nil {
		e.styledByGen = map[uint64][]*leafStyled{}
	}
	styles := e.styledByGen[e.styleGen]
	if len(styles) < len(e.styleSigs) {
		styles = append(styles, make([]*leafStyled, len(e.styleSigs)-len(styles))...)
	}
	if s := styles[id]; s != nil {
		l.styled, l.styleGen, l.styleRev = s, e.styleGen, rev
		return s
	}
	s := e.buildStyled(l)
	styles[id] = s
	e.styledByGen[e.styleGen] = styles
	l.styled, l.styleGen, l.styleRev = s, e.styleGen, rev
	return s
}

// tableElement returns the element of a table.
func (e *Editor) tableElement(l *leaf) theme.Element {
	el := theme.Element{Type: "table"}
	el.States |= e.leafStates(l)
	return el
}

// cellElement returns the element of a table cell.
func (e *Editor) cellElement(l *leaf) theme.Element {
	row, col := l.rowCol()
	el := theme.Element{Type: "cell"}
	t := l.table.tab
	if row == 0 {
		el.Classes = append(el.Classes, "header")
	}
	if col >= 0 && col < len(t.align) {
		if name := alignName(t.align[col]); name != "" {
			el.Attrs = append(el.Attrs, theme.Attr{Name: "align", Value: name})
		}
	}
	el.States |= e.cellStates(l)
	return el
}

// cellStates returns a cell's place among the cells of its row.
func (e *Editor) cellStates(l *leaf) theme.States {
	row, col := l.rowCol()
	var st theme.States
	if col == 0 {
		st |= theme.FirstChild
	}
	if rows := l.table.tab.rows; row >= 0 && len(rows) > 0 && col == len(rows[row])-1 {
		st |= theme.LastChild
	}
	return st
}

// rowElementAt returns the element of a table's row.
func rowElementAt(row int) theme.Element {
	el := theme.Element{Type: "row"}
	switch {
	case row == 0:
		el.Classes = append(el.Classes, "header")
	case row%2 == 1:
		el.Classes = append(el.Classes, "odd")
	default:
		el.Classes = append(el.Classes, "even")
	}
	return el
}

// rowElement returns the element of the row holding a cell.
func (e *Editor) rowElement(l *leaf) theme.Element {
	row, _ := l.rowCol()
	return rowElementAt(row)
}

// tableRowNode returns the node of one of a table's rows.
func (e *Editor) tableRowNode(t *leaf, row int) *theme.Node {
	n := e.containerNode(t)
	n = n.Child(e.tableElement(t))
	return n.Child(rowElementAt(row))
}

// styleNode returns the node of a leaf's block, at the end of its path.
func (e *Editor) styleNode(l *leaf) *theme.Node {
	return e.styledLeaf(l).node
}

// leafStyle returns a leaf's block style.
func (e *Editor) leafStyle(l *leaf) *theme.Computed {
	return e.styledLeaf(l).style
}

// leafStyleID returns the identity of a leaf's block style.
func (e *Editor) leafStyleID(l *leaf) uint64 {
	return e.styledLeaf(l).id
}

// textNode returns the node of a leaf's text: its block, or the document
// for a heading being typed.
func (e *Editor) textNode(l *leaf) *theme.Node {
	if l.kind == kHeading && l.pending {
		return e.res.Root()
	}
	return e.styleNode(l)
}

// leafStyleState returns a leaf's block style with extra states, as a
// formula that could not be typeset is :broken.
func (e *Editor) leafStyleState(l *leaf, extra theme.States) *theme.Computed {
	if extra == 0 || l.kind == kPara && l.table != nil {
		return e.leafStyle(l)
	}
	n := e.containerNode(l)
	el := e.leafElement(l)
	if l.kind != kTable {
		el.States |= e.leafStates(l)
	}
	el.States |= extra
	return n.Child(el).Style()
}
