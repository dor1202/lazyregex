package ui

import tea "charm.land/bubbletea/v2"

// binding is one global key. Keys are tea.KeyPressMsg.String() values.
//
// Never bind ctrl+m, ctrl+i, ctrl+[, ctrl+s or ctrl+q: terminals send the
// first three for enter, tab and esc, and the last two are flow control.
// ("tab" is bound by name; ctrl+i is never bound separately.)
type binding struct {
	keys       []string
	help       KeyHelp
	inEditor   bool // also active while an editor panel is being edited
	onlyEditor bool // active only while an editor panel is being edited
	bar        bool // shown in the key bar (all are shown in the help popup)
	run        func(m *Model, key string) tea.Cmd
}

// globalBindings is the global keymap, walked by the key handler, the key
// bar and the help popup.
var globalBindings = []binding{
	{keys: []string{"ctrl+c"}, help: KeyHelp{"ctrl+c", "Quit"}, inEditor: true,
		run: func(*Model, string) tea.Cmd { return tea.Quit }},
	{keys: []string{"esc"}, help: KeyHelp{"esc", "Leave editor"}, inEditor: true, onlyEditor: true, bar: true,
		run: func(m *Model, _ string) tea.Cmd { m.state.Editing = false; return nil }},
	{keys: []string{"tab"}, help: KeyHelp{"tab", "Next pane"}, inEditor: true, bar: true,
		run: func(m *Model, _ string) tea.Cmd { return m.focus(m.state.Focus%panelCount + 1) }},
	{keys: []string{"shift+tab"}, help: KeyHelp{"shift+tab", "Previous pane"}, inEditor: true,
		run: func(m *Model, _ string) tea.Cmd { return m.focus((m.state.Focus+panelCount-2)%panelCount + 1) }},
	{keys: []string{"1", "2", "3", "4", "5", "6", "7"}, help: KeyHelp{"1-7", "Focus pane"},
		run: func(m *Model, k string) tea.Cmd { return m.focus(PanelID(k[0] - '0')) }},
	{keys: []string{"["}, help: KeyHelp{"[", "Previous tab"},
		run: func(m *Model, _ string) tea.Cmd { m.switchTab(-1); return nil }},
	{keys: []string{"]"}, help: KeyHelp{"]", "Next tab"},
		run: func(m *Model, _ string) tea.Cmd { m.switchTab(+1); return nil }},
	{keys: []string{"+"}, help: KeyHelp{"+", "Widen left column"},
		run: func(m *Model, _ string) tea.Cmd { m.resizeLeft(+resizeStep); return nil }},
	{keys: []string{"_"}, help: KeyHelp{"_", "Narrow left column"},
		run: func(m *Model, _ string) tea.Cmd { m.resizeLeft(-resizeStep); return nil }},
	{keys: []string{":"}, help: KeyHelp{":", "Command"}, bar: true,
		run: func(m *Model, _ string) tea.Cmd { return m.openPrompt() }},
	{keys: []string{"?"}, help: KeyHelp{"?", "Keybindings"}, bar: true,
		run: func(m *Model, _ string) tea.Cmd { return m.openHelp() }},
	{keys: []string{"q"}, help: KeyHelp{"q", "Quit"},
		run: func(*Model, string) tea.Cmd { return tea.Quit }},
}

// globalBinding returns the binding for key in the current context.
func (m *Model) globalBinding(key string) (binding, bool) {
	editing := m.editing()
	for _, b := range globalBindings {
		if !b.activeWhen(editing) {
			continue
		}
		for _, k := range b.keys {
			if k == key {
				return b, true
			}
		}
	}
	return binding{}, false
}

// activeWhen reports whether the binding applies in the editing context.
func (b binding) activeWhen(editing bool) bool {
	if editing {
		return b.inEditor
	}
	return !b.onlyEditor
}
