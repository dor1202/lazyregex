package ui

import (
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/dor1202/lazyregex/internal/config"
	"github.com/dor1202/lazyregex/internal/runner"
)

// libraryPanel is [4]: saved patterns. enter loads the selected entry, n
// saves the current input under a name typed in the prompt (":save "), and
// d deletes the selected entry after a y/n prompt line.
type libraryPanel struct {
	basePanel
	sel, top int
}

func newLibraryPanel() *libraryPanel { return &libraryPanel{} }

func (*libraryPanel) Tabs() []string { return []string{"Library"} }

// Height shows up to five entries, at least three rows.
func (*libraryPanel) Height(s *State, _ int) int { return clamp(len(s.Library.Entries), 3, 5) }

func (p *libraryPanel) Counter(s *State) string {
	return counterOf(p.cursor(s), len(s.Library.Entries))
}

func (p *libraryPanel) cursor(s *State) int {
	return clamp(p.sel, 0, max(0, len(s.Library.Entries)-1))
}

func (*libraryPanel) Keys(s *State) []KeyHelp {
	if len(s.Library.Entries) == 0 {
		return []KeyHelp{{"n", "Save current"}}
	}
	return []KeyHelp{{"enter", "Load"}, {"n", "Save current"}, {"d", "Delete"}, {"↑↓", "Move"}}
}

func (p *libraryPanel) Update(msg tea.Msg, s *State) tea.Cmd {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	n := len(s.Library.Entries)
	p.sel = p.cursor(s)
	switch k.String() {
	case "up", "k":
		p.sel = max(0, p.sel-1)
	case "down", "j":
		p.sel = min(max(0, n-1), p.sel+1)
	case "n":
		s.ask = newPrompt("save ")
	case "enter":
		if n > 0 {
			loadEntry(s, s.Library.Entries[p.sel])
		}
	case "d":
		if n > 0 {
			name := s.Library.Entries[p.sel].Name
			s.ask = &prompt{
				question: fmt.Sprintf("delete %q? (y/n)", name),
				yes:      func(s *State) (string, error) { return deleteEntry(s, name) },
			}
		}
	}
	return nil
}

func (p *libraryPanel) View(s *State, w, h int) string {
	if len(s.Library.Entries) == 0 {
		return s.Theme.fg("dim").Render("no saved patterns · n saves the current one")
	}
	cur := p.cursor(s)
	top := scrollTo(&p.top, cur, h, len(s.Library.Entries))
	var out []string
	for i := top; i < min(len(s.Library.Entries), top+max(h, 1)); i++ {
		e := s.Library.Entries[i]
		out = append(out, listRow(s.Theme, "  "+padRight(sanitize(e.Name), 20)+" "+string(e.Flavor), w, i == cur))
	}
	return strings.Join(out, "\n")
}

// loadEntry puts a library entry into the state: flavor, its flags,
// pattern, replacement and text.
func loadEntry(s *State, e config.Entry) {
	if e.Flavor != "" {
		s.Flavor = e.Flavor
	}
	s.Flags[s.Flavor] = e.Flags
	s.Pattern, s.Replacement = e.Pattern, e.Replacement
	prep := runner.PrepareText([]byte(e.Text))
	s.Text, s.Manual, s.InvalidUTF8, s.Stale = prep.Text, prep.Manual, prep.InvalidUTF8, prep.Manual
	s.ExplainSel = 0
}

// errNoConfigDir is returned when the library cannot be written.
var errNoConfigDir = errors.New("no config directory: library kept in memory only")

// saveEntry stores the current input as name and writes the library.
func saveEntry(s *State, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("usage: save <name>")
	}
	flags, ok := flagsOf(s, s.Flavor)
	if !ok {
		flags = s.Flags[s.Flavor]
	}
	s.Library.Put(config.Entry{
		Name: name, Flavor: s.Flavor, Flags: flags,
		Pattern: s.Pattern, Replacement: s.Replacement, Text: s.Text,
	})
	if err := writeLibrary(s); err != nil {
		return "", err
	}
	return fmt.Sprintf("saved %q", name), nil
}

// deleteEntry removes name from the library and writes it.
func deleteEntry(s *State, name string) (string, error) {
	s.Library.Delete(name)
	if err := writeLibrary(s); err != nil {
		return "", err
	}
	return fmt.Sprintf("deleted %q", name), nil
}

// openEntry loads the entry called name.
func openEntry(s *State, name string) error {
	for _, e := range s.Library.Entries {
		if e.Name == name {
			loadEntry(s, e)
			return nil
		}
	}
	return fmt.Errorf("no saved pattern named %q", name)
}

func writeLibrary(s *State) error {
	if s.ConfigDir == "" {
		return errNoConfigDir
	}
	return config.SaveLibrary(s.ConfigDir, *s.Library)
}
