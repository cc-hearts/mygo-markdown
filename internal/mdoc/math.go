package mdoc

import "strings"

// texToUnicode turns the common TeX used in Markdown into readable Unicode.
// It is deliberately a small, self-contained approximation of KaTeX: Greek
// letters, operators, fractions, roots, scripts and text commands. It is not
// a layout engine, but raw dollar signs no longer leak into the document.
func texToUnicode(src string) string {
	s := strings.TrimSpace(src)
	s = strings.ReplaceAll(s, "\\\\", "\n")
	s = strings.ReplaceAll(s, "\\{", "\uE000")
	s = strings.ReplaceAll(s, "\\}", "\uE001")
	s = expandTextCommands(s)
	s = expandFrac(s)
	s = expandSqrt(s)
	s = expandScripts(s)
	s = replaceMathSymbols(s)
	s = strings.ReplaceAll(s, "&", " ")
	s = strings.ReplaceAll(s, "~", " ")
	s = strings.ReplaceAll(s, "{", "")
	s = strings.ReplaceAll(s, "}", "")
	s = strings.ReplaceAll(s, "\uE000", "{")
	s = strings.ReplaceAll(s, "\uE001", "}")
	s = stripUnknownCommands(s)
	// Collapse runs of spaces per line, keeping hard line breaks.
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.Join(strings.Fields(line), " ")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func expandTextCommands(s string) string {
	for _, cmd := range []string{
		"\\text", "\\textrm", "\\mathrm", "\\mathbf", "\\mathit", "\\operatorname",
		"\\overline", "\\underline", "\\vec", "\\hat", "\\bar", "\\displaystyle",
		"\\limits", "\\left", "\\right", "\\big", "\\Big", "\\bigg", "\\Bigg",
	} {
		s = strings.ReplaceAll(s, cmd, "")
	}
	return s
}

func expandFrac(s string) string {
	for {
		i, cmd := -1, ""
		for _, c := range []string{"\\frac", "\\dfrac", "\\tfrac"} {
			if j := strings.Index(s, c); j >= 0 && (i < 0 || j < i) {
				i, cmd = j, c
			}
		}
		if i < 0 {
			return s
		}
		k := i + len(cmd)
		for k < len(s) && (s[k] == ' ' || s[k] == '\t') {
			k++
		}
		if k >= len(s) || s[k] != '{' {
			s = s[:i] + s[k:]
			continue
		}
		num, afterNum, ok := readGroup(s, k)
		if !ok {
			return s
		}
		k = afterNum
		for k < len(s) && (s[k] == ' ' || s[k] == '\t') {
			k++
		}
		if k >= len(s) || s[k] != '{' {
			return s
		}
		den, afterDen, ok := readGroup(s, k)
		if !ok {
			return s
		}
		n, d := texToUnicode(num), texToUnicode(den)
		rep := n + "/" + d
		if needsParens(n) {
			rep = "(" + n + ")/" + d
		}
		if needsParens(d) {
			if strings.HasPrefix(rep, "(") {
				rep = "(" + n + ")/(" + d + ")"
			} else {
				rep = n + "/(" + d + ")"
			}
		}
		s = s[:i] + rep + s[afterDen:]
	}
}

func expandSqrt(s string) string {
	for {
		i := strings.Index(s, "\\sqrt")
		if i < 0 {
			return s
		}
		k := i + len("\\sqrt")
		index := ""
		if k < len(s) && s[k] == '[' {
			j := strings.IndexByte(s[k:], ']')
			if j < 0 {
				return s
			}
			index = texToUnicode(s[k+1 : k+j])
			k += j + 1
		}
		for k < len(s) && (s[k] == ' ' || s[k] == '\t') {
			k++
		}
		if k >= len(s) || s[k] != '{' {
			return s
		}
		body, after, ok := readGroup(s, k)
		if !ok {
			return s
		}
		rep := "√(" + texToUnicode(body) + ")"
		if index != "" {
			rep = index + rep
		}
		s = s[:i] + rep + s[after:]
	}
}

func expandScripts(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if (s[i] == '^' || s[i] == '_') && i+1 < len(s) {
			sup := s[i] == '^'
			j := i + 1
			arg := ""
			if s[j] == '{' {
				if body, after, ok := readGroup(s, j); ok {
					arg = body
					j = after
				}
			} else {
				arg = s[j : j+1]
				j++
			}
			mapped, ok := script(arg, sup)
			if ok {
				b.WriteString(mapped)
			} else {
				b.WriteByte(s[i])
				b.WriteString(texToUnicode(arg))
			}
			i = j
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func script(arg string, sup bool) (string, bool) {
	arg = texToUnicode(arg)
	var table map[rune]rune
	if sup {
		table = map[rune]rune{'0': '⁰', '1': '¹', '2': '²', '3': '³', '4': '⁴', '5': '⁵', '6': '⁶', '7': '⁷', '8': '⁸', '9': '⁹', '+': '⁺', '-': '⁻', '=': '⁼', '(': '⁽', ')': '⁾', 'n': 'ⁿ', 'i': 'ⁱ', 'x': 'ˣ', 'y': 'ʸ'}
	} else {
		table = map[rune]rune{'0': '₀', '1': '₁', '2': '₂', '3': '₃', '4': '₄', '5': '₅', '6': '₆', '7': '₇', '8': '₈', '9': '₉', '+': '₊', '-': '₋', '=': '₌', '(': '₍', ')': '₎', 'a': 'ₐ', 'e': 'ₑ', 'h': 'ₕ', 'i': 'ᵢ', 'j': 'ⱼ', 'k': 'ₖ', 'l': 'ₗ', 'm': 'ₘ', 'n': 'ₙ', 'o': 'ₒ', 'p': 'ₚ', 'r': 'ᵣ', 's': 'ₛ', 't': 'ₜ', 'u': 'ᵤ', 'v': 'ᵥ', 'x': 'ₓ'}
	}
	var b strings.Builder
	for _, r := range arg {
		m, ok := table[r]
		if !ok {
			return "", false
		}
		b.WriteRune(m)
	}
	return b.String(), true
}

func replaceMathSymbols(s string) string {
	pairs := []string{
		"\\mathbb{R}", "ℝ", "\\mathbb{N}", "ℕ", "\\mathbb{Z}", "ℤ", "\\mathbb{Q}", "ℚ", "\\mathbb{C}", "ℂ",
		"\\varepsilon", "ε", "\\vartheta", "ϑ", "\\varphi", "φ",
		"\\iint", "∬", "\\oint", "∮", "\\infty", "∞", "\\int", "∫",
		"\\alpha", "α", "\\beta", "β", "\\gamma", "γ", "\\delta", "δ",
		"\\epsilon", "ε", "\\zeta", "ζ", "\\eta", "η",
		"\\theta", "θ", "\\iota", "ι", "\\kappa", "κ",
		"\\lambda", "λ", "\\mu", "μ", "\\nu", "ν", "\\xi", "ξ", "\\pi", "π",
		"\\rho", "ρ", "\\sigma", "σ", "\\tau", "τ", "\\upsilon", "υ",
		"\\phi", "φ", "\\chi", "χ", "\\psi", "ψ", "\\omega", "ω",
		"\\Gamma", "Γ", "\\Delta", "Δ", "\\Theta", "Θ", "\\Lambda", "Λ",
		"\\Xi", "Ξ", "\\Pi", "Π", "\\Sigma", "Σ", "\\Phi", "Φ", "\\Psi", "Ψ", "\\Omega", "Ω",
		"\\sum", "∑", "\\prod", "∏",
		"\\pm", "±", "\\mp", "∓", "\\times", "×", "\\div", "÷",
		"\\cdot", "·", "\\leq", "≤", "\\le", "≤", "\\geq", "≥", "\\ge", "≥",
		"\\neq", "≠", "\\ne", "≠", "\\approx", "≈", "\\equiv", "≡",
		"\\to", "→", "\\rightarrow", "→", "\\leftarrow", "←", "\\Rightarrow", "⇒",
		"\\Leftrightarrow", "⇔", "\\in", "∈", "\\notin", "∉", "\\subset", "⊂",
		"\\cup", "∪", "\\cap", "∩", "\\forall", "∀", "\\exists", "∃",
		"\\nabla", "∇", "\\partial", "∂", "\\ldots", "…", "\\dots", "…",
		"\\quad", " ", "\\qquad", "  ", "\\,", " ", "\\;", " ", "\\:", " ", "\\!", "",
		"\\%", "%", "\\$", "$", "\\_", "_", "\\#", "#",
	}

	return strings.NewReplacer(pairs...).Replace(s)
}

func stripUnknownCommands(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '\\' {
			b.WriteByte(s[i])
			i++
			continue
		}
		j := i + 1
		for j < len(s) && (s[j] >= 'a' && s[j] <= 'z' || s[j] >= 'A' && s[j] <= 'Z') {
			j++
		}
		if j == i+1 {
			i++
			continue
		}
		b.WriteString(s[i+1 : j])
		i = j
	}
	return b.String()
}

func readGroup(s string, at int) (body string, after int, ok bool) {
	if at >= len(s) || s[at] != '{' {
		return "", at, false
	}
	depth := 0
	for i := at; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[at+1 : i], i + 1, true
			}
		}
	}
	return "", at, false
}

func needsParens(s string) bool {
	return strings.ContainsAny(s, " +-") || strings.Contains(s, "/")
}

// TeXToUnicode exposes the lightweight Unicode math fallback to the editor.
func TeXToUnicode(expr string) string { return texToUnicode(expr) }
