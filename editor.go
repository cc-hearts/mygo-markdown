package livemark

import (
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
)

// Options configure an editor. The app sets them before building the
// editor and changes them as its settings change.
type Options struct {
	// FontSize is the body text's size in DIPs (16 when 0), LineHeight its
	// line height as a multiple of it (1.75 when 0).
	FontSize   float32
	LineHeight float32
	// Font and MonoFont are the families of text and of code: the
	// theme's font and the system's monospaced one when empty.
	Font, MonoFont string
	// MaxWidth caps the column of text (820 when 0); PadX is the least room
	// beside it.
	MaxWidth float32
	PadX     float32
	// BaseDir resolves the relative paths of images.
	BaseDir string
	// Placeholder shows in an empty document.
	Placeholder string
	// Label names the editor to assistive technology.
	Label string
	// Invalidate asks the window for a frame, from any goroutine, as an
	// image loaded in the background does.
	Invalidate func()
	// OnChange is called after every change of the source, the user's and
	// the app's.
	OnChange func()
	// OnLink opens a link the user Cmd-clicked (Ctrl-clicked).
	OnLink func(url string)
	// Code makes the document code rather than Markdown, as a .json or a
	// .go file is: its lines show as they are, highlighted as the language
	// of that name, or plain when the name is "text".
	Code string
	// SaveImage stores an image pasted into the document and returns the
	// path a Markdown image links it by; nil keeps pasted images out.
	SaveImage func(data []byte, ext string) (string, error)
	// OnDropFiles takes the files dropped on the editor that are not
	// images, as a document to open.
	OnDropFiles func(paths []string)
	// ReadClipboard and WriteClipboard replace the system's clipboard, as
	// tests do.
	ReadClipboard  func() transfer.Data
	WriteClipboard func(transfer.Data)
}

// Editor is the state of a Markdown editor, kept by the app across frames:
// the source and its parse, the selection, the history of undo and what
// the editor laid out. View builds it.
type Editor struct {
	opts Options

	src    string
	doc    *parser.Document
	fm     int
	leaves []*leaf
	// rev counts changes of the source; parsed is the rev leaves reflect.
	rev, parsed uint64

	// The selection: its anchor and its caret, byte offsets of the source.
	anchor, caret int
	// phantom is the empty paragraph the caret is in, if it is in one.
	phantom *phantom
	// goalX is the x a run of Up and Down keeps the caret at.
	goalX   float32
	hasGoal bool
	// upstream draws the caret at the end of the line before when it sits
	// where a line wraps.
	upstream bool

	// An input method's composition, shown at the caret until committed;
	// composeSel is its selection, in UTF-16 units of it.
	compose    string
	composeSel ui.TextInputRange
	composing  bool

	hist history

	lk       look
	views    map[viewKey]*leafView
	frame    uint64
	assetGen atomic.Uint64

	// The element's size and where the column of text is in it.
	elemW, viewH float32
	colX, colW   float32
	padTop       float32
	padBottom    float32
	docH         float32

	scroll       ui.ScrollState
	scrollHandle ui.Handle
	handle       ui.Handle
	focused      bool
	blinkStart   time.Time
	// wantReveal scrolls the caret into view at the next frame.
	wantReveal bool
	// scrollTo, when set, scrolls an offset to the top of the view at the
	// next frame.
	scrollTo    int
	hasScrollTo bool

	// The pointer: a drag selecting by characters, words or blocks, from
	// the range it started on; the syntax shown is frozen meanwhile, so
	// that the text does not move under it.
	dragging   bool
	dragUnit   int
	dragFrom   [2]int
	frozen     bool
	frozenAt   int
	pointerX   float32
	pointerY   float32
	hoverLink  string
	services   ui.Services
	lastChange time.Time

	// highlights are ranges of the source drawn as found, as a search's
	// matches; current is the one chosen.
	highlights []Range
	current    int

	// elemRect is the element's box in the window, as the last frame drew it.
	elemRect ui.Rect
	// cursor is the pointer's shape where it rests.
	cursor ui.Cursor

	// phantomStays keeps the phantom an edit made through its commit.
	phantomStays bool
	u16ix        u16Index
	laidKey      laid
	leavesLaid   bool
}

// Range is a range of the source, in bytes.
type Range struct{ Start, End int }

// phantom is an empty paragraph the caret is in: Markdown has no empty
// paragraphs, so it is not in the source until text is typed in it, which
// goes in at at with pre before it and post after it.
type phantom struct {
	at        int
	pre, post string
	conts     []cont
	// before places it above the leaf starting at at, rather than below
	// the one ending there.
	before bool
}

