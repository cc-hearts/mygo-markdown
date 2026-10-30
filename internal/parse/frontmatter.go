package parse

import "strings"

// frontMatterEnd returns where the YAML front matter at the top of src
// ends, after its closing line, or 0 when it has none: a first line of
// ---, up to a line of --- or ....
func frontMatterEnd(src string) int {
	if !strings.HasPrefix(src, "---\n") && !strings.HasPrefix(src, "---\r\n") {
		return 0
	}
	for off := strings.IndexByte(src, '\n') + 1; off < len(src); {
		end := lineEndAt(src, off)
		l := strings.TrimRight(src[off:end], " \t\r")
		if l == "---" || l == "..." {
			if off == 4 || off == 5 {
				// An empty front matter is two rules: leave them be.
				return 0
			}
			return end
		}
		off = end + 1
	}
	return 0
}

// maskFrontMatter returns src with its front matter blanked out, its
// newlines kept, so that the parser reads blank lines there and every
// offset after it stays where it was.
func maskFrontMatter(src string, end int) string {
	if end == 0 {
		return src
	}
	b := []byte(src)
	for i := 0; i < end; i++ {
		if b[i] != '\n' && b[i] != '\r' {
			b[i] = ' '
		}
	}
	return string(b)
}
