package ui

// detailsPanel is [7]: Explanation, Code and Compare. Placeholder until
// Task 15.
type detailsPanel struct{ basePanel }

func newDetailsPanel() *detailsPanel { return &detailsPanel{} }

func (*detailsPanel) Tabs() []string { return []string{"Explanation", "Code", "Compare"} }

func (*detailsPanel) View(s *State, w, h int) string {
	msg := [...]string{"pattern explanation", "code snippet", "flavor comparison"}[s.Tab[PDetails]]
	return s.Theme.fg("dim").Render(msg)
}
