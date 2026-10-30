package livemark

import (
	"time"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

// laid is what the last layout was made of: the layout is done again only
// when it changes.
type laid struct {
	rev        uint64
	caret      int
	reveal     int
	compose    string
	colX, colW float32
	styleGen   uint64
	assets     uint64
	phantom    *phantom
	focused    bool
	frame      uint64
}

// layoutIfNeeded lays the document out again when what it shows changed:
// the source, the caret, the width, the theme.
func (e *Editor) layoutIfNeeded() {
	e.sync()
	k := laid{
		rev:      e.rev,
		caret:    e.caret,
		reveal:   e.revealCaret(),
		compose:  e.compose,
		colX:     e.colX,
		colW:     e.colW,
		styleGen: e.styleGen,
		assets:   e.assetGen.Load(),
		phantom:  e.phantom,
		focused:  e.focused,
	}
	if k == e.laidKey && e.leavesLaid {
		return
	}
	e.laidKey = k
	e.leavesLaid = true
	if e.colW <= 0 {
		e.colW = 720
	}
	e.layoutDoc(e.colX, e.colW)
}

// View builds the editor: a scroll view of the document as one surface
// the user writes on.
func (e *Editor) View(c *ui.Context) ui.Element {
	t := c.Theme()
	e.uiFont = t.Font
	e.ensureResolver(t)
	e.frame++
	e.services = c.Services()
	e.focused = e.handle.Focused(c)
	if !e.focused && e.dragging {
		e.dragging, e.frozen = false, false
	}

	b := e.scrollHandle.Bounds(c)
	if b.W > 0 {
		e.elemW, e.viewH = b.W, b.H
	}
	root := e.rootStyle()
	maxW := root.MaxWidth
	if maxW <= 0 {
		maxW = 820
	}
	padX := max(root.Padding.Left, root.Padding.Right)
	if e.elemW > 0 {
		e.colW = max(min(maxW, e.elemW-2*padX), 120)
		e.colX = max((e.elemW-e.colW)/2, 0)
	}
	e.padTop = root.Padding.Top
	e.padBottom = max(e.viewH*0.35, e.baseSize()*4)
	e.layoutIfNeeded()
	e.pruneViews()
	e.scrollIntoView()

	h := max(e.docH, e.viewH)
	sc := ui.Scroll(c).Fill().TrackScroll(&e.scroll).Bind(&e.scrollHandle)
	sc.Children(func() {
		label := e.opts.Label
		if label == "" {
			label = "Document"
		}
		el := ui.Box(c).FillWidth().Height(h).Focusable().FocusRing(false).Bind(&e.handle).
			Cursor(e.cursor).Role(ui.RoleTextField).Label(label).
			HandleTextInput(e).HandleInput(e.input).Draw(e.paint).ContextMenu(e.menu)
		if files := el.DroppedFiles(); len(files) > 0 {
			if x, y, ok := el.PointerPosition(); ok {
				e.setCaret(e.hitTest(x, y), false)
			}
			e.dropFiles(files)
		}
	})
	if e.autoScroll() {
		c.After(16 * time.Millisecond)
	}
	if e.focused {
		phase := c.Now().Sub(e.blinkStart)
		if phase < 30*time.Second {
			c.After(blink - phase%blink)
		}
	}
	if e.elemW <= 0 {
		// The first frame has no size yet: lay out again at the next.
		c.After(time.Millisecond)
	}
	return sc
}

// scrollIntoView scrolls the caret into view after it moved, or a source
// offset to the top for ScrollToOffset.
func (e *Editor) scrollIntoView() {
	if e.viewH <= 0 {
		return
	}
	if e.hasScrollTo {
		e.hasScrollTo = false
		e.wantReveal = false
		if r, ok := e.caretRect(e.scrollTo); ok {
			e.scroll.Y = max(r.Y-e.baseSize()*2, 0)
		}
		return
	}
	if !e.wantReveal {
		return
	}
	e.wantReveal = false
	r, ok := e.caretRect(e.caret)
	if !ok {
		return
	}
	margin := min(e.baseSize()*3, e.viewH/4)
	switch {
	case r.Y < e.scroll.Y+margin:
		e.scroll.Y = max(r.Y-margin, 0)
	case r.Y+r.H > e.scroll.Y+e.viewH-margin:
		e.scroll.Y = r.Y + r.H - e.viewH + margin
	}
}

// textData returns text as clipboard data.
func textData(s string) transfer.Data { return transfer.TextData(s) }