// New returns an editor of src.
func New(src string, opts Options) *Editor {
	e := &Editor{opts: opts, views: map[viewKey]*leafView{}, current: -1, cursor: ui.CursorText}
	// The window's theme comes with the first frame; until then, the light
	// one, so that a document opened before it lays out.
	e.setLook(ui.LightTheme())
	e.SetSource(src)
	return e
}

// SetOptions changes the editor's options.
func (e *Editor) SetOptions(opts Options) {
	if opts.Code != e.opts.Code {
		e.rev++
		e.parsed = 0
	}
	e.opts = opts
}

// Options returns the editor's options.
func (e *Editor) Options() Options { return e.opts }

// Source returns the Markdown being edited.
func (e *Editor) Source() string { return e.src }

// SetSource replaces the document, as opening another file does: the
// caret goes to its start and the history of undo is cleared.
func (e *Editor) SetSource(src string) {
	e.src = src
	e.doc = parser.New(exts)
	e.rev++
	e.anchor, e.caret = 0, 0
	e.phantom = nil
	e.compose, e.composing = "", false
	e.hist = history{}
	e.highlights, e.current = nil, -1
	e.scroll.Y = 0
	e.hasGoal = false
	e.sync()
	if strings.TrimSpace(src) == "" {
		e.phantom = &phantom{at: len(src)}
		e.sync()
	}
	e.caret = e.firstStop()
	e.anchor = e.caret
}

// sync parses the source again when it changed, and makes the leaves.
func (e *Editor) sync() {
	if e.parsed == e.rev && e.leaves != nil {
		return
	}
	if e.opts.Code != "" {
		e.fm = 0
		e.leaves = []*leaf{codeDocLeaf(e.src, e.opts.Code)}
		e.parsed = e.rev
		return
	}
	e.fm = frontMatterEnd(e.src)
	e.doc.Set(cjkMask(maskFrontMatter(e.src, e.fm)))
	e.leaves = buildLeaves(e.src, e.doc, e.fm)
	if p := e.phantom; p != nil {
		e.leaves = insertPhantom(e.leaves, p)
	}
	e.parsed = e.rev
}

// insertPhantom puts the phantom's leaf among the leaves, by its offset.
func insertPhantom(leaves []*leaf, p *phantom) []*leaf {
	pl := &leaf{kind: kPhantom, id: -1, start: p.at, end: p.at, lineStart: p.at, conts: p.conts}
	i := sort.Search(len(leaves), func(i int) bool {
		l := leaves[i]
		if p.before {
			return l.lineStart >= p.at
		}
		return l.lineStart > p.at || (l.lineStart >= p.at && l.end > p.at)
	})
	if !p.before {
		// After every leaf ending at or before it.
		for i < len(leaves) && leaves[i].end <= p.at && leaves[i].lineStart <= p.at && leaves[i].kind != kPhantom {
			i++
		}
	}
	out := make([]*leaf, 0, len(leaves)+1)
	out = append(out, leaves[:i]...)
	out = append(out, pl)
	return append(out, leaves[i:]...)
}

// leafLo returns the first offset the caret can take in a leaf: a fenced
// code block's info string comes before its lines.
func leafLo(l *leaf) int {
	if l.kind == kCode && l.fenced {
		return l.infoStart
	}
	return l.start
}

// leafIndex returns the index of the leaf the caret at off is in: the
// phantom when it is in it, else the leaf whose content holds off, else
// -1.
func (e *Editor) leafIndex(off int) int {
	if e.phantom != nil {
		for i, l := range e.leaves {
			if l.kind == kPhantom && off == l.start {
				return i
			}
		}
	}
	for i, l := range e.leaves {
		if l.kind == kPhantom {
			continue
		}
		if off >= leafLo(l) && off <= l.end {
			if l.kind == kCode && l.fenced && off > l.infoEnd && off < l.start {
				continue
			}
			return i
		}
	}
	return -1
}

// leafAt returns the leaf the caret at off is in, or nil.
func (e *Editor) leafAt(off int) *leaf {
	if i := e.leafIndex(off); i >= 0 {
		return e.leaves[i]
	}
	return nil
}

// cellAt returns the cell of a table holding off, with its row and column.
func cellAt(l *leaf, off int) (*leaf, int, int) {
	if l.tab == nil {
		return nil, -1, -1
	}
	for r, row := range l.tab.rows {
		for c, cell := range row {
			if off >= cell.start && off <= cell.end {
				return cell, r, c
			}
		}
	}
	return nil, -1, -1
}

