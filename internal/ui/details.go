package ui

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dor1202/lazyregex/internal/codegen"
	"github.com/dor1202/lazyregex/internal/engine"
	"github.com/dor1202/lazyregex/internal/explain"
	"github.com/dor1202/lazyregex/internal/hints"
	"github.com/dor1202/lazyregex/internal/runner"
	"github.com/dor1202/lazyregex/internal/syntax"
)

// Details tabs, in State.Tab[PDetails] order.
const (
	tabExplanation = iota
	tabCode
	tabCompare
)

// Code tab limits (R17): a test text longer than snippetTextLimit is
// replaced in the snippet by snippetPlaceholder, so the program stays short
// enough to paste.
const (
	snippetTextLimit   = 4 << 10
	snippetPlaceholder = "<your text here>"
)

// compareValues is how many matched values a Compare row shows.
const compareValues = 3

// detailsPanel is [7]: Explanation (the pattern as a tree of plain-English
// rows; the selected row's span is underlined in [5] while [7] has focus),
// Code (a runnable program for the current flavor) and Compare (the
// pattern in every flavor, with a hint and rewrite for the current
// flavor's failure).
type detailsPanel struct {
	basePanel
	top     [3]int // scroll offset of each tab
	cmpSel  int    // selected Compare row (index into engine.Flavors)
	started bool   // cmpSel has been put on the current flavor
	code    codeHighlight
}

func newDetailsPanel() *detailsPanel { return &detailsPanel{} }

func (*detailsPanel) Tabs() []string { return []string{"Explanation", "Code", "Compare"} }

// compareVisible reports whether the Compare tab is showing.
func compareVisible(s *State) bool { return s.Tab[PDetails] == tabCompare }

// --- Explanation ---------------------------------------------------------

// explainCache holds the explanation of one (pattern, flavor, flags).
type explainCache struct {
	pattern string
	flavor  engine.Flavor
	flags   string
	rows    []explain.Row
	lines   []string // explain.Render, one line per row
}

// explanation returns the current pattern's explanation rows and their
// rendered lines (cached).
func explanation(s *State) ([]explain.Row, []string) {
	flags, _ := flagsOf(s, s.Flavor)
	c := s.explained
	if c == nil || c.pattern != s.Pattern || c.flavor != s.Flavor || c.flags != flags {
		c = &explainCache{pattern: s.Pattern, flavor: s.Flavor, flags: flags}
		if s.Pattern != "" {
			c.rows = explain.Explain(s.Pattern, s.Flavor, flags)
			c.lines = strings.Split(strings.TrimSuffix(explain.Render(c.rows), "\n"), "\n")
		}
		s.explained = c
	}
	return c.rows, c.lines
}

// explainSpan is the pattern byte range to underline in [5]: the selected
// explanation row's span while [7] has focus on the Explanation tab.
func explainSpan(s *State) [2]int {
	if s.Focus != PDetails || s.Tab[PDetails] != tabExplanation {
		return noSpan
	}
	rows, _ := explanation(s)
	if len(rows) == 0 {
		return noSpan
	}
	return rows[clamp(s.ExplainSel, 0, len(rows)-1)].Span
}

func (p *detailsPanel) viewExplanation(s *State, w, h int) string {
	t := s.Theme
	rows, lines := explanation(s)
	if len(rows) == 0 {
		return t.fg("dim").Render("type a pattern to see it explained")
	}
	s.ExplainSel = clamp(s.ExplainSel, 0, len(rows)-1)
	top := scrollTo(&p.top[tabExplanation], s.ExplainSel, h, len(rows))
	var out []string
	for i := top; i < min(len(rows), top+h); i++ {
		r := rows[i]
		line := sanitize(lines[i])
		if i == s.ExplainSel {
			out = append(out, t.selectedRow().Render(fitWidth(line, w)))
			continue
		}
		if r.Unknown {
			out = append(out, t.fg("dim").Render(line))
			continue
		}
		// guides (3 runes per level), then the token, then the description.
		left := strings.TrimSuffix(line, sanitize(r.Text))
		cut := 0
		for range 3 * r.Depth {
			_, n := utf8.DecodeRuneInString(left[cut:])
			cut += n
		}
		tok := "escape"
		if r.Group > 0 && strings.HasPrefix(r.Token, "(") {
			tok = groupToken(r.Group)
		}
		out = append(out, t.fg("border").Render(left[:cut])+t.fg(tok).Render(left[cut:])+t.fg("text").Render(line[len(left):]))
	}
	return strings.Join(out, "\n")
}

// scrollTo adjusts *top so that row sel is visible in h rows of n and
// returns it.
func scrollTo(top *int, sel, h, n int) int {
	h = max(h, 1)
	if sel < *top {
		*top = sel
	}
	if sel >= *top+h {
		*top = sel - h + 1
	}
	*top = clamp(*top, 0, max(0, n-h))
	return *top
}

// --- Code ----------------------------------------------------------------

