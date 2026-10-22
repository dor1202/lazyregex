package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// textPanel is [6]: the test string editor. Placeholder until Task 14: it
// shows the numbered lines but does not edit or highlight them.
type textPanel struct{ editorBase }

func newTextPanel() *textPanel { return &textPanel{} }

func (*textPanel) Tabs() []string { return []string{"Test String"} }

func (*textPanel) Counter(s *State) string {
	n := len(strings.Split(s.Text, "\n"))
	unit := "lines"
	if n == 1 {
		unit = "line"
	}
	return fmt.Sprintf("%d %s · %d chars", n, unit, utf8.RuneCountInString(s.Text))
}

func (*textPanel) View(s *State, w, h int) string {
	t := s.Theme
	lines := strings.Split(s.Text, "\n")
	gw := len(fmt.Sprint(len(lines)))
	out := make([]string, 0, min(h, len(lines)))
	for i, l := range lines[:min(h, len(lines))] {
		out = append(out, t.fg("dim").Render(fmt.Sprintf("%*d ", gw, i+1))+t.fg("text").Render(l))
	}
	return strings.Join(out, "\n")
}
