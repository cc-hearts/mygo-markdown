package livemark

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui/markdown/parser"
)

// Emphasis beside CJK punctuation. CommonMark decides whether a run of *,
// _ or ~ opens or closes emphasis by the characters around it, and reads
// "**注意：**内容" as literal asterisks: the closing run follows
// punctuation and comes before a letter, so it closes nothing. Typora,
// and the CJK-friendly amendment to CommonMark
// (github.com/tats-u/markdown-cjk-friendly), take it for strong emphasis.
//
// The parser follows CommonMark, so the editor gives it the source with
// a character beside such a run swapped for one of the same length that
// makes the parser read the run as the amendment does. No offset moves,
// and nothing shows the swap: the display is drawn from the source.

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
		r >= 0x2e80 && r <= 0x303e, // radicals, CJK symbols and punctuation
		r >= 0x3041 && r <= 0x33ff, // kana, bopomofo, enclosed letters, compatibility
		r >= 0x3400 && r <= 0x4dbf, // ideographs, extension A
		r >= 0x4e00 && r <= 0x9fff, // ideographs
		r >= 0xa000 && r <= 0xa4cf, // Yi
		r >= 0xa960 && r <= 0xa97f, // Hangul Jamo extended A
		r >= 0xac00 && r <= 0xd7a3, // Hangul syllables
		r >= 0xf900 && r <= 0xfaff, // compatibility ideographs
		r >= 0xfe10 && r <= 0xfe19, // vertical forms
		r >= 0xfe30 && r <= 0xfe6f, // compatibility and small forms
		r >= 0xff00 && r <= 0xffef, // halfwidth and fullwidth forms
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
// around it: the offsets of their first bytes, -1 for a line's edge.
type delimRun struct {
	start, end int
	c          byte
	before     int
	after      int
}

// cjkMask returns src with characters beside delimiter runs swapped, so
// that the parser reads emphasis and strikethrough beside CJK text as the
// CJK-friendly amendment does, and src itself when nothing needs it.
func cjkMask(src string) string {
	runs := delimRuns(src)
	if len(runs) == 0 {
		return src
	}
	m := &masker{src: src, runs: runs, swaps: map[int]rune{}, beside: map[int][]int{}}
	for k, r := range runs {
		if r.before >= 0 {
			m.beside[r.before] = append(m.beside[r.before], k)
		}
		if r.after >= 0 {
			m.beside[r.after] = append(m.beside[r.after], k)
		}
	}
	for k := range runs {
		if !m.matches(k) {
			m.fix(k)
		}
	}
	if len(m.swaps) == 0 {
		return src
	}
	b := []byte(src)
	for off, r := range m.swaps {
		utf8.EncodeRune(b[off:], r)
	}
	return string(b)
}

// masker finds the swaps of cjkMask.
type masker struct {
	src   string
	runs  []delimRun
	swaps map[int]rune
	// beside are the runs beside each character, by its offset.
	beside map[int][]int
}

// letters and puncts are the stand-ins of a character by its length in
// UTF-8: a letter, and a punctuation mark or a symbol.
var (
	letters = [5]rune{0, 'a', 'é', '一', '𠀀'}
	puncts  = [5]rune{0, '!', '¡', '、', '𝄀'}
)

// fix swaps a character beside run k for a stand-in, so that the parser
// reads the run as the amendment does; unless none does, or the swap
// changes how the parser reads a run on the other side of the character
// that it read right. The first that works of a letter for the character
// before the run, then after it, then punctuation for either.
func (m *masker) fix(k int) {
	r := m.runs[k]
	for _, c := range [...]struct {
		off int
		by  *[5]rune
	}{{r.before, &letters}, {r.after, &letters}, {r.before, &puncts}, {r.after, &puncts}} {
		// Not a line's edge, nor ASCII, which may be syntax, nor a
		// character swapped already.
		if c.off < 0 || m.src[c.off] < utf8.RuneSelf {
			continue
		}
		if _, ok := m.swaps[c.off]; ok {
			continue
		}
		ch, n := utf8.DecodeRuneInString(m.src[c.off:])
		if ch == utf8.RuneError || n >= len(c.by) || classOf(ch) == clsSpace {
			continue
		}
		was := map[int]bool{}
		for _, j := range m.beside[c.off] {
			if j != k {
				was[j] = m.matches(j)
			}
		}
		m.swaps[c.off] = c.by[n]
		ok := m.matches(k)
		for j, right := range was {
			if right && !m.matches(j) {
				ok = false
			}
		}
		if ok {
			return
		}
		delete(m.swaps, c.off)
	}
}

// matches reports whether the parser reads run k as the amendment does.
func (m *masker) matches(k int) bool {
	o1, c1 := m.can(k, false)
	o2, c2 := m.can(k, true)
	return o1 == o2 && c1 == c2
}

// can returns whether run k can open and close: as the parser reads it,
// with the swaps made so far, or with cjk set, as the amendment reads the
// source.
func (m *masker) can(k int, cjk bool) (open, close bool) {
	r := m.runs[k]
	br, ar := m.runeAt(r.before, !cjk), m.runeAt(r.after, !cjk)
	b, a := classOf(br), classOf(ar)
	if cjk {
		b = m.seqClass(r.before, b)
	}
	if r.before >= 0 && quotePrefix(m.src, r.before) {
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

// runeAt returns the character at off: as the parser reads it, with the
// swaps made so far, or as the source has it. A line's edge is a newline.
func (m *masker) runeAt(off int, swapped bool) rune {
	if off < 0 {
		return '\n'
	}
	if r, ok := m.swaps[off]; ok && swapped {
		return r
	}
	r, _ := utf8.DecodeRuneInString(m.src[off:])
	return r
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
	for i := 0; i < len(src); {
		c := src[i]
		if c != '*' && c != '_' && c != '~' {
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
			continue
		}
		r := delimRun{start: s, end: j, c: c, before: -1, after: -1}
		if s > 0 {
			if p, n := utf8.DecodeLastRuneInString(src[:s]); p != '\n' && p != '\r' {
				r.before = s - n
			}
		}
		if j < len(src) && src[j] != '\n' && src[j] != '\r' {
			r.after = j
		}
		wide = wide || j < len(src) && src[j] >= utf8.RuneSelf || s > 0 && src[s-1] >= utf8.RuneSelf
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

// quotePrefix reports whether the character at off ends a line's prefix
// of quotation markers, which the parser does not read as text before a
// run: "> **注意：**".
func quotePrefix(src string, off int) bool {
	if src[off] != '>' && src[off] != ' ' && src[off] != '\t' {
		return false
	}
	return strings.Trim(src[lineStartAt(src, off):off+1], " \t>") == ""
}

func isAlnumRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// plainText returns the text of inlines, as parser.PlainText does, with
// the characters cjkMask swapped taken back: a Text inline that is the
// parsed source as it is reads the editor's source instead.
func (e *Editor) plainText(ins []parser.Inline) string {
	parsed := e.doc.Source()
	var b strings.Builder
	for k := range ins {
		switch in := &ins[k]; in.Kind {
		case parser.Text:
			if in.End <= len(parsed) && in.End <= len(e.src) && in.Text == parsed[in.Start:in.End] {
				b.WriteString(e.src[in.Start:in.End])
			} else {
				b.WriteString(in.Text)
			}
		case parser.Code, parser.HTML, parser.Math:
			b.WriteString(in.Text)
		case parser.SoftBreak, parser.LineBreak:
			b.WriteByte(' ')
		case parser.FootnoteRef:
			b.WriteString("[^" + in.Text + "]")
		}
	}
	return b.String()
}
