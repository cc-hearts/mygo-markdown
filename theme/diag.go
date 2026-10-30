package theme

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

// Severity is how bad a diagnostic is.
type Severity uint8

const (
	// Error is CSS that cannot be read: the parser skipped it, as a
	// browser does.
	Error Severity = iota
	// Warning is CSS that reads but does nothing here: an unknown
	// property, a selector of no node, a value outside the subset.
	Warning
)

func (s Severity) String() string {
	if s == Error {
		return "error"
	}
	return "warning"
}

// Diagnostic is a problem in a sheet, at a line and a column, both from 1;
// columns count characters.
type Diagnostic struct {
	Severity  Severity
	Line, Col int
	Message   string
}

func (d Diagnostic) String() string {
	return fmt.Sprintf("%d:%d: %s: %s", d.Line, d.Col, d.Severity, d.Message)
}

// diagnostics collects the diagnostics of a source.
type diagnostics struct {
	src  string
	list []Diagnostic
	// lines are the offsets the source's lines start at, found at the
	// first diagnostic.
	lines []int
}

func (d *diagnostics) add(s Severity, pos int, msg string) {
	if d.lines == nil {
		d.lines = []int{0}
		for i := range len(d.src) {
			if d.src[i] == '\n' {
				d.lines = append(d.lines, i+1)
			}
		}
	}
	pos = min(max(pos, 0), len(d.src))
	line, _ := slices.BinarySearch(d.lines, pos+1)
	start := d.lines[line-1]
	col := 1 + utf8.RuneCountInString(d.src[start:pos])
	d.list = append(d.list, Diagnostic{Severity: s, Line: line, Col: col, Message: msg})
}

func (d *diagnostics) addf(s Severity, pos int, format string, args ...any) {
	d.add(s, pos, fmt.Sprintf(format, args...))
}

// suggest returns the candidate a word is likely a misspelling of, or "".
func suggest(word string, candidates []string) string {
	word = strings.ToLower(word)
	best, bestD := "", max(len(word)/3, 1)+1
	for _, c := range candidates {
		if d := distance(word, c); d < bestD {
			best, bestD = c, d
		}
	}
	return best
}

// didYouMean returns " (did you mean "x"?)" for the candidate a word is
// likely a misspelling of, or "".
func didYouMean(word string, candidates []string) string {
	if s := suggest(word, candidates); s != "" {
		return fmt.Sprintf(" (did you mean %q?)", s)
	}
	return ""
}

// distance returns how many edits make a into b: insertions, deletions,
// substitutions and transpositions of neighbors.
func distance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev2 := make([]int, len(rb)+1)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				cur[j] = min(cur[j], prev2[j-2]+1)
			}
		}
		prev2, prev, cur = prev, cur, prev2
	}
	return prev[len(rb)]
}
