package livemark

import (
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestLargeDocumentFrames measures the frames of a large document: the
// first, which lays everything out, and those of typing in it.
func TestLargeDocumentFrames(t *testing.T) {
	path := os.Getenv("MDEDIT_LARGE")
	if path == "" {
		t.Skip("MDEDIT_LARGE names a large Markdown file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, "")
	start := time.Now()
	h.e.SetSource(string(data))
	h.frame()
	t.Logf("%d bytes, %d leaves: first frame %v", len(data), len(h.e.leaves), time.Since(start))
	h.caretAt(len(data) / 2)
	start = time.Now()
	for i := 0; i < 20; i++ {
		h.tt.Type("x")
		h.frame()
	}
	t.Logf("typing: %v a key", time.Since(start)/20)
	start = time.Now()
	for i := 0; i < 20; i++ {
		h.key(0, ui.KeyDown)
	}
	t.Logf("moving down: %v a key", time.Since(start)/20)
	start = time.Now()
	h.tt.SetSize(640, 700)
	h.frame()
	h.frame()
	t.Logf("relayout at a new width: %v", time.Since(start))
}
