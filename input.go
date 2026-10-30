package livemark

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
)

// input takes the editor's input as it comes: keys, text typed, the edit
// commands of the menus, and the pointer.
func (e *Editor) input(ev ui.InputEvent) bool {
	defer e.invalidate()
	switch ev.Kind {
	case ui.InputKeyDown:
		if e.key(ev.Key, ev.Mods) {
			e.blinkStart = time.Now()
			return true
		}
		return false
	case ui.InputText:
		if ev.Text == "" {
			return false
		}
		e.compose = ""
		e.typeText(ev.Text)
		e.blinkStart = time.Now()
		return true
	case ui.InputCompose:
		e.compose = ev.Text
		n := ui.UTF16Len(ev.Text)
		c := min(ui.UTF16Len(string([]rune(ev.Text)[:min(ev.Caret, len([]rune(ev.Text)))])), n)
		e.composeSel = ui.TextInputRange{Start: c, End: c}
		return true
	case ui.InputCommand:
		return e.command(ev.Text)
	case ui.InputPointerDown:
		return e.press(ev)
	case ui.InputPointerMove:
		return e.drag(ev)
	case ui.InputPointerUp:
		if e.dragging {
			e.dragging = false
			if e.frozen {
				e.frozen = false
				e.caret = e.normalize(e.caret, 1)
			}
			return true
		}
		return false
	}
	return false
}

// command runs an edit command of the menus.
func (e *Editor) command(cmd string) bool {
	switch cmd {
	case "copy":
		e.Copy()
	case "cut":
		e.Cut()
	case "paste":
		e.Paste(false)
	case "pasteAndMatchStyle":
		e.Paste(true)
	case "selectAll":
		e.SelectAll()
	case "undo":
		e.Undo()
	case "redo":
		e.Redo()
	case "delete":
		e.deleteSelection()
	case "bold":
		e.ToggleBold()
	case "italic":
		e.ToggleItalic()
	case "underline":
		e.ToggleUnderline()
	case "strikethrough":
		e.ToggleStrike()
	default:
		return false
	}
	return true
}

// SelectAll selects the whole document.
func (e *Editor) SelectAll() {
	e.dropPhantom()
	e.anchor, e.caret = 0, len(e.src)
	e.wantReveal = false
	e.invalidate()
}

// key handles a key pressed, and reports whether it took it.
func (e *Editor) key(key ui.Key, mods ui.Modifiers) bool {
	shift := mods&ui.Shift != 0
	m := mods &^ ui.Shift
	word := (isMac && m == ui.Alt) || (!isMac && m == ui.Ctrl)
	if m == ui.Cmd || m == ui.Cmd|ui.Alt {
		if e.shortcut(key, shift, m&ui.Alt != 0) {
			return true
		}
	}
	switch key {
	case ui.KeyLeft, ui.KeyRight:
		dir := 1
		if key == ui.KeyLeft {
			dir = -1
		}
		switch {
		case isMac && m == ui.Cmd:
			s, z := e.lineEdges(e.caret)
			if dir < 0 {
				e.setCaret(s, shift)
			} else {
				e.setCaret(z, shift)
			}
		case word:
			e.setCaret(e.step(e.caret, dir, true), shift)
		case m != 0:
			return false
		case e.anchor != e.caret && !shift:
			a, z := e.selection()
			if dir < 0 {
				e.setCaret(a, false)
			} else {
				e.setCaret(z, false)
			}
		default:
			e.setCaret(e.step(e.caret, dir, false), shift)
		}
		e.hasGoal = false
		e.upstream = false
		return true
	case ui.KeyUp, ui.KeyDown:
		dir := 1
		if key == ui.KeyUp {
			dir = -1
		}
		if isMac && m == ui.Cmd {
			if dir < 0 {
				e.setCaret(e.firstStop(), shift)
			} else {
				e.setCaret(e.lastStop(), shift)
			}
			return true
		}
		if m != 0 {
			return false
		}
		e.vertical(dir, shift)
		return true
	case ui.KeyHome, ui.KeyEnd:
		if m == ui.Ctrl || m == ui.Cmd {
			if key == ui.KeyHome {
				e.setCaret(e.firstStop(), shift)
			} else {
				e.setCaret(e.lastStop(), shift)
			}
			return true
		}
		s, z := e.lineEdges(e.caret)
		if key == ui.KeyHome {
			e.setCaret(s, shift)
			e.upstream = false
		} else {
			e.setCaret(z, shift)
			e.upstream = true
		}
		e.hasGoal = false
		return true
	case ui.KeyPageUp, ui.KeyPageDown:
		e.page(key == ui.KeyPageDown, shift)
		return true
	case ui.KeyBackspace:
		if isMac && m == ui.Cmd {
			s, _ := e.lineEdges(e.caret)
			if s < e.caret {
				e.anchor = s
				e.deleteSelection()
			}
			return true
		}
		e.backspace(word)
		return true
	case ui.KeyDelete:
		e.deleteForward(word)
		return true
	case ui.KeyEnter:
		if m == ui.Cmd {
			if t, r, c := e.cellPos(e.caret); t != nil {
				e.insertRow(t, r, c)
				return true
			}
			return false
		}
		if m != 0 {
			return false
		}
		e.enter(shift)
		return true
	case ui.KeyTab:
		if m != 0 {
			return false
		}
		return e.tab(shift)
	case ui.KeyEscape:
		if e.anchor != e.caret {
			e.anchor = e.caret
			return true
		}
		return false
	}
	return false
}

