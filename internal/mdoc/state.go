package mdoc

import (
	"strings"
)

// parseState turns a stateDiagram into the same node/edge model used by the
// native flowchart renderer. Composite state bodies and notes are skipped;
// the transitions inside them are still read when they are plain lines.
func parseState(src string) (*mermaidGraph, bool) {
	lines := strings.Split(src, "\n")
	if len(lines) == 0 {
		return nil, false
	}
	head := strings.ToLower(strings.TrimSpace(lines[0]))
	if head != "statediagram" && head != "statediagram-v2" {
		return nil, false
	}
	g := &mermaidGraph{dir: "TD", byID: map[string]*mermaidNode{}}
	for _, raw := range lines[1:] {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "%%") {
			continue
		}
		low := strings.ToLower(line)
		if strings.HasPrefix(low, "direction ") {
			dir := strings.ToUpper(strings.TrimSpace(line[len("direction "):]))
			if dir == "LR" || dir == "RL" || dir == "TD" || dir == "TB" || dir == "BT" {
				g.dir = dir
			}
			continue
		}
		if strings.HasPrefix(low, "state ") {
			rest := strings.TrimSpace(line[len("state "):])
			if strings.HasPrefix(rest, `"`) {
				if end := strings.Index(rest[1:], `"`); end >= 0 {
					label := rest[1 : end+1]
					after := strings.TrimSpace(rest[end+2:])
					if strings.HasPrefix(strings.ToLower(after), "as ") {
						id := strings.TrimSpace(after[3:])
						if n := g.node(id); n != nil {
							n.label = label
						}
					}
				}
				continue
			}
			// Ignore the opening of a composite state body; its transitions
			// are still picked up below.
			id := rest
			if i := strings.IndexAny(id, " {"); i >= 0 {
				id = id[:i]
			}
			g.node(strings.TrimSpace(id))
			continue
		}
		skip := false
		for _, p := range []string{"note ", "classdef", "class ", "style "} {
			if strings.HasPrefix(low, p) {
				skip = true
				break
			}
		}
		if skip || line == "}" {
			continue
		}
		at := strings.Index(line, "-->")
		if at < 0 {
			continue
		}
		left := strings.TrimSpace(line[:at])
		right := strings.TrimSpace(line[at+3:])
		label := ""
		if colon := strings.Index(right, ":"); colon >= 0 {
			label = strings.TrimSpace(right[colon+1:])
			right = strings.TrimSpace(right[:colon])
		}
		fromID, toID := left, right
		if fromID == "[*]" {
			fromID = "[*]start"
		}
		if toID == "[*]" {
			toID = "[*]end"
		}
		from := g.node(fromID)
		to := g.node(toID)
		if from == nil || to == nil {
			continue
		}
		if fromID == "[*]start" {
			from.label = "开始"
		}
		if toID == "[*]end" {
			to.label = "结束"
		}
		g.edges = append(g.edges, mermaidEdge{from: from, to: to, label: label})
	}
	if len(g.nodes) == 0 || len(g.edges) == 0 {
		return nil, false
	}
	g.levels()
	return g, true
}
