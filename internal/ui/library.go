package ui

// libraryPanel is [4]: saved patterns. Placeholder until Task 15 (lists the
// entries, no actions yet).
type libraryPanel struct {
	basePanel
	sel int
}

func newLibraryPanel() *libraryPanel { return &libraryPanel{} }

func (*libraryPanel) Tabs() []string { return []string{"Library"} }

// Height shows up to five entries, at least three rows.
func (*libraryPanel) Height(s *State, _ int) int { return clamp(len(s.Library.Entries), 3, 5) }

func (p *libraryPanel) Counter(s *State) string { return counterOf(p.sel, len(s.Library.Entries)) }

func (p *libraryPanel) View(s *State, w, h int) string {
	if len(s.Library.Entries) == 0 {
		return s.Theme.fg("dim").Render("no saved patterns")
	}
	out := ""
	for i, e := range s.Library.Entries {
		if i > 0 {
			out += "\n"
		}
		out += listRow(s.Theme, "  "+padRight(e.Name, 20)+" "+string(e.Flavor), w, i == p.sel)
	}
	return out
}
