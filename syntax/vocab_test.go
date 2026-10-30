package syntax

import (
	"strings"
	"testing"

	"github.com/CarlOpenLab/livemark/theme"
)

// TestVocabulary checks that every element a tree gives matches theme's
// vocabulary: a sheet selecting each element parses without a diagnostic.
func TestVocabulary(t *testing.T) {
	srcs := []string{
		sample,
		"[^n]\n\n[^n]: note\n",
		"| a | b |\n|:-:|--:|\n| 1 | 2 |\n",
		"<u>u</u> <mark>m</mark> <sup>s</sup> <sub>b</sub> <kbd>k</kbd> <b>b</b>\n",
		"www.example.com <https://a.b> ![x](i)\n",
	}
	var sb strings.Builder
	for _, src := range srcs {
		for n := range Parse(src).Walk() {
			sb.WriteString(n.Element().String())
			sb.WriteString(" { color: red }\n")
		}
	}
	if _, diags := theme.Parse("vocab.css", sb.String()); len(diags) > 0 {
		for _, d := range diags {
			t.Errorf("%v", d)
		}
	}
}
