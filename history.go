package livemark

import (
	"strings"
	"time"
)

// change is an edit of the source: at at, old became new.
type change struct {
	at       int
	old, new string
}

// stepKind tells how a step of undo joins the one before it.
type stepKind uint8

const (
	stepEdit stepKind = iota
	stepTyping
	stepDelete
)

// step is one step of undo: its changes, in the order they were made, and
// the selection before and after them.
type step struct {
	changes             []change
	before, after       [2]int
	phBefore, phAfter   *phantom
	kind                stepKind
	at                  time.Time
	closedForCoalescing bool
}

// history is the editor's undo and redo.
type history struct {
	undo, redo []step
	// open is the step edits are recorded in until it is committed.
	open *step
}

const maxSteps = 500

// begin opens a step of kind for the edits that follow.
func (e *Editor) begin(kind stepKind) {
	if e.hist.open != nil {
		return
	}
	e.hist.open = &step{kind: kind, before: [2]int{e.anchor, e.caret}, phBefore: e.phantom, at: time.Now()}
}

// replace replaces the source's bytes [from, to) with text, recorded in the
// open step.
func (e *Editor) replace(from, to int, text string) {
	from = max(0, min(from, len(e.src)))
	to = max(from, min(to, len(e.src)))
	old := e.src[from:to]
	if old == text {
		return
	}
	e.src = e.src[:from] + text + e.src[to:]
	e.rev++
	if e.hist.open == nil {
		e.begin(stepEdit)
	}
	e.hist.open.changes = append(e.hist.open.changes, change{at: from, old: old, new: text})
	// The selection follows the text: an offset after the change moves
	// with it.
	shift := func(p int) int {
		switch {
		case p >= to:
			return p + len(text) - (to - from)
		case p > from:
			return from + len(text)
		}
		return p
	}
	e.anchor, e.caret = shift(e.anchor), shift(e.caret)
	for i := range e.highlights {
		e.highlights[i].Start, e.highlights[i].End = shift(e.highlights[i].Start), shift(e.highlights[i].End)
	}
}

// commit closes the open step with the caret at caret (and the anchor
// there too, or at anchor when it differs), parses the source and tells
// the app.
func (e *Editor) commit(anchor, caret int) {
	s := e.hist.open
	e.hist.open = nil
	e.anchor, e.caret = anchor, caret
	if e.phantom != nil && e.rev != e.parsed {
		// Text came in around the phantom: it is no longer empty.
		if !(caret == e.phantom.at && e.phantomStays) {
			e.phantom = nil
		}
	}
	e.phantomStays = false
	e.parsed = 0
	e.sync()
	e.caret = e.normalize(e.caret, 1)
	if anchor == caret {
		e.anchor = e.caret
	}
	e.wantReveal = true
	e.hasGoal = false
	e.compose = ""
	if s == nil || len(s.changes) == 0 {
		return
	}
	s.after = [2]int{e.anchor, e.caret}
	s.phAfter = e.phantom
	e.hist.redo = nil
	if n := len(e.hist.undo); n > 0 && e.coalesces(&e.hist.undo[n-1], s) {
		prev := &e.hist.undo[n-1]
		prev.changes = append(prev.changes, s.changes...)
		prev.after, prev.phAfter, prev.at = s.after, s.phAfter, s.at
	} else {
		e.hist.undo = append(e.hist.undo, *s)
		if len(e.hist.undo) > maxSteps {
			e.hist.undo = e.hist.undo[len(e.hist.undo)-maxSteps:]
		}
	}
	e.changed()
}

// coalesces reports whether step s joins prev as one step of undo: typing
// on, a word at a time, or deleting on.
func (e *Editor) coalesces(prev, s *step) bool {
	if prev.kind != s.kind || s.kind == stepEdit || prev.closedForCoalescing {
		return false
	}
	if s.at.Sub(prev.at) > 2*time.Second || len(s.changes) != 1 || len(prev.changes) == 0 {
		return false
	}
	c, p := s.changes[0], prev.changes[len(prev.changes)-1]
	switch s.kind {
	case stepTyping:
		if c.old != "" || strings.ContainsRune(c.new, '\n') || c.at != p.at+len(p.new) {
			return false
		}
		// A new word starts a new step.
		if strings.HasSuffix(p.new, " ") && !strings.HasPrefix(c.new, " ") {
			return false
		}
		return true
	case stepDelete:
		return c.new == "" && (c.at+len(c.old) == p.at || c.at == p.at)
	}
	return false
}

// breakCoalescing makes the next edit a step of its own.
func (e *Editor) breakCoalescing() {
	if n := len(e.hist.undo); n > 0 {
		e.hist.undo[n-1].closedForCoalescing = true
	}
}

// changed tells the app the source changed.
func (e *Editor) changed() {
	e.lastChange = time.Now()
	if e.opts.OnChange != nil {
		e.opts.OnChange()
	}
}

// Undo takes back the last step.
func (e *Editor) Undo() bool {
	n := len(e.hist.undo)
	if n == 0 {
		return false
	}
	s := e.hist.undo[n-1]
	e.hist.undo = e.hist.undo[:n-1]
	for i := len(s.changes) - 1; i >= 0; i-- {
		c := s.changes[i]
		e.src = e.src[:c.at] + c.old + e.src[c.at+len(c.new):]
	}
	e.hist.redo = append(e.hist.redo, s)
	e.restore(s.before, s.phBefore)
	return true
}

// Redo makes the last step undone again.
func (e *Editor) Redo() bool {
	n := len(e.hist.redo)
	if n == 0 {
		return false
	}
	s := e.hist.redo[n-1]
	e.hist.redo = e.hist.redo[:n-1]
	for _, c := range s.changes {
		e.src = e.src[:c.at] + c.new + e.src[c.at+len(c.old):]
	}
	s.closedForCoalescing = true
	e.hist.undo = append(e.hist.undo, s)
	e.restore(s.after, s.phAfter)
	return true
}

// restore puts the selection and the phantom back as a step had them.
func (e *Editor) restore(sel [2]int, ph *phantom) {
	e.rev++
	e.phantom = ph
	e.parsed = 0
	e.sync()
	e.anchor = max(0, min(sel[0], len(e.src)))
	e.caret = max(0, min(sel[1], len(e.src)))
	e.compose = ""
	e.wantReveal = true
	e.hasGoal = false
	e.changed()
	e.invalidate()
}

// CanUndo and CanRedo report whether there is a step to take back or to
// make again.
func (e *Editor) CanUndo() bool { return len(e.hist.undo) > 0 }
func (e *Editor) CanRedo() bool { return len(e.hist.redo) > 0 }