// shortcut runs the editor's own shortcut of Cmd (Ctrl) with key.
func (e *Editor) shortcut(key ui.Key, shift, alt bool) bool {
	if alt {
		// Cmd+Alt (Ctrl+Alt): the blocks, and strikethrough as Typora's
		// Alt+Shift+5.
		switch {
		case key == ui.KeyX && !shift:
			e.ToggleList("- [ ]")
		case key == ui.KeyC && !shift:
			e.InsertCodeBlock()
		case key == ui.KeyT && !shift:
			e.InsertTable(3, 3)
		case key == ui.Key5 && shift:
			e.ToggleStrike()
		default:
			return false
		}
		return true
	}
	switch {
	case key == ui.KeyZ && !shift:
		e.Undo()
	case key == ui.KeyZ && shift, key == ui.KeyY && !isMac:
		e.Redo()
	case key == ui.KeyA && !shift:
		e.SelectAll()
	case key == ui.KeyC && !shift:
		e.Copy()
	case key == ui.KeyX && !shift:
		e.Cut()
	case key == ui.KeyV:
		e.Paste(shift)
	case key == ui.KeyB && !shift:
		e.ToggleBold()
	case key == ui.KeyI && !shift:
		e.ToggleItalic()
	case key == ui.KeyU && !shift:
		e.ToggleUnderline()
	case key == ui.KeyE && !shift, key == ui.KeyBackquote && shift:
		e.ToggleCode()
	case key == ui.KeyX && shift:
		e.ToggleStrike()
	case key == ui.KeyK && !shift:
		e.InsertLink()
	case key == ui.KeyD && !shift:
		a, z := e.wordRangeAt(e.caret)
		e.Select(a, z)
	case key >= ui.Key0 && key <= ui.Key6 && !shift:
		e.SetHeading(int(key - ui.Key0))
	case key == ui.KeyEqual && !shift:
		e.ShiftHeading(1)
	case key == ui.KeyMinus && !shift:
		e.ShiftHeading(-1)
	case key == ui.KeyQ && shift:
		e.ToggleQuote()
	case key == ui.KeyBracketLeft && shift:
		e.ToggleList("1.")
	case key == ui.KeyBracketRight && shift:
		e.ToggleList("-")
	case key == ui.KeyK && shift:
		e.InsertCodeBlock()
	case key == ui.KeyM && shift:
		e.InsertMathBlock()
	case key == ui.KeyEnter && !shift:
		if t, r, c := e.cellPos(e.caret); t != nil {
			e.insertRow(t, r, c)
		} else {
			return false
		}
	default:
		return false
	}
	return true
}

