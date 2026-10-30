package parse

import (
	"unicode"
	"unicode/utf8"
)

// Emphasis beside CJK punctuation. CommonMark decides whether a run of *,
// _ or ~ opens or closes emphasis by the characters around it, and reads
// "**注意：**内容" as literal asterisks: the closing run follows
// punctuation and comes before a letter, so it closes nothing. Typora,
// and the CJK-friendly amendment to CommonMark
// (github.com/tats-u/markdown-cjk-friendly), take it for strong emphasis.
//
// Parsers follow CommonMark, so CJKFriendly gives them the source with a
// character beside such a run swapped for one of the same length that
// makes them read the run as the amendment does. No offset moves, so the
// text of what they parse is read from the source as it was.

// cjkClass is what a character beside a delimiter run is, for flanking.
type cjkClass uint8

const (
	// clsSpace is whitespace, or the start or the end of a line.
	clsSpace cjkClass = iota
	// clsPunct is punctuation or a symbol that is not CJK.
	clsPunct
	// clsCJKPunct is CJK punctuation, as ：and （.
	clsCJKPunct
	// clsCJK is a CJK character that is not punctuation.
	clsCJK
	clsOther
)

func classOf(r rune) cjkClass {
	switch {
	case r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' || unicode.Is(unicode.Zs, r):
		return clsSpace
	case isPunctRune(r):
		if isCJK(r) {
			return clsCJKPunct
		}
		return clsPunct
	case isCJK(r):
		return clsCJK
	}
	return clsOther
}

// isPunctRune reports whether r is punctuation or a symbol, as CommonMark
// counts them.
func isPunctRune(r rune) bool {
	if r < utf8.RuneSelf {
		return isASCIIPunct(byte(r))
	}
	return unicode.IsPunct(r) || unicode.IsSymbol(r)
}

// isCJK reports whether r is a CJK character: East Asian wide, fullwidth
// or halfwidth, and not shown as an emoji, or Hangul.
func isCJK(r rune) bool {
	switch {
	case r < 0x1100:
		return false
	case r <= 0x115f, // Hangul Jamo
		r >= 0x2e80 && r <= 0x303e,   // radicals, CJK symbols and punctuation
		r >= 0x3041 && r <= 0x33ff,   // kana, bopomofo, enclosed letters, compatibility
		r >= 0x3400 && r <= 0x4dbf,   // ideographs, extension A
		r >= 0x4e00 && r <= 0x9fff,   // ideographs
		r >= 0xa000 && r <= 0xa4cf,   // Yi
		r >= 0xa960 && r <= 0xa97f,   // Hangul Jamo extended A
		r >= 0xac00 && r <= 0xd7a3,   // Hangul syllables
		r >= 0xf900 && r <= 0xfaff,   // compatibility ideographs
		r >= 0xfe10 && r <= 0xfe19,   // vertical forms
		r >= 0xfe30 && r <= 0xfe6f,   // compatibility and small forms
		r >= 0xff00 && r <= 0xffef,   // halfwidth and fullwidth forms
		r >= 0x16fe0 && r <= 0x18cff, // Tangut, Khitan
		r >= 0x1b000 && r <= 0x1b2ff, // kana supplement and extensions
		r >= 0x20000 && r <= 0x3fffd: // ideographs, extensions B and later
		return true
	case r >= 0x1f200 && r <= 0x1f2ff:
		// Enclosed ideographs, but for those shown as emoji.
		switch {
		case r == 0x1f201, r == 0x1f21a, r == 0x1f22f, r >= 0x1f232 && r <= 0x1f236,
			r >= 0x1f238 && r <= 0x1f23a, r >= 0x1f250 && r <= 0x1f251:
			return false
		}
		return true
	}
	return unicode.Is(unicode.Hangul, r)
}

// flanks reports whether a delimiter run between characters of classes b
// and a is left- and right-flanking: by CommonMark's rules, or with cjk
// set, by the CJK-friendly amendment's.
func flanks(b, a cjkClass, cjk bool) (left, right bool) {
	if !cjk {
		bp := b == clsPunct || b == clsCJKPunct
		ap := a == clsPunct || a == clsCJKPunct
		left = a != clsSpace && (!ap || b == clsSpace || bp)
		right = b != clsSpace && (!bp || a == clsSpace || ap)
		return left, right
	}
	// Only punctuation that is not CJK keeps a run from flanking, and CJK
	// characters count as punctuation on the other side of it.
	left = a != clsSpace && (a != clsPunct || b != clsOther)
	right = b != clsSpace && (b != clsPunct || a != clsOther)
	return left, right
}

