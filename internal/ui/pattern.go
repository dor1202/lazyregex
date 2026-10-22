package ui

import "github.com/dor1202/lazyregex/internal/engine"

// patternPanel is [5]: the pattern editor, with tabs Match, Replace and Split
// (the shell maps the tab to State.Mode). Placeholder until Task 14: it shows
// the pattern but does not edit it.
type patternPanel struct{ editorBase }

func newPatternPanel() *patternPanel { return &patternPanel{} }

func (*patternPanel) Tabs() []string { return []string{"Match", "Replace", "Split"} }

// Height is one line, two in Replace mode.
func (*patternPanel) Height(s *State, _ int) int {
	if s.Mode == engine.ModeReplace {
		return 2
	}
	return 1
}

func (*patternPanel) View(s *State, w, h int) string {
	t := s.Theme
	line := t.fg("text").Render(s.Pattern)
	if s.Pattern == "" {
		line = t.fg("dim").Render("type a pattern…")
	}
	if s.Mode == engine.ModeReplace {
		line += "\n" + t.fg("dim").Render("replace ") + t.fg("text").Render(s.Replacement)
	}
	return line
}
