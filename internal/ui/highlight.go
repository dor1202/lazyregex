package ui

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"

	"github.com/dor1202/lazyregex/internal/engine"
	"github.com/dor1202/lazyregex/internal/explain"
)

// paint is the look of a run of cells.
type paint struct {
	fg, bg    string // theme tokens; "" = none
	bold      bool
	underline bool // the selected match in [6]
	curly     bool // a compile error's position in [5]
	reverse   bool // the cursor
}

func (t *Theme) paintStyle(p paint) lipgloss.Style {
	st := lipgloss.NewStyle()
	if p.fg != "" {
		st = st.Foreground(t.color(p.fg))
	}
	if p.bg != "" {
		st = st.Background(t.color(p.bg))
	}
	if p.bold {
		st = st.Bold(true)
	}
	if p.underline {
		st = st.Underline(true)
	}
	if p.curly {
		st = st.UnderlineStyle(lipgloss.UnderlineCurly).UnderlineColor(t.color("error"))
	}
	if p.reverse {
		st = st.Reverse(true)
	}
	return st
}

// displayRune is how a rune is drawn: control characters (tab included)
// become their one-cell Unicode control pictures so widths stay exact.
func displayRune(r rune) rune {
	switch {
	case r < 0x20:
		return 0x2400 + r
	case r == 0x7f:
		return '␡'
	}
	return r
}

// renderRuns renders s[from:to], asking at for the paint of each rune (by
// byte offset) and merging equal neighbours into one styled run. cursor is a
// byte offset drawn reversed (-1 = none); a cursor at to draws a reversed
// space.
func (t *Theme) renderRuns(s string, from, to int, at func(i int) paint, cursor int) string {
	var out, run strings.Builder
	var cur paint
	flush := func() {
		if run.Len() > 0 {
			out.WriteString(t.paintStyle(cur).Render(run.String()))
			run.Reset()
		}
	}
	for i := from; i < to; {
		r, n := utf8.DecodeRuneInString(s[i:])
		p := at(i)
		if i == cursor {
			p.reverse = true
		}
		if p != cur {
			flush()
			cur = p
		}
		run.WriteRune(displayRune(r))
		i += n
	}
	flush()
	if cursor == to {
		out.WriteString(t.paintStyle(paint{reverse: true}).Render(" "))
	}
	return out.String()
}

// groupToken is the colour token of capture group i (1-based), cycling
// through six colours.
func groupToken(i int) string { return fmt.Sprintf("group%d", (max(i, 1)-1)%6+1) }

// byteMatches converts a result's spans (in res.Unit) into byte offsets of
// text, in one pass.
func byteMatches(text string, res engine.Result) []engine.Match {
	var offs []int
	for _, m := range res.Matches {
		offs = append(offs, m.Span.Start, m.Span.End)
		for _, g := range m.Groups {
			offs = append(offs, g.Span.Start, g.Span.End)
		}
	}
	if len(offs) == 0 {
		return nil
	}
	b := engine.ToByteOffsets(text, res.Unit, offs)
	out := make([]engine.Match, len(res.Matches))
	k := 0
	next := func() engine.Span { s := engine.Span{Start: b[k], End: b[k+1]}; k += 2; return s }
	for i, m := range res.Matches {
		out[i].Span = next()
		out[i].Groups = make([]engine.Group, len(m.Groups))
		for j, g := range m.Groups {
			g.Span = next()
			out[i].Groups[j] = g
		}
	}
	return out
}

// textPaint is the paint of byte i inside match m: the match background, or
// the colours of the innermost matched group covering i.
func textPaint(m engine.Match, selected bool, i int) paint {
	p := paint{fg: "text", bg: "match", underline: selected}
	best, bestLen := -1, 0
	for gi, g := range m.Groups {
		if g.Matched && g.Span.Start <= i && i < g.Span.End {
			if l := g.Span.End - g.Span.Start; best < 0 || l <= bestLen {
				best, bestLen = gi, l
			}
		}
	}
	if best >= 0 {
		tok := groupToken(m.Groups[best].Index)
		p.fg, p.bg = tok+".fg", tok+".bg"
	}
	return p
}

// paintText renders text[from:to] (normally one line) with the matches ms
// (byte spans, sorted and non-overlapping) highlighted; match sel is
// underlined and cursor is drawn reversed.
func (t *Theme) paintText(text string, from, to int, ms []engine.Match, sel, cursor int) string {
	k0 := sort.Search(len(ms), func(k int) bool { return ms[k].Span.End > from })
	return t.renderRuns(text, from, to, func(i int) paint {
		for k := k0; k < len(ms) && ms[k].Span.Start <= i; k++ {
			if i < ms[k].Span.End {
				return textPaint(ms[k], k == sel, i)
			}
		}
		return paint{fg: "text"}
	}, cursor)
}

// highlightText renders text with res's matches highlighted: the match
// background, each matched group in its own colour (group i uses token
// group((i-1)%6+1)), and match sel underlined. Spans are converted from the
// result's unit to bytes first. Lines are separated by "\n".
func (t *Theme) highlightText(text string, res engine.Result, sel int) string {
	ms := byteMatches(text, res)
	var lines []string
	for start := 0; ; {
		end := lineEnd(text, start)
		lines = append(lines, t.paintText(text, start, end, ms, sel, -1))
		if end == len(text) {
			break
		}
		start = end + 1
	}
	return strings.Join(lines, "\n")
}