// page moves the caret a view's height up or down.
func (e *Editor) page(down bool, extend bool) {
	r, ok := e.caretRect(e.caret)
	if !ok {
		return
	}
	if !e.hasGoal {
		e.goalX, e.hasGoal = r.X, true
	}
	h := max(e.viewH-e.baseSize()*3, e.baseSize()*3)
	y := r.Y + r.H/2 - h
	if down {
		y = r.Y + r.H/2 + h
	}
	off := e.hitTest(e.goalX, y)
	g := e.goalX
	e.setCaretAtPoint(off, y, extend)
	e.goalX, e.hasGoal = g, true
	if down {
		e.scroll.Y += h
	} else {
		e.scroll.Y = max(e.scroll.Y-h, 0)
	}
}

// press takes a press of the pointer: it puts the caret there, selects
// the word of a double click and the block of a triple one, and starts a
// drag selecting from there; it checks a task's box, opens a link
// Cmd-clicked and goes to a code block's language.
func (e *Editor) press(ev ui.InputEvent) bool {
	if ev.Button != 0 {
		return false
	}
	e.layoutIfNeeded()
	x, y := ev.X, ev.Y
	if it, ok := e.taskBoxAt(x, y); ok {
		e.toggleTaskAt(it)
		return true
	}
	if l := e.codeLabelAt(x, y); l != nil {
		e.setCaret(l.infoEnd, false)
		return true
	}
	off := e.hitTest(x, y)
	if ev.Mods&ui.Cmd != 0 {
		if url := e.linkAt(off); url != "" {
			if e.opts.OnLink != nil {
				e.opts.OnLink(url)
			}
			return true
		}
	}
	// Below the last leaf, a paragraph comes after a block that is not
	// text.
	if len(e.leaves) > 0 && ev.Clicks <= 1 && ev.Mods&ui.Shift == 0 {
		last := e.leaves[len(e.leaves)-1]
		if y > last.y+last.h+e.baseSize()*0.5 && e.phantomAtEdge(len(e.leaves)-1, 1) {
			return true
		}
	}
	e.dragging = true
	e.hasGoal = false
	switch {
	case ev.Clicks >= 3:
		a, z := e.blockRangeAt(off)
		e.dropPhantom()
		e.anchor, e.caret = a, z
		e.dragUnit, e.dragFrom = 2, [2]int{a, z}
	case ev.Clicks == 2:
		a, z := e.wordRangeAt(off)
		e.dropPhantom()
		e.anchor, e.caret = a, z
		e.dragUnit, e.dragFrom = 1, [2]int{a, z}
	default:
		extend := ev.Mods&ui.Shift != 0
		e.setCaretAtPoint(off, y, extend)
		e.dragUnit, e.dragFrom = 0, [2]int{e.anchor, e.anchor}
	}
	e.frozen, e.frozenAt = true, e.caret
	e.blinkStart = time.Now()
	return true
}

// drag extends the selection to the pointer, by the unit the press chose.
func (e *Editor) drag(ev ui.InputEvent) bool {
	e.pointerX, e.pointerY = ev.X, ev.Y
	if !e.dragging || ev.Button != 0 {
		e.hover(ev.X, ev.Y, ev.Mods)
		return false
	}
	e.dragTo(ev.X, ev.Y)
	return true
}

// dragTo extends the selection of a drag to the pointer at x, y in the
// element.
func (e *Editor) dragTo(x, y float32) {
	off := e.hitTest(x, y)
	a, z := off, off
	switch e.dragUnit {
	case 1:
		a, z = e.wordRangeAt(off)
	case 2:
		a, z = e.blockRangeAt(off)
	}
	if a < e.dragFrom[0] {
		e.anchor, e.caret = e.dragFrom[1], a
	} else {
		e.anchor, e.caret = e.dragFrom[0], max(z, e.dragFrom[1])
		if e.dragUnit == 0 {
			e.caret = off
		}
	}
}

