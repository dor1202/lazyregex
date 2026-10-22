package ui

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dor1202/lazyregex/internal/engine"
	"github.com/dor1202/lazyregex/internal/runner"
)

// engineLabels names each flavor's engine before it has loaded.
var engineLabels = map[engine.Flavor]string{
	engine.Python: "CPython re",
	engine.JS:     "QuickJS",
	engine.PCRE:   "PCRE2",
	engine.Go:     "regexp · RE2",
}

// flavorPanel is [1]: the flavor list. ↑/↓ (j/k) move the cursor and enter
// selects; ● marks the current flavor and a red ✗ a flavor where the
// pattern does not compile. Switching never changes the pattern.
type flavorPanel struct {
	basePanel
	sel     int
	started bool // sel has been put on the current flavor
}

func newFlavorPanel() *flavorPanel { return &flavorPanel{} }

func (*flavorPanel) Tabs() []string         { return []string{"Flavor"} }
func (*flavorPanel) Height(*State, int) int { return len(engine.Flavors) }

func (*flavorPanel) Keys(*State) []KeyHelp {
	return []KeyHelp{{"enter", "Select flavor"}, {"↑↓", "Move"}}
}

// cursor is the highlighted row, starting on the current flavor.
func (p *flavorPanel) cursor(s *State) int {
	if !p.started {
		if i := slices.Index(engine.Flavors, s.Flavor); i >= 0 {
			return i
		}
	}
	return p.sel
}

func (p *flavorPanel) Counter(s *State) string {
	return counterOf(p.cursor(s), len(engine.Flavors))
}

func (p *flavorPanel) Update(msg tea.Msg, s *State) tea.Cmd {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	p.sel, p.started = p.cursor(s), true
	switch k.String() {
	case "up", "k":
		p.sel = max(0, p.sel-1)
	case "down", "j":
		p.sel = min(len(engine.Flavors)-1, p.sel+1)
	case "enter":
		s.Flavor = engine.Flavors[p.sel]
	}
	return nil
}

func (p *flavorPanel) View(s *State, w, h int) string {
	t := s.Theme
	cur := p.cursor(s)
	rows := make([]string, 0, len(engine.Flavors))
	for i, f := range engine.Flavors {
		base := t.fg("text")
		if i == cur {
			base = t.selectedRow()
		}
		mark := " "
		if f == s.Flavor {
			mark = "●"
		}
		label := engineLabels[f]
		var notes []string // rendered after the label, never truncated away
		noteW := 0
		addNote := func(text, tok string) {
			notes = append(notes, base.Foreground(t.color(tok)).Render(text))
			noteW += ansi.StringWidth(text)
		}
		if s.Runner != nil {
			if info, ok := s.Runner.Info(f); ok {
				label = strings.TrimSpace(info.EngineName + " " + info.Version)
			}
			switch s.Runner.State(f) {
			case runner.Loading:
				addNote(" loading…", "dim")
			case runner.Failed:
				addNote(" failed", "error")
			}
		}
		if flavorFails(s, f) {
			addNote(" ✗", "error")
		}
		text := strings.TrimRight(fitWidth(fmt.Sprintf("%s %-8s %s", mark, f, label), max(0, w-noteW)), " ")
		pad := strings.Repeat(" ", max(0, w-ansi.StringWidth(text)-noteW))
		rows = append(rows, base.Render(text)+strings.Join(notes, "")+base.Render(pad))
	}
	return strings.Join(rows, "\n")
}
