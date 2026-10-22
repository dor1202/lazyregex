package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/dor1202/lazyregex/internal/version"
)

// keyBar renders the bottom line: the focused panel's keys, then the global
// ones, as "Desc: key | …" in the key colour, and the version dimmed on the
// right. Entries that do not fit are dropped from the end.
func (m *Model) keyBar(w int) string {
	s := &m.state
	editing := m.editing()
	items := m.panels[s.Focus].Keys(s)
	if !editing && len(m.panels[s.Focus].Tabs()) > 1 {
		items = append(items, KeyHelp{"[ ]", "Switch tab"})
	}
	for _, b := range globalBindings {
		if b.bar && b.activeWhen(editing) {
			items = append(items, b.help)
		}
	}

	right := "lazyregex " + version.Version
	room := w - ansi.StringWidth(right) - 2 // keep a gap before the version
	sep := " | "
	var parts []string
	used := 0
	for _, it := range items {
		p := it.Desc + ": " + it.Key
		add := ansi.StringWidth(p)
		if len(parts) > 0 {
			add += len(sep)
		}
		if used+add > room {
			break
		}
		parts = append(parts, p)
		used += add
	}
	left := m.theme.fg("key").Render(strings.Join(parts, sep))
	gap := max(1, w-used-ansi.StringWidth(right))
	return fitWidth(left+strings.Repeat(" ", gap)+m.theme.fg("dim").Render(right), w)
}