// autoScroll scrolls the view while a drag holds the pointer above or below
// it, faster the farther, and extends the selection with it. It reports
// whether it scrolled, for another frame to scroll on.
func (e *Editor) autoScroll() bool {
	if !e.dragging || e.viewH <= 0 {
		return false
	}
	top, bottom := e.scroll.Y, e.scroll.Y+e.viewH
	var d float32
	switch {
	case e.pointerY < top:
		d = max((e.pointerY-top)/3, -40)
	case e.pointerY > bottom:
		d = min((e.pointerY-bottom)/3, 40)
	default:
		return false
	}
	y := max(min(e.scroll.Y+d, e.docH-e.viewH), 0)
	if y == e.scroll.Y {
		return false
	}
	e.pointerY += y - e.scroll.Y
	e.scroll.Y = y
	e.dragTo(e.pointerX, e.pointerY)
	return true
}

// hover sets the pointer's shape for where it rests: a hand over what a
// click acts on (a check box, a code block's language, a link with Cmd
// held), an I-beam over the text.
func (e *Editor) hover(x, y float32, mods ui.Modifiers) {
	cursor := ui.CursorText
	if _, ok := e.taskBoxAt(x, y); ok || e.codeLabelAt(x, y) != nil {
		cursor = ui.CursorPointer
	} else if mods&ui.Cmd != 0 && e.linkAt(e.hitTest(x, y)) != "" {
		cursor = ui.CursorPointer
	}
	if cursor != e.cursor {
		e.cursor = cursor
		e.invalidate()
	}
}

// taskBoxAt returns the item whose check box is at x, y.
func (e *Editor) taskBoxAt(x, y float32) (cont, bool) {
	for _, l := range e.leaves {
		if y < l.y-4 || y > l.y+e.baseSize()*e.baseLine()+4 {
			continue
		}
		for _, k := range l.firstOf {
			c := l.conts[k]
			if c.kind != cItem || c.task == parser.NoTask {
				continue
			}
			x0 := e.colX + e.indentOf(l.conts[:k])
			w := e.indentOf(l.conts[k : k+1])
			if x >= x0 && x <= x0+w {
				return c, true
			}
		}
	}
	return cont{}, false
}

// codeLabelAt returns the code block whose language label is at x, y.
func (e *Editor) codeLabelAt(x, y float32) *leaf {
	for _, l := range e.leaves {
		if l.kind != kCode || !l.fenced || l.view == nil || l.view.reveal {
			continue
		}
		if y >= l.y && y <= l.y+e.baseSize()*1.4 && x >= l.x+l.w-e.baseSize()*6 && x <= l.x+l.w {
			if l.lang != "" || e.leafAt(e.caret) == l {
				return l
			}
		}
	}
	return nil
}

// linkAt returns the destination of the link at off.
func (e *Editor) linkAt(off int) string {
	u := e.unitAt(off)
	if u == nil || u.id < 0 || !(u.kind == kPara || u.kind == kHeading) {
		return ""
	}
	for _, in := range e.doc.Inlines(u.id) {
		if (in.Kind == parser.Link || in.Kind == parser.Image) && in.Start <= off && off <= in.End {
			return in.Dest
		}
	}
	return ""
}

