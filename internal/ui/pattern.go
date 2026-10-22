package ui

import (
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dor1202/lazyregex/internal/engine"
	"github.com/dor1202/lazyregex/internal/runner"
)

// maxErrorLines bounds the engine error shown under the pattern.
const maxErrorLines = 3

// patternPanel is [5]: the pattern editor with tabs Match, Replace and Split
// (the shell maps the tab to State.Mode). Replace adds a replacement line;
// ↑/↓ move between the two. The pattern is syntax coloured; on an error the
// engine's message is shown verbatim below it and its offset underlined.
type patternPanel struct {
	editorBase
	pat, repl *editor
	line      int // 0 = pattern, 1 = replacement
}

func newPatternPanel() *patternPanel {
	return &patternPanel{pat: newEditor(false), repl: newEditor(false)}
}

func (*patternPanel) Tabs() []string { return []string{"Match", "Replace", "Split"} }

// errorLines is the current error, verbatim, split into lines.
func errorLines(s *State) []string {
	if s.Err == nil {
		return nil
	}
	lines := strings.Split(strings.TrimRight(s.Err.Error(), "\n"), "\n")
	lines = lines[:min(len(lines), maxErrorLines)]
	for i, l := range lines {
		lines[i] = sanitize(l)
	}
	return lines
}

// Height is the pattern line, the replacement line in Replace mode, and the
// error lines.
func (*patternPanel) Height(s *State, _ int) int {
	n := 1
	if s.Mode == engine.ModeReplace {
		n++
	}
	return n + len(errorLines(s))
}

func (p *patternPanel) Keys(s *State) []KeyHelp {
	if !s.Editing {
		return p.editorBase.Keys(s)
	}
	keys := []KeyHelp{{"ctrl+z", "Undo"}, {"ctrl+y", "Redo"}}
	if s.Mode == engine.ModeReplace {
		keys = append(keys, KeyHelp{"↑↓", "Pattern/replacement"})
	}
	if s.Manual {
		keys = append([]KeyHelp{{"enter", "Evaluate"}}, keys...)
	}
	return keys
}

// sync loads text changed outside the editors (initial input, library);
// it runs before every edit and render.
func (p *patternPanel) sync(s *State) {
	if p.pat.Value() != s.Pattern {
		p.pat.SetValue(s.Pattern)
	}
	if p.repl.Value() != s.Replacement {
		p.repl.SetValue(s.Replacement)
	}
	if s.Mode != engine.ModeReplace {
		p.line = 0
	}
}

func (p *patternPanel) Update(msg tea.Msg, s *State) tea.Cmd {
	switch msg.(type) {
	case tea.KeyPressMsg, tea.PasteMsg:
	default:
		return nil
	}
	if !s.Editing {
		return p.editorBase.Update(msg, s)
	}
	p.sync(s)
	if k, ok := msg.(tea.KeyPressMsg); ok && s.Mode == engine.ModeReplace {
		switch k.String() {
		case "up":
			p.line = 0
			return nil
		case "down":
			p.line = 1
			return nil
		}
	}
	if p.line == 1 {
		if p.repl.Update(msg) {
			s.Replacement = p.repl.Value()
		}
		return nil
	}
	if p.pat.Update(msg) {
		s.Pattern = p.pat.Value()
	}
	return nil
}

func (p *patternPanel) View(s *State, w, h int) string {
	t := s.Theme
	p.sync(s)
	editing := s.Focus == PPattern && s.Editing
	flags, _ := flagsOf(s, s.Flavor)
	pre, post := patternDelims(s.Flavor, flags)

	underline := [2]int{-1, -1}
	if ce := compileError(s.Err); ce != nil && s.EvalPattern == s.Pattern && s.EvalFlavor == s.Flavor {
		underline = errorUnderline(s.Pattern, s.Flavor, ce.Offset)
	}
	cursor := -1
	if editing && p.line == 0 {
		cursor = p.pat.Cursor()
	}
	body := t.paintPattern(s.Pattern, s.Flavor, flags, underline, explainSpan(s), cursor)
	if s.Pattern == "" {
		body += t.fg("dim").Render("type a pattern…")
	}
	line := t.fg("delim").Render(pre) + body + t.fg("delim").Render(post)
	focus := displayWidth(pre)
	if cursor >= 0 {
		focus += displayWidth(s.Pattern[:cursor])
	}
	status := p.status(s)
	avail := w
	if status != "" {
		avail = max(1, w-ansi.StringWidth(status)-1)
	}
	line = padRight(window(line, focus, avail), avail)
	if status != "" {
		line += " " + status
	}
	out := []string{line}

	if s.Mode == engine.ModeReplace {
		label := "replace "
		cur := -1
		if editing && p.line == 1 {
			cur = p.repl.Cursor()
		}
		repl := t.renderRuns(s.Replacement, 0, len(s.Replacement), func(int) paint { return paint{fg: "text"} }, cur)
		rfocus := len(label)
		if cur >= 0 {
			rfocus += displayWidth(s.Replacement[:cur])
		}
		out = append(out, window(t.fg("dim").Render(label)+repl, rfocus, w))
	}
	for _, l := range errorLines(s) {
		out = append(out, t.fg("error").Render(l))
	}
	return strings.Join(out, "\n")
}

// status is the right side of the pattern line: loading, evaluation and
// match count.
func (p *patternPanel) status(s *State) string {
	t := s.Theme
	if s.Runner == nil {
		return ""
	}
	switch s.Runner.State(s.Flavor) {
	case runner.NotLoaded, runner.Loading:
		return t.fg("dim").Render(fmt.Sprintf("loading %s engine…", s.Flavor))
	case runner.Failed:
		return t.fg("error").Render("✗ engine failed")
	}
	switch {
	case s.Evaluating:
		return t.fg("dim").Render("evaluating…")
	case s.Manual && s.Stale:
		return t.fg("anchor").Render("large input: press enter to evaluate")
	case s.Pattern == "":
		return ""
	case errors.Is(s.Err, runner.ErrTimeout):
		return t.fg("error").Render("✗ timeout")
	case s.Err != nil:
		return t.fg("error").Render("✗ error")
	case s.Result == nil:
		return ""
	}
	took := fmt.Sprintf("%.2f ms", float64(s.EvalTime.Microseconds())/1000)
	switch s.Mode {
	case engine.ModeReplace:
		return t.fg("ok").Render("✓ replaced · " + took)
	case engine.ModeSplit:
		return t.fg("ok").Render(fmt.Sprintf("✓ %d parts · %s", len(s.Result.Parts), took))
	}
	n := len(s.Result.Matches)
	if n == 0 {
		return t.fg("dim").Render("no matches · " + took)
	}
	return t.fg("ok").Render(fmt.Sprintf("✓ %d %s · %s", n, plural(n, "match", "matches"), took))
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// displayWidth is the cell width of s as renderRuns draws it.
func displayWidth(s string) int {
	return ansi.StringWidth(sanitize(s))
}

// window cuts a styled line to w cells, scrolled horizontally so that cell
// focus stays visible.
func window(s string, focus, w int) string {
	if ansi.StringWidth(s) <= w {
		return s
	}
	left := max(0, focus-w+2)
	return ansi.Cut(s, left, left+w)
}
