package mdoc

import "strings"

// go-latex is intentionally small. Normalise the common wrappers and aliases
// KaTeX accepts into forms it can parse; ^/_ are handled separately by the
// script layout.
var texMacroAliases = []struct{ from, to string }{
	{`\le`, `\leq`},
	{`\ge`, `\geq`},
	{`\ne`, `\neq`},
	{`\to`, `\rightarrow`},
	{`\gets`, `\leftarrow`},
	{`\iff`, `\Leftrightarrow`},
	{`\implies`, `\Rightarrow`},
	{`\dfrac`, `\frac`},
	{`\tfrac`, `\frac`},
}

func normalizeTeX(expr string) string {
	for _, name := range []string{
		"text", "mathrm", "mathbf", "mathit", "mathsf", "mathtt",
		"mathcal", "mathbb", "operatorname",
	} {
		expr = unwrapTexCommand(expr, name)
	}
	for _, name := range []string{
		"left", "right", "displaystyle", "textstyle", "scriptstyle", "limits", "nolimits",
	} {
		expr = removeTexCommand(expr, name)
	}
	for _, a := range texMacroAliases {
		expr = replaceTexMacro(expr, a.from, a.to)
	}
	return expr
}

func isTexNameByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

func replaceTexMacro(s, from, to string) string {
	var b strings.Builder
	for {
		i := strings.Index(s, from)
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		end := i + len(from)
		if end < len(s) && isTexNameByte(s[end]) {
			b.WriteString(s[:i+1])
			s = s[i+1:]
			continue
		}
		b.WriteString(s[:i])
		b.WriteString(to)
		s = s[end:]
	}
}

func removeTexCommand(s, name string) string {
	macro := `\` + name
	var b strings.Builder
	for {
		i := strings.Index(s, macro)
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		end := i + len(macro)
		if end < len(s) && isTexNameByte(s[end]) {
			b.WriteString(s[:i+1])
			s = s[i+1:]
			continue
		}
		b.WriteString(s[:i])
		s = s[end:]
	}
}

func unwrapTexCommand(s, name string) string {
	macro := `\` + name + `{`
	for {
		i := strings.Index(s, macro)
		if i < 0 {
			return s
		}
		depth := 0
		end := -1
		for j := i + len(macro) - 1; j < len(s); j++ {
			switch s[j] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					end = j
				}
			}
			if end >= 0 {
				break
			}
		}
		if end < 0 {
			return s
		}
		s = s[:i] + s[i+len(macro):end] + s[end+1:]
	}
}