// menu builds the editor's context menu.
func (e *Editor) menu(m *ui.Menu) {
	a, z := e.selection()
	if m.Item("剪切").Shortcut(ui.Cmd, ui.KeyX).Disabled(a == z).Chosen() {
		e.Cut()
	}
	if m.Item("复制").Shortcut(ui.Cmd, ui.KeyC).Disabled(a == z).Chosen() {
		e.Copy()
	}
	if m.Item("粘贴").Shortcut(ui.Cmd, ui.KeyV).Chosen() {
		e.Paste(false)
	}
	if m.Item("粘贴为纯文本").Shortcut(ui.Cmd|ui.Shift, ui.KeyV).Chosen() {
		e.Paste(true)
	}
	m.Separator()
	if url := e.linkAt(e.caret); url != "" {
		if m.Item("打开链接").Chosen() && e.opts.OnLink != nil {
			e.opts.OnLink(url)
		}
		if m.Item("复制链接地址").Chosen() {
			if e.opts.WriteClipboard != nil {
				e.opts.WriteClipboard(textData(url))
			} else {
				e.services.WriteClipboard(url)
			}
		}
		m.Separator()
	}
	if t, _, _ := e.cellPos(e.caret); t != nil {
		m.Submenu("表格", func(m *ui.Menu) {
			for _, it := range []struct{ label, cmd string }{
				{"在上方插入行", "rowAbove"}, {"在下方插入行", "rowBelow"}, {"删除行", "deleteRow"},
				{"", ""},
				{"在左侧插入列", "colLeft"}, {"在右侧插入列", "colRight"}, {"删除列", "deleteCol"},
				{"", ""},
				{"左对齐", "alignLeft"}, {"居中", "alignCenter"}, {"右对齐", "alignRight"},
				{"", ""},
				{"删除表格", "deleteTable"},
			} {
				if it.label == "" {
					m.Separator()
					continue
				}
				if m.Item(it.label).Chosen() {
					e.TableCommand(it.cmd)
				}
			}
		})
	}
	m.Submenu("段落", func(m *ui.Menu) {
		lv := e.headingLevel()
		for i := 1; i <= 6; i++ {
			if m.Item("标题 "+string(rune('0'+i))).Shortcut(ui.Cmd, ui.Key0+ui.Key(i)).Checked(lv == i).Chosen() {
				e.SetHeading(i)
			}
		}
		if m.Item("正文").Shortcut(ui.Cmd, ui.Key0).Checked(lv == 0).Chosen() {
			e.SetHeading(0)
		}
		m.Separator()
		if m.Item("引用").Shortcut(ui.Cmd|ui.Shift, ui.KeyQ).Chosen() {
			e.ToggleQuote()
		}
		if m.Item("有序列表").Shortcut(ui.Cmd|ui.Shift, ui.KeyBracketLeft).Chosen() {
			e.ToggleList("1.")
		}
		if m.Item("无序列表").Shortcut(ui.Cmd|ui.Shift, ui.KeyBracketRight).Chosen() {
			e.ToggleList("-")
		}
		if m.Item("任务列表").Chosen() {
			e.ToggleList("- [ ]")
		}
		m.Separator()
		if m.Item("代码块").Shortcut(ui.Cmd|ui.Shift, ui.KeyK).Chosen() {
			e.InsertCodeBlock()
		}
		if m.Item("公式块").Shortcut(ui.Cmd|ui.Shift, ui.KeyM).Chosen() {
			e.InsertMathBlock()
		}
		if m.Item("表格").Shortcut(ui.Cmd|ui.Alt, ui.KeyT).Chosen() {
			e.InsertTable(3, 3)
		}
		if m.Item("分隔线").Chosen() {
			e.InsertRule()
		}
	})
	m.Submenu("格式", func(m *ui.Menu) {
		if m.Item("加粗").Shortcut(ui.Cmd, ui.KeyB).Chosen() {
			e.ToggleBold()
		}
		if m.Item("斜体").Shortcut(ui.Cmd, ui.KeyI).Chosen() {
			e.ToggleItalic()
		}
		if m.Item("下划线").Shortcut(ui.Cmd, ui.KeyU).Chosen() {
			e.ToggleUnderline()
		}
		if m.Item("删除线").Shortcut(ui.Cmd|ui.Shift, ui.KeyX).Chosen() {
			e.ToggleStrike()
		}
		if m.Item("行内代码").Shortcut(ui.Cmd, ui.KeyE).Chosen() {
			e.ToggleCode()
		}
		if m.Item("链接").Shortcut(ui.Cmd, ui.KeyK).Chosen() {
			e.InsertLink()
		}
	})
	m.Separator()
	if m.Item("全选").Shortcut(ui.Cmd, ui.KeyA).Chosen() {
		e.SelectAll()
	}
}