// selection returns the selection's start and end.
func (e *Editor) selection() (int, int) {
	if e.anchor < e.caret {
		return e.anchor, e.caret
	}
	return e.caret, e.anchor
}

// SelectionRange returns the selected range of the source.
func (e *Editor) SelectionRange() Range {
	a, z := e.selection()
	return Range{a, z}
}

// SelectedText returns the source the selection holds.
func (e *Editor) SelectedText() string {
	a, z := e.selection()
	return e.src[a:z]
}

// revealCaret returns the offset whose syntax shows: the caret's, or where
// it was when a drag began. -1 while the editor does not have the focus.
func (e *Editor) revealCaret() int {
	if !e.focused {
		return -1
	}
	if e.frozen {
		return e.frozenAt
	}
	return e.caret
}

// firstStop returns the first offset the caret can take.
func (e *Editor) firstStop() int {
	for _, l := range e.leaves {
		if s, ok := e.edgeStop(l, false); ok {
			return s
		}
	}
	return 0
}

// lastStop returns the last offset the caret can take.
func (e *Editor) lastStop() int {
	for i := len(e.leaves) - 1; i >= 0; i-- {
		if s, ok := e.edgeStop(e.leaves[i], true); ok {
			return s
		}
	}
	return len(e.src)
}

// setCaret moves the caret to off, keeping the anchor when extend is set,
// and leaves the phantom unless the caret stays in it.
func (e *Editor) setCaret(off int, extend bool) {
	off = max(0, min(off, len(e.src)))
	if e.phantom != nil && !(off == e.phantom.at && e.leafIsPhantom(off)) {
		e.dropPhantom()
	}
	e.caret = off
	if !extend {
		e.anchor = off
	}
	e.wantReveal = true
	e.compose = ""
	e.blinkStart = time.Now()
}

// leafIsPhantom reports whether the caret at off stays in the phantom.
func (e *Editor) leafIsPhantom(off int) bool {
	i := e.leafIndex(off)
	return i >= 0 && e.leaves[i].kind == kPhantom
}

// dropPhantom takes the phantom away, as the caret leaves it.
func (e *Editor) dropPhantom() {
	if e.phantom == nil {
		return
	}
	e.phantom = nil
	e.rev++
	e.parsed = 0
	e.sync()
}

// setPhantom puts the caret in a phantom paragraph.
func (e *Editor) setPhantom(p *phantom) {
	e.phantom = p
	e.rev++
	e.parsed = 0
	e.sync()
	e.caret, e.anchor = p.at, p.at
	e.wantReveal = true
	e.compose = ""
	e.blinkStart = time.Now()
}

// Select selects a range of the source, as a search's match, and scrolls
// to it.
func (e *Editor) Select(start, end int) {
	e.dropPhantom()
	start = max(0, min(start, len(e.src)))
	end = max(start, min(end, len(e.src)))
	e.anchor, e.caret = start, end
	e.wantReveal = true
	e.hasGoal = false
	e.invalidate()
}

// ScrollToOffset scrolls the source offset off to the top of the view and
// puts the caret there, as an outline's heading is gone to.
func (e *Editor) ScrollToOffset(off int) {
	e.dropPhantom()
	off = e.normalize(max(0, min(off, len(e.src))), 1)
	e.anchor, e.caret = off, off
	e.scrollTo, e.hasScrollTo = off, true
	e.invalidate()
}

// SetHighlights draws ranges of the source as found, current the one
// chosen (or -1).
func (e *Editor) SetHighlights(rs []Range, current int) {
	e.highlights, e.current = rs, current
	e.invalidate()
}

// Focus gives the editor the keyboard focus.
func (e *Editor) Focus() { e.handle.Focus() }

// Focused reports whether the editor has the keyboard focus.
func (e *Editor) Focused() bool { return e.focused }

func (e *Editor) invalidate() {
	if e.opts.Invalidate != nil {
		e.opts.Invalidate()
	}
}

// Headings returns the document's headings, in order: their level, text
// and offset.
func (e *Editor) Headings() []Heading {
	e.sync()
	var out []Heading
	for _, l := range e.leaves {
		if l.kind != kHeading || l.pending {
			continue
		}
		text := strings.TrimSpace(e.plainText(e.doc.Inlines(l.id)))
		out = append(out, Heading{Level: l.level, Text: text, Offset: l.start})
	}
	return out
}

// Heading is a heading of the document.
type Heading struct {
	Level  int
	Text   string
	Offset int
}
