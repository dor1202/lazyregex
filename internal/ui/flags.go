package ui

// flagsPanel is [2]: the current flavor's flags. Placeholder until Task 14.
type flagsPanel struct{ basePanel }

func newFlagsPanel() *flagsPanel { return &flagsPanel{} }

func (*flagsPanel) Tabs() []string { return []string{"Flags"} }

// Height reserves the mockup's five flag rows.
func (*flagsPanel) Height(*State, int) int { return 5 }

func (*flagsPanel) View(s *State, w, h int) string {
	return s.Theme.fg("dim").Render("flags appear once the engine loads")
}