// snippet is the program for the current input, raw (it is copied as is
// and sanitised only on screen). ok is false until the flavor's flags are
// known.
func snippet(s *State) (string, bool) {
	flags, ok := flagsOf(s, s.Flavor)
	if !ok {
		return "", false
	}
	text := s.Text
	if len(text) > snippetTextLimit {
		text = snippetPlaceholder
	}
	return codegen.Snippet(s.Flavor, engine.Request{
		Pattern: s.Pattern, Flags: flags, Text: text, Mode: s.Mode, Replacement: s.Replacement,
	}), true
}

func (p *detailsPanel) viewCode(s *State, w, h int) string {
	t := s.Theme
	code, ok := snippet(s)
	if !ok {
		return t.fg("dim").Render(fmt.Sprintf("waiting for the %s engine…", s.Flavor))
	}
	lines := p.codeLines(s.Flavor, code, codeStyle(t))
	top := clamp(p.top[tabCode], 0, max(0, len(lines)-h))
	p.top[tabCode] = top
	var out []string
	for _, l := range lines[top:min(len(lines), top+h)] {
		if ansi.StringWidth(l) > w {
			l = ansi.Truncate(l, w, "…") + syntax.Reset
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// codeLines is the snippet as highlighted screen lines (cached per snippet
// and style). Each line is sanitised first, with tabs as four spaces, and
// only then coloured, so no control character of the snippet reaches the
// terminal; every line ends with an SGR reset.
func (p *detailsPanel) codeLines(f engine.Flavor, code, style string) []string {
	if c := &p.code; c.lines == nil || c.code != code || c.style != style {
		lines := strings.Split(strings.TrimSuffix(code, "\n"), "\n")
		for i, l := range lines {
			lines[i] = sanitize(strings.ReplaceAll(l, "\t", "    "))
		}
		*c = codeHighlight{code: code, style: style, lines: syntax.Lines(codeLang[f], lines, style)}
	}
	return p.code.lines
}

// --- Compare -------------------------------------------------------------

// compareCursor is the selected Compare row, starting on the current flavor.
func (p *detailsPanel) compareCursor(s *State) int {
	if !p.started {
		if i := slices.Index(engine.Flavors, s.Flavor); i >= 0 {
			return i
		}
	}
	return p.cmpSel
}

// flavorOutcome is f's latest result for the current pattern: the main
// evaluation for the current flavor, EvalAll for the others. text is the
// text it ran on; ok is false when there is none yet.
func flavorOutcome(s *State, f engine.Flavor) (res *engine.Result, text string, ok bool, err error) {
	if f == s.Flavor {
		if s.EvalFlavor != f || s.EvalPattern != s.Pattern || (s.Result == nil && s.Err == nil) {
			return nil, "", false, nil
		}
		return s.Result, s.ResultText, true, s.Err
	}
	o, found := s.All[f]
	if !found || s.AllPattern != s.Pattern {
		return nil, "", false, nil
	}
	if o.Err != nil {
		return nil, s.AllText, true, o.Err
	}
	return &o.Result, s.AllText, true, nil
}

// currentHint is the hint for the current flavor's compile error.
func currentHint(s *State) (hints.Hint, bool) {
	ce := compileError(s.Err)
	if ce == nil || s.EvalFlavor != s.Flavor || s.EvalPattern != s.Pattern {
		return hints.Hint{}, false
	}
	flags, _ := flagsOf(s, s.Flavor)
	return hints.For(s.Flavor, s.Pattern, flags, ce)
}

// compareRow renders flavor f's row (without the selection style).
func compareRow(s *State, f engine.Flavor) (result, matches, tok string) {
	if s.Runner != nil {
		switch s.Runner.State(f) {
		case runner.NotLoaded, runner.Loading:
			return "loading…", "", "dim"
		case runner.Failed:
			return "✗ failed", firstLine(loadError(s.Runner, f).Error()), "error"
		}
	}
	res, text, ok, err := flavorOutcome(s, f)
	switch {
	case !ok:
		return "…", "", "dim"
	case errors.Is(err, runner.ErrTimeout):
		return "✗ timeout", firstLine(err.Error()), "error"
	case err != nil:
		return "✗ error", firstLine(err.Error()), "error"
	}
	if f != s.Flavor && s.Manual {
		return "✓ compiles", "text too large to compare", "ok"
	}
	if f != s.Flavor && text != s.Text {
		return "✓ compiles", "…", "ok" // values arrive with the next EvalAll
	}
	if f == s.Flavor && s.Mode != engine.ModeMatch {
		return "✓ compiles", "", "ok"
	}
	ms := byteMatches(text, *res)
	if len(ms) == 0 {
		return "✓ compiles", "no matches", "ok"
	}
	vals := make([]string, 0, compareValues)
	for _, m := range ms[:min(len(ms), compareValues)] {
		vals = append(vals, fmt.Sprintf("%q", clipRunes(slice(text, m.Span), 40)))
	}
	more := ""
	if len(ms) > compareValues {
		more = ", …"
	}
	return "✓ compiles", fmt.Sprintf("%d: %s%s", len(ms), strings.Join(vals, ", "), more), "ok"
}

func firstLine(s string) string {
	l, _, _ := strings.Cut(s, "\n")
	return l
}

func (p *detailsPanel) viewCompare(s *State, w, h int) string {
	t := s.Theme
	if s.Pattern == "" {
		return t.fg("dim").Render("type a pattern to compare flavors")
	}
	cur := p.compareCursor(s)
	row := func(flavor, result, matches string) string {
		return padRight(flavor, 9) + padRight(result, 13) + matches
	}
	out := []string{t.fg("dim").Render(fitWidth(row("FLAVOR", "RESULT", "MATCHES"), w))}
	for i, f := range engine.Flavors {
		result, matches, tok := compareRow(s, f)
		mark := "  "
		if f == s.Flavor {
			mark = "● "
		}
		line := sanitize(row(mark+string(f), result, matches))
		if i == cur {
			out = append(out, t.selectedRow().Render(fitWidth(line, w)))
			continue
		}
		name := padRight(mark+string(f), 9)
		out = append(out, t.fg("text").Render(name)+t.fg(tok).Render(sanitize(padRight(result, 13)))+t.fg("text").Render(sanitize(matches)))
	}
	if hint, ok := currentHint(s); ok {
		out = append(out, "")
		for _, l := range wrap("Why: "+sanitize(hint.Why), w) {
			out = append(out, t.fg("anchor").Render(l))
		}
		if hint.Rewrite != "" {
			flags, _ := flagsOf(s, s.Flavor)
			try := t.fg("ok").Render("Try: ") + t.highlightPattern(hint.Rewrite, s.Flavor, flags, noSpan)
			if hint.RewriteNote != "" {
				try += t.fg("dim").Render("  (" + sanitize(hint.RewriteNote) + ")")
			}
			out = append(out, try, t.fg("key").Render("Apply rewrite: a"))
		}
	}
	return strings.Join(out[:min(len(out), max(h, 1))], "\n")
}

// wrap breaks plain text into lines of at most w cells at spaces.
func wrap(text string, w int) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		switch {
		case line == "":
			line = word
		case displayWidth(line)+1+displayWidth(word) <= w:
			line += " " + word
		default:
			lines = append(lines, line)
			line = word
		}
	}
	return append(lines, line)
}

// --- Panel -----------------------------------------------------------------

func (p *detailsPanel) Keys(s *State) []KeyHelp {
	switch s.Tab[PDetails] {
	case tabCode:
		return []KeyHelp{{"y", "Copy snippet"}, {"↑↓", "Scroll"}}
	case tabCompare:
		keys := []KeyHelp{{"enter", "Use flavor"}, {"↑↓", "Move"}}
		if h, ok := currentHint(s); ok && h.Rewrite != "" {
			keys = append([]KeyHelp{{"a", "Apply rewrite"}}, keys...)
		}
		return keys
	}
	return []KeyHelp{{"↑↓", "Select row"}}
}

func (p *detailsPanel) Counter(s *State) string {
	switch s.Tab[PDetails] {
	case tabCode:
		if code, ok := snippet(s); ok {
			n := strings.Count(strings.TrimSuffix(code, "\n"), "\n") + 1
			return fmt.Sprintf("%d %s", n, plural(n, "line", "lines"))
		}
		return ""
	case tabCompare:
		return counterOf(p.compareCursor(s), len(engine.Flavors))
	}
	rows, _ := explanation(s)
	return counterOf(clamp(s.ExplainSel, 0, max(0, len(rows)-1)), len(rows))
}

func (p *detailsPanel) Update(msg tea.Msg, s *State) tea.Cmd {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	key := k.String()
	switch s.Tab[PDetails] {
	case tabExplanation:
		rows, _ := explanation(s)
		switch key {
		case "up", "k":
			s.ExplainSel = max(0, min(s.ExplainSel, len(rows)-1)-1)
		case "down", "j":
			s.ExplainSel = clamp(s.ExplainSel+1, 0, max(0, len(rows)-1))
		}
	case tabCode:
		switch key {
		case "up", "k":
			p.top[tabCode] = max(0, p.top[tabCode]-1)
		case "down", "j":
			p.top[tabCode]++ // clamped when drawn
		case "y":
			if code, ok := snippet(s); ok {
				return tea.SetClipboard(code)
			}
		}
	case tabCompare:
		p.cmpSel, p.started = p.compareCursor(s), true
		switch key {
		case "up", "k":
			p.cmpSel = max(0, p.cmpSel-1)
		case "down", "j":
			p.cmpSel = min(len(engine.Flavors)-1, p.cmpSel+1)
		case "enter":
			s.Flavor = engine.Flavors[p.cmpSel]
		case "a":
			// The only path that edits the pattern for the user.
			if h, ok := currentHint(s); ok && h.Rewrite != "" {
				s.Pattern = h.Rewrite
			}
		}
	}
	return nil
}

func (p *detailsPanel) View(s *State, w, h int) string {
	switch s.Tab[PDetails] {
	case tabCode:
		return p.viewCode(s, w, h)
	case tabCompare:
		return p.viewCompare(s, w, h)
	}
	return p.viewExplanation(s, w, h)
}
