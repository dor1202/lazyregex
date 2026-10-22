package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/dor1202/lazyregex/internal/engine"
)

// flagsPanel is [2]: the selected flavor's flags (its engine's
// Info().Flags), toggled with space. Flags are stored per flavor.
type flagsPanel struct {
	basePanel
	sel int
}

func newFlagsPanel() *flagsPanel { return &flagsPanel{} }

func (*flagsPanel) Tabs() []string { return []string{"Flags"} }

// flagInfos returns the flavor's flags; false until its engine is loaded.
func flagInfos(s *State, f engine.Flavor) ([]engine.FlagInfo, bool) {
	if s.Runner == nil {
		return nil, false
	}
	info, ok := s.Runner.Info(f)
	return info.Flags, ok
}

// flagsOf returns the flag letters for flavor f: the user's choice, or the
// engine's defaults. ok is false while the engine is not loaded, because
// its flags are unknown until then.
func flagsOf(s *State, f engine.Flavor) (string, bool) {
	infos, ok := flagInfos(s, f)
	if !ok {
		return "", false
	}
	if v, set := s.Flags[f]; set {
		return v, true
	}
	var b strings.Builder
	for _, fi := range infos {
		if fi.Default {
			b.WriteByte(fi.Letter)
		}
	}
	return b.String(), true
}

// Height is one row per flag; five rows (as in the mockup) while loading.
func (*flagsPanel) Height(s *State, _ int) int {
	if infos, ok := flagInfos(s, s.Flavor); ok {
		return max(1, len(infos))
	}
	return 5
}

func (p *flagsPanel) Counter(s *State) string {
	infos, _ := flagInfos(s, s.Flavor)
	return counterOf(min(p.sel, len(infos)-1), len(infos))
}

func (*flagsPanel) Keys(*State) []KeyHelp {
	return []KeyHelp{{"space", "Toggle flag"}, {"↑↓", "Move"}}
}

func (p *flagsPanel) Update(msg tea.Msg, s *State) tea.Cmd {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	infos, ok := flagInfos(s, s.Flavor)
	if !ok || len(infos) == 0 {
		return nil
	}
	p.sel = clamp(p.sel, 0, len(infos)-1)
	switch k.String() {
	case "up", "k":
		p.sel = max(0, p.sel-1)
	case "down", "j":
		p.sel = min(len(infos)-1, p.sel+1)
	case "space":
		cur, _ := flagsOf(s, s.Flavor)
		letter := infos[p.sel].Letter
		var b strings.Builder
		for _, fi := range infos { // keep the engine's order
			on := strings.IndexByte(cur, fi.Letter) >= 0
			if fi.Letter == letter {
				on = !on
			}
			if on {
				b.WriteByte(fi.Letter)
			}
		}
		if s.Flags == nil {
			s.Flags = map[engine.Flavor]string{}
		}
		s.Flags[s.Flavor] = b.String()
	}
	return nil
}

func (p *flagsPanel) View(s *State, w, h int) string {
	infos, ok := flagInfos(s, s.Flavor)
	if !ok {
		return s.Theme.fg("dim").Render("flags appear once the engine loads")
	}
	if len(infos) == 0 {
		return s.Theme.fg("dim").Render("no flags")
	}
	cur, _ := flagsOf(s, s.Flavor)
	sel := clamp(p.sel, 0, len(infos)-1)
	rows := make([]string, len(infos))
	for i, fi := range infos {
		box := "[ ]"
		if strings.IndexByte(cur, fi.Letter) >= 0 {
			box = "[x]"
		}
		rows[i] = listRow(s.Theme, fmt.Sprintf("%s %c  %s", box, fi.Letter, fi.Name), w, i == sel)
	}
	return strings.Join(rows, "\n")
}
