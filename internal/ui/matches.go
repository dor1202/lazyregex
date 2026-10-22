package ui

// matchesPanel is [3]: matches and their groups. Placeholder until Task 14.
type matchesPanel struct{ basePanel }

func newMatchesPanel() *matchesPanel { return &matchesPanel{} }

func (*matchesPanel) Tabs() []string { return []string{"Matches", "Groups"} }

func (*matchesPanel) View(s *State, w, h int) string {
	return s.Theme.fg("dim").Render("no matches yet")
}
