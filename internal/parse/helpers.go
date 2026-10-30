package parse

import "strings"

// lineStartAt returns the offset of the start of the line holding off.
func lineStartAt(src string, off int) int {
	off = min(max(off, 0), len(src))
	return strings.LastIndexByte(src[:off], '\n') + 1
}

// lineEndAt returns the offset of the end of the line holding off, before
// its newline.
func lineEndAt(src string, off int) int {
	off = min(max(off, 0), len(src))
	if i := strings.IndexByte(src[off:], '\n'); i >= 0 {
		return off + i
	}
	return len(src)
}

// isASCIIPunct reports whether c is ASCII punctuation.
func isASCIIPunct(c byte) bool {
	return c >= '!' && c <= '/' || c >= ':' && c <= '@' || c >= '[' && c <= '`' || c >= '{' && c <= '~'
}
