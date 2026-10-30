package mermaid

import (
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/livemark/render"
)

// ganttChart is a native Gantt subset: sections, dated tasks, after-id
// dependencies, day/week/hour durations, done/active/crit flags and
// milestones. Excludes and axisFormat are ignored.
type ganttChart struct {
	title    string
	format   string
	tasks    []*ganttTask
	sections []string
	rows     []ganttRow
	start    time.Time
	end      time.Time
}

type ganttTask struct {
	name      string
	id        string
	section   string
	startSpec string
	duration  time.Duration
	start     time.Time
	done      bool
	active    bool
	crit      bool
	milestone bool
}

type ganttRow struct {
	section string
	task    *ganttTask
}

func parseGantt(src string) (*ganttChart, bool) {
	lines := strings.Split(src, "\n")
	if len(lines) == 0 || !strings.EqualFold(strings.TrimSpace(lines[0]), "gantt") {
		return nil, false
	}
	g := &ganttChart{format: "2006-01-02"}
	section := "任务"
	for _, raw := range lines[1:] {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "%%") {
			continue
		}
		low := strings.ToLower(line)
		switch {
		case strings.HasPrefix(low, "title "):
			g.title = strings.TrimSpace(line[len("title "):])
			continue
		case strings.HasPrefix(low, "dateformat "):
			g.format = dateLayout(strings.TrimSpace(line[len("dateformat "):]))
			continue
		case strings.HasPrefix(low, "axisformat "), strings.HasPrefix(low, "excludes "),
			strings.HasPrefix(low, "todaymarker"), strings.HasPrefix(low, "tickinterval"),
			strings.HasPrefix(low, "weekday"), strings.HasPrefix(low, "inclusiveenddates"):
			continue
		case strings.HasPrefix(low, "section "):
			section = strings.TrimSpace(line[len("section "):])
			if !containsString(g.sections, section) {
				g.sections = append(g.sections, section)
			}
			continue
		}
		colon := strings.Index(line, ":")
		if colon < 0 {
			continue
		}
		name := strings.TrimSpace(line[:colon])
		fields := strings.Split(line[colon+1:], ",")
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}
		if len(fields) < 2 {
			continue
		}
		task := &ganttTask{name: name, section: section}
		for _, f := range fields[:len(fields)-2] {
			fl := strings.ToLower(f)
			switch fl {
			case "done":
				task.done = true
			case "active":
				task.active = true
			case "crit":
				task.crit = true
			case "milestone":
				task.milestone = true
			default:
				if task.id == "" {
					task.id = f
				}
			}
		}
		task.startSpec = fields[len(fields)-2]
		task.duration = parseGanttDuration(fields[len(fields)-1])
		g.tasks = append(g.tasks, task)
	}
	if len(g.tasks) == 0 {
		return nil, false
	}
	// Resolve dates and after-id starts.
	for pass := 0; pass < len(g.tasks)+1; pass++ {
		for _, task := range g.tasks {
			if !task.start.IsZero() {
				continue
			}
			spec := strings.ToLower(task.startSpec)
			if strings.HasPrefix(spec, "after ") {
				id := strings.TrimSpace(strings.TrimPrefix(spec, "after "))
				if dep := g.taskByID(id); dep != nil && !dep.start.IsZero() {
					task.start = dep.start.Add(dep.duration)
				}
				continue
			}
			if strings.HasPrefix(spec, "until ") {
				continue
			}
			if strings.EqualFold(spec, "today") {
				task.start = time.Now().Truncate(24 * time.Hour)
				continue
			}
			if t, err := time.Parse(g.format, task.startSpec); err == nil {
				task.start = t
			}
		}
	}
	for _, task := range g.tasks {
		if task.start.IsZero() {
			continue
		}
		if g.start.IsZero() || task.start.Before(g.start) {
			g.start = task.start
		}
		if task.duration <= 0 && task.milestone {
			task.duration = 0
		}
		end := task.start.Add(task.duration)
		if end.After(g.end) {
			g.end = end
		}
	}
	if g.start.IsZero() || !g.end.After(g.start) {
		return nil, false
	}
	// Rows: one section label followed by its tasks.
	for _, sec := range g.sections {
		g.rows = append(g.rows, ganttRow{section: sec})
		for _, task := range g.tasks {
			if task.section == sec {
				g.rows = append(g.rows, ganttRow{task: task})
			}
		}
	}
	for _, task := range g.tasks {
		if !containsString(g.sections, task.section) {
			g.sections = append(g.sections, task.section)
			g.rows = append(g.rows, ganttRow{section: task.section})
			g.rows = append(g.rows, ganttRow{task: task})
		}
	}
	return g, true
}

func (g *ganttChart) taskByID(id string) *ganttTask {
	id = strings.TrimSpace(id)
	for _, t := range g.tasks {
		if t.id == id {
			return t
		}
	}
	return nil
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func dateLayout(s string) string {
	switch strings.ToUpper(s) {
	case "YYYY-MM-DD", "YYYY/MM/DD":
		out := strings.ToUpper(s)
		out = strings.ReplaceAll(out, "YYYY", "2006")
		out = strings.ReplaceAll(out, "MM", "01")
		out = strings.ReplaceAll(out, "DD", "02")
		return out
	}
	if strings.Contains(s, "/") {
		return "2006/01/02"
	}
	return "2006-01-02"
}

func parseGanttDuration(s string) time.Duration {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0
	}
	unit := s[len(s)-1]
	number := s
	mult := 24 * float64(time.Hour)
	switch unit {
	case 'd':
		number = s[:len(s)-1]
	case 'w':
		number = s[:len(s)-1]
		mult *= 7
	case 'h':
		number = s[:len(s)-1]
		mult = float64(time.Hour)
	case 'm':
		number = s[:len(s)-1]
		mult *= 30
	default:
		unit = 'd'
	}
	n, err := strconv.ParseFloat(strings.TrimSpace(number), 64)
	if err != nil || n < 0 {
		return 0
	}
	return time.Duration(n * mult)
}

func (g *ganttChart) height() float32 {
	h := float32(20)
	if g.title != "" {
		h += 24
	}
	h += 26 // axis
	for range g.rows {
		h += 26
	}
	return h + 20
}

// draw paints the diagram in r.
func (g *ganttChart) draw(p *ui.Painter, r ui.Rect, s render.DiagramStyle) {
	y := r.Y + 12
	if g.title != "" {
		sp := ui.Span{Text: g.title, Size: 13, Weight: 650, Color: s.Text}
		p.RichText(r.X+16, y, 0, sp)
		y += 24
	}
	labelW := float32(160)
	timelineX := r.X + labelW
	timelineW := r.W - labelW - 20
	if timelineW < 120 {
		timelineW = 120
	}
	days := float32(g.end.Sub(g.start).Hours() / 24)
	if days <= 0 {
		days = 1
	}
	pxPerDay := timelineW / days

	// Date ticks.
	ticks := 5
	for i := 0; i <= ticks; i++ {
		x := timelineX + timelineW*float32(i)/float32(ticks)
		p.Line(x, y, x, y+float32(len(g.rows))*26+4, 1, s.Line)
		label := g.start.Add(time.Duration(float64(days) * float64(i) / float64(ticks) * float64(24*time.Hour))).Format("01-02")
		sp := ui.Span{Text: label, Size: 9.5, Color: s.Muted}
		p.RichText(x+3, y-16, 0, sp)
	}
	y += 8

	rowY := y
	for _, row := range g.rows {
		if row.task == nil {
			sp := ui.Span{Text: row.section, Size: 11, Weight: 650, Color: s.Text}
			p.RichText(r.X+16, rowY+5, labelW-24, sp)
			rowY += 26
			continue
		}
		task := row.task
		sp := ui.Span{Text: task.name, Size: 10.5, Color: s.Text}
		p.RichText(r.X+24, rowY+6, labelW-32, sp)
		offset := float32(task.start.Sub(g.start).Hours() / 24)
		x := timelineX + offset*pxPerDay
		w := float32(task.duration.Hours()/24) * pxPerDay
		if task.milestone {
			path := ui.Path{}
			path.MoveTo(x+5, rowY+5)
			path.LineTo(x+10, rowY+13)
			path.LineTo(x+5, rowY+21)
			path.LineTo(x, rowY+13)
			path.Close()
			p.FillPath(&path, s.Muted)
			rowY += 26
			continue
		}
		if w < 4 {
			w = 4
		}
		color := ui.Hex(piePalette[0])
		if task.done {
			color = ui.Hex("#6aa84f")
		} else if task.active {
			color = ui.Hex("#4f7cff")
		} else if task.crit {
			color = ui.Hex("#e5679b")
		}
		p.Fill(ui.Rect{X: x, Y: rowY + 7, W: w, H: 12}, color, 3)
		rowY += 26
	}
}
