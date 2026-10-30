package theme

// Computed is the style of a node: every length in DIPs, every color with
// the node's opacity applied.
type Computed struct {
	Color      Color
	FontFamily []string
	FontSize   float32
	FontWeight int
	Italic     bool
	// FontFeatures are the OpenType features of the text, comma separated,
	// as MyGo's ui.Font takes them: "tnum" turns one on, "-liga" off.
	FontFeatures  string
	LetterSpacing float32
	// LineHeight is a multiple of FontSize; 0 is the font's own.
	LineHeight float32
	TextAlign  Align
	CaretColor Color
	ListStyle  ListStyle

	// Decoration is that of the node's text: its own, and that of the
	// elements around it.
	Decoration Decoration
	// TextBackground is the color behind the node's text, its corners
	// rounded by TextRadius: its own, or that of the inline element
	// around it.
	TextBackground Color
	TextRadius     float32
	// Shift raises the text above its baseline: vertical-align's, of the
	// node and of the inline elements around it.
	Shift float32
	// Opacity is the node's times that of the nodes around it, which the
	// colors here are multiplied by already.
	Opacity float32

	// Background fills the node's box, its corners rounded by Radius.
	Background Color
	Radius     float32
	Padding    Edges
	Margin     Edges
	// Border is the box's border of each side: top, right, bottom, left.
	Border [4]Border
	// MaxWidth is 0 for none, as Width and Height are for auto.
	MaxWidth float32
	Width    float32
	Height   float32

	raw  rawStyle
	vars varMap
	set  uint64
}

// rawStyle is what a node's children inherit as it is, before the node's
// opacity.
type rawStyle struct {
	color    Color
	caret    colorValue
	opacity  float32
	lhNormal bool
	// lhMul is the line height as a multiple, or lhPx a length.
	lhMul, lhPx float32
	features    string
	tabular     bool
}

// Edges are lengths of the four sides of a box.
type Edges struct{ Top, Right, Bottom, Left float32 }

// Border is the border of a side of a box: none when Width is 0.
type Border struct {
	Width float32
	Color Color
}

// Decoration is the lines of text.
type Decoration struct {
	Underline, LineThrough bool
	Wavy                   bool
	// Color is that of the lines, and Thickness theirs, 0 for the font's.
	Color     Color
	Thickness float32
}

// Specified reports whether a rule declared the property name of the
// node, rather than the node inheriting it or taking its initial value: a
// code token whose color no rule sets takes the code editor's color.
func (c *Computed) Specified(name string) bool {
	if id, ok := propIDs[name]; ok {
		return c.set&(1<<id) != 0
	}
	return false
}

// Var returns the value of the variable name, --x, as CSS writes it.
func (c *Computed) Var(name string) (string, bool) {
	ts, ok := c.vars[name]
	if !ok {
		return "", false
	}
	return tokensText(ts), true
}

// VarColor returns the value of the variable name as a color, with the
// node's opacity, as renderers take theirs: --diagram-line.
func (c *Computed) VarColor(name string) (Color, bool) {
	ts, ok := c.vars[name]
	if !ok {
		return Color{}, false
	}
	p := &vparser{ts: ts}
	v, ok := parseColor(p)
	if !ok || !p.done() {
		return Color{}, false
	}
	return v.resolve(c.raw.color).Alpha(c.Opacity), true
}

// varMap is the variables of a node, shared with its parent until it
// declares its own.
type varMap map[string][]token
