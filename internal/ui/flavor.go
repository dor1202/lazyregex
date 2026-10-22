package ui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/dor1202/lazyregex/internal/engine"
)

// engineLabels names each flavor's engine before it has loaded.
var engineLabels = map[engine.Flavor]string{
	engine.Python: "CPython re",
	engine.JS:     "QuickJS",
	engine.PCRE:   "PCRE2",
	engine.Go:     "regexp · RE2",
}

// flavorPanel is [1]: the flavor list. Placeholder until Task 14 (no
// navigation yet); it marks and selects the current flavor.
type flavorPanel struct{ basePanel }

func newFlavorPanel() *flavorPanel { return &flavorPanel{} }

func (*flavorPanel) Tabs() []string         { return []string{"Flavor"} }
func (*flavorPanel) Height(*State, int) int { return len(engine.Flavors) }

func (*flavorPanel) Keys(*State) []KeyHelp { return []KeyHelp{{"enter", "Select flavor"}} }

func (*flavorPanel) Counter(s *State) string {
	i := slices.Index(engine.Flavors, s.Flavor)
	if i < 0 {
		return ""
	}
	return counterOf(i, len(engine.Flavors))
}

func (*flavorPanel) View(s *State, w, h int) string {
	rows := make([]string, 0, len(engine.Flavors))
	for _, f := range engine.Flavors {
		mark := " "
		if f == s.Flavor {
			mark = "●"
		}
		rows = append(rows, listRow(s.Theme, fmt.Sprintf("%s %-8s %s", mark, f, engineLabels[f]), w, f == s.Flavor))
	}
	return strings.Join(rows, "\n")
}