// delimRun is a run of delimiters in the source, and the characters
// around it.
type delimRun struct {
	start, end int
	c          byte
	// before and after are the offsets of the characters before and after
	// the run, -1 at a line's edge; br and ar the characters, a newline at
	// a line's edge, and sb and sa their stand-ins, or 0.
	before, after int
	br, ar        rune
	sb, sa        rune
	// lead tells a run after a line's prefix of quotation markers, which
	// the parser does not read as text before it: "> **注意：**".
	lead bool
	// open and close are whether the run opens and closes emphasis, as the
	// amendment reads it.
	open, close bool
}

// CJKFriendly returns src with characters beside delimiter runs swapped,
// so that a CommonMark parser reads emphasis and strikethrough beside CJK
// text as the CJK-friendly amendment does; src itself when nothing needs
// it. The result is as long as src, every character where it was: what
// the parser makes of it is read from src. What it swaps on a line
// depends on that line alone.
func CJKFriendly(src string) string {
	runs := delimRuns(src)
	if len(runs) == 0 {
		return src
	}
	m := masker{src: src, runs: runs}
	for k := range runs {
		runs[k].open, runs[k].close = m.can(k, true)
	}
	swapped := false
	for k := range runs {
		if !m.matches(k) && m.fix(k) {
			swapped = true
		}
	}
	if !swapped {
		return src
	}
	b := []byte(src)
	for _, r := range runs {
		if r.sb != 0 {
			utf8.EncodeRune(b[r.before:], r.sb)
		}
		if r.sa != 0 {
			utf8.EncodeRune(b[r.after:], r.sa)
		}
	}
	return string(b)
}

// masker finds the swaps of CJKFriendly.
type masker struct {
	src  string
	runs []delimRun
}

// letters and puncts are the stand-ins of a character by its length in
// UTF-8: a letter, and a punctuation mark or a symbol.
var (
	letters = [5]rune{0, 'a', 'é', '一', '𠀀'}
	puncts  = [5]rune{0, '!', '¡', '、', '𝄀'}
)

// fix swaps a character beside run k for a stand-in, so that the parser
// reads the run as the amendment does, and reports whether it did; it
// does not when no stand-in does, nor when the swap changes how the parser
// reads the run on the other side of the character, which it read right.
// The first that works of a letter for the character before the run,
// then after it, then punctuation for either.
func (m *masker) fix(k int) bool {
	r := &m.runs[k]
	for _, c := range [...]struct {
		off      int
		ch, done rune
		by       *[5]rune
	}{{r.before, r.br, r.sb, &letters}, {r.after, r.ar, r.sa, &letters}, {r.before, r.br, r.sb, &puncts}, {r.after, r.ar, r.sa, &puncts}} {
		// Not a line's edge, nor ASCII, which may be syntax, nor a
		// character swapped already.
		if c.off < 0 || c.ch < utf8.RuneSelf || c.done != 0 || c.ch == utf8.RuneError || classOf(c.ch) == clsSpace {
			continue
		}
		n := utf8.RuneLen(c.ch)
		if n >= len(c.by) {
			continue
		}
		j := m.beside(k, c.off)
		was := j >= 0 && m.matches(j)
		m.swap(k, j, c.off, c.by[n])
		if m.matches(k) && !(was && !m.matches(j)) {
			return true
		}
		m.swap(k, j, c.off, 0)
	}
	return false
}

// beside returns the run other than k beside the character at off, before
// or after k: the run before it or after it, or -1.
func (m *masker) beside(k, off int) int {
	if off == m.runs[k].before && k > 0 && m.runs[k-1].after == off {
		return k - 1
	}
	if off == m.runs[k].after && k+1 < len(m.runs) && m.runs[k+1].before == off {
		return k + 1
	}
	return -1
}

