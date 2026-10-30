package livemark

import "testing"

// TestImageLabel checks what an image not loaded shows: its alt text, the
// name of its file, or the placeholder for an image with neither.
func TestImageLabel(t *testing.T) {
	cases := []struct{ alt, src, want string }{
		{"a cat", "x.png", "🖼 a cat"},
		{"", "pics/x.png", "🖼 x.png"},
		{"", `C:\pics\x.png`, "🖼 x.png"},
		{"", "https://example.com/a/b.png?w=1#top", "🖼 b.png"},
		{"", "data:image/png;base64,AAAA", "🖼 图片"},
		{"", "", "🖼 图片"},
	}
	for _, c := range cases {
		if got := imageLabel(&object{alt: c.alt, src: c.src}); got != c.want {
			t.Errorf("imageLabel(alt=%q, src=%q) = %q, want %q", c.alt, c.src, got, c.want)
		}
	}
}