// highlightPattern renders pattern with syntax colours for flavor f (anchors,
// group parentheses and names, escapes, quantifiers and alternation), and
// the byte range underline (a compile error's position) with a red curly
// underline. underline[0] < 0 means none; a range starting at len(pattern)
// underlines a trailing space.
func (t *Theme) highlightPattern(pattern string, f engine.Flavor, flags string, underline [2]int) string {
	return t.paintPattern(pattern, f, flags, underline, -1)
}

func (t *Theme) paintPattern(pattern string, f engine.Flavor, flags string, underline [2]int, cursor int) string {
	toks := patternTokens(pattern, f, flags)
	out := t.renderRuns(pattern, 0, len(pattern), func(i int) paint {
		if underline[0] >= 0 && underline[0] <= i && i < underline[1] {
			return paint{fg: "error", curly: true}
		}
		if toks[i] == "" {
			return paint{fg: "text"}
		}
		return paint{fg: toks[i]}
	}, cursor)
	if underline[0] >= 0 && underline[0] >= len(pattern) && cursor != len(pattern) {
		out += t.paintStyle(paint{fg: "error", curly: true}).Render(" ")
	}
	return out
}

// patternTokens assigns a colour token to every byte of pattern ("" = plain
// text) from the explain parser's syntax tree.
func patternTokens(pattern string, f engine.Flavor, flags string) []string {
	toks := make([]string, len(pattern))
	set := func(s, e int, tok string) {
		for i := max(s, 0); i < min(e, len(toks)); i++ {
			toks[i] = tok
		}
	}
	var walk func(n *explain.Node)
	walk = func(n *explain.Node) {
		s, e := n.Span[0], n.Span[1]
		switch k := n.Kind; {
		case k == explain.Anchor:
			set(s, e, "anchor")
		case k.IsGroup() || k == explain.Conditional:
			tok := "paren"
			if k == explain.Capture {
				tok = groupToken(n.Index)
			}
			open := e
			if len(n.Children) > 0 {
				open = n.Children[0].Span[0]
			} else if strings.HasSuffix(n.Raw, ")") {
				open = e - 1
			}
			set(s, open, tok)
			if strings.HasSuffix(n.Raw, ")") && e-1 >= open {
				set(e-1, e, tok)
			}
			if k == explain.Named && n.Name != "" {
				if j := strings.Index(pattern[s:open], n.Name); j >= 0 {
					set(s+j, s+j+len(n.Name), groupToken(n.Index))
				}
			}
		case k == explain.Shorthand, k == explain.Backref, k == explain.NamedBackref,
			k == explain.UnicodeProp, k == explain.KeepOut, k == explain.Dot, k == explain.Recursion:
			set(s, e, "escape")
		case k == explain.Literal && strings.HasPrefix(n.Raw, `\`):
			set(s, e, "escape")
		case k == explain.CharClass:
			if len(n.Raw) > 0 {
				open := s + 1
				if strings.HasPrefix(n.Raw, "[^") {
					open++
				}
				set(s, open, "escape")
				if strings.HasSuffix(n.Raw, "]") {
					set(e-1, e, "escape")
				}
			}
		case k == explain.Quantifier:
			body := s
			for _, c := range n.Children {
				body = max(body, c.Span[1])
			}
			set(body, e, "quant")
		case k == explain.Alternation:
			prev := s
			for _, c := range n.Children {
				markBars(pattern, toks, prev, c.Span[0])
				prev = c.Span[1]
			}
			markBars(pattern, toks, prev, e)
		case k == explain.Comment:
			set(s, e, "dim")
		case k == explain.Unknown:
			set(s, e, "error")
		case k == explain.Verb:
			set(s, e, "paren")
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(explain.Parse(pattern, explain.DialectFor(f, flags)))
	return toks
}

// markBars colours the "|" separators between alternation branches.
func markBars(pattern string, toks []string, from, to int) {
	for i := max(from, 0); i < min(to, len(pattern)); i++ {
		if pattern[i] == '|' {
			toks[i] = "quant"
		}
	}
}

// patternDelims are the dim decorations around the pattern, as the flavor's
// source code would write it.
func patternDelims(f engine.Flavor, flags string) (pre, post string) {
	switch f {
	case engine.Python:
		return `r"`, `"`
	case engine.JS:
		return "/", "/" + flags
	case engine.PCRE:
		return "/", "/" + strings.ReplaceAll(flags, "g", "")
	case engine.Go:
		return "`", "`"
	}
	return "", ""
}

// errorUnit is the unit each flavor reports CompileError.Offset in: Python
// counts code points, JavaScript UTF-16 units, PCRE and Go bytes.
func errorUnit(f engine.Flavor) engine.Unit {
	switch f {
	case engine.Python:
		return engine.UnitCodePoints
	case engine.JS:
		return engine.UnitUTF16
	}
	return engine.UnitBytes
}

// errorUnderline converts a compile error's offset into the byte range of
// the offending rune in pattern ({-1,-1} when there is no offset).
func errorUnderline(pattern string, f engine.Flavor, offset int) [2]int {
	if offset < 0 {
		return [2]int{-1, -1}
	}
	b := engine.ToByteOffsets(pattern, errorUnit(f), []int{offset})[0]
	return [2]int{b, max(nextRune(pattern, b), b+1)}
}