// swap sets the stand-in of the character at off beside runs k and j (-1
// for none), or with to 0 takes it back.
func (m *masker) swap(k, j, off int, to rune) {
	for _, i := range [2]int{k, j} {
		if i < 0 {
			continue
		}
		r := &m.runs[i]
		if r.before == off {
			r.sb = to
		}
		if r.after == off {
			r.sa = to
		}
	}
}

// matches reports whether the parser reads run k as the amendment does.
func (m *masker) matches(k int) bool {
	o, c := m.can(k, false)
	return o == m.runs[k].open && c == m.runs[k].close
}

// can returns whether run k can open and close: as the parser reads it,
// with the swaps made so far, or with cjk set, as the amendment reads the
// source.
func (m *masker) can(k int, cjk bool) (open, close bool) {
	r := &m.runs[k]
	br, ar := r.br, r.ar
	if !cjk && r.sb != 0 {
		br = r.sb
	}
	if !cjk && r.sa != 0 {
		ar = r.sa
	}
	b, a := classOf(br), classOf(ar)
	if cjk {
		b = m.seqClass(r.before, b)
	}
	if r.lead {
		b = clsSpace
	}
	left, right := flanks(b, a, cjk)
	switch r.c {
	case '_':
		bp := b == clsPunct || b == clsCJKPunct
		ap := a == clsPunct || a == clsCJKPunct
		return left && (!right || bp), right && (!left || ap)
	case '~':
		// The parser's: a tilde between letters or digits is not
		// strikethrough, as 20~25.
		if r.end-r.start == 1 && isAlnumRune(br) && isAlnumRune(ar) {
			return false, false
		}
	}
	return left, right
}

// seqClass returns the class, b, of the character at off before a run as
// the amendment reads it: a variation selector goes with the character
// before it, and an ideographic one is a CJK character.
func (m *masker) seqClass(off int, b cjkClass) cjkClass {
	if off < 0 {
		return b
	}
	r, _ := utf8.DecodeRuneInString(m.src[off:])
	switch {
	case r >= 0xe0100 && r <= 0xe01ef:
		return clsCJK
	case r >= 0xfe00 && r <= 0xfe0e && off > 0:
		p, _ := utf8.DecodeLastRuneInString(m.src[:off])
		if r == 0xfe01 && (p == '‘' || p == '’' || p == '“' || p == '”') {
			// A quotation mark in its fullwidth form is CJK.
			return clsCJKPunct
		}
		if c := classOf(p); c != clsSpace && c != clsOther {
			return c
		}
	}
	return b
}

// delimRuns returns the runs of *, _ and ~ in src that may be emphasis or
// strikethrough, when one has a character that is not ASCII beside it;
// nil otherwise, when the parser reads them all as the amendment does.
func delimRuns(src string) []delimRun {
	var runs []delimRun
	wide := false
	// lead tells the line so far is a prefix of quotation markers.
	lead := true
	for i := 0; i < len(src); {
		c := src[i]
		if c != '*' && c != '_' && c != '~' {
			if c == '\n' {
				lead = true
			} else if c != ' ' && c != '\t' && c != '>' {
				lead = false
			}
			i++
			continue
		}
		j := i
		for j < len(src) && src[j] == c {
			j++
		}
		s := i
		if escapedAt(src, i) {
			s++
		}
		i = j
		if s == j || c == '~' && j-s > 2 {
			lead = false
			continue
		}
		r := delimRun{start: s, end: j, c: c, before: -1, after: -1, br: '\n', ar: '\n'}
		if s > 0 {
			if p, n := utf8.DecodeLastRuneInString(src[:s]); p != '\n' && p != '\r' {
				r.before, r.br = s-n, p
				r.lead = lead
			}
		}
		if j < len(src) && src[j] != '\n' && src[j] != '\r' {
			r.after = j
			r.ar, _ = utf8.DecodeRuneInString(src[j:])
		}
		lead = false
		wide = wide || r.br >= utf8.RuneSelf || r.ar >= utf8.RuneSelf
		runs = append(runs, r)
	}
	if !wide {
		return nil
	}
	return runs
}

// escapedAt reports whether the character at i is escaped by a backslash.
func escapedAt(src string, i int) bool {
	n := 0
	for i-n-1 >= 0 && src[i-n-1] == '\\' {
		n++
	}
	return n%2 == 1
}

func isAlnumRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }
