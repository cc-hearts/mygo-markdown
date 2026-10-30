package mdoc

import "testing"

func TestParseGantt(t *testing.T) {
	g, ok := parseGantt(`gantt
title 计划
dateFormat YYYY-MM-DD
section 设计
需求 :a1, 2026-10-01, 3d
评审 :after a1, 2d
section 开发
实现 :done, 2026-10-06, 1w
`)
	if !ok || len(g.tasks) != 3 || !g.end.After(g.start) {
		t.Fatalf("gantt = %+v", g)
	}
	if got := g.tasks[1].start.Format("2006-01-02"); got != "2026-10-04" {
		t.Fatalf("after start = %s", got)
	}
}
