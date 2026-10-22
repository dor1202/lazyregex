package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dor1202/lazyregex/internal/config"
	"github.com/dor1202/lazyregex/internal/engine"
	"github.com/dor1202/lazyregex/internal/runner"
)

// Deps is everything the UI needs from main.
type Deps struct {
	Runner    *runner.Runner
	Config    config.Config
	ConfigDir string
	Library   config.Library
	Initial   InitialInput
}

// InitialInput is the starting flavor, pattern and text (from the command
// line or stdin). TextPrepared, when its Text is set, wins over Text.
type InitialInput struct {
	Flavor       engine.Flavor
	Pattern      string
	Text         string
	TextPrepared runner.Prepared
}

// Model is the Bubble Tea model: the shared State, the seven panels and the
// theme. Focus, tabs, layout and global keys live here; everything else is
// in the panels.
type Model struct {
	state   State
	panels  map[PanelID]Panel
	theme   *Theme
	library config.Library
	ev      evalLoop
}

// patternModes maps the Pattern panel's tabs to evaluation modes.
var patternModes = []engine.Mode{engine.ModeMatch, engine.ModeReplace, engine.ModeSplit}

// New builds the UI model. Focus starts in the pattern editor.
func New(d Deps) tea.Model {
	m := &Model{library: d.Library}
	m.theme = newTheme(true, d.Config.Theme)

	flavor := d.Initial.Flavor
	if flavor == "" {
		flavor = d.Config.DefaultFlavor
	}
	if flavor == "" {
		flavor = engine.Python
	}
	text := d.Initial.Text
	prep := d.Initial.TextPrepared
	if prep.Text != "" {
		text = prep.Text
	}
	m.state = State{
		Flavor:      flavor,
		Flags:       map[engine.Flavor]string{},
		Mode:        engine.ModeMatch,
		Pattern:     d.Initial.Pattern,
		Text:        text,
		Manual:      prep.Manual,
		InvalidUTF8: prep.InvalidUTF8,
		Stale:       prep.Manual,
		Focus:       PPattern,
		Editing:     true,
		Tab:         map[PanelID]int{},
		Mouse:       d.Config.Mouse,
		Runner:      d.Runner,
		Config:      d.Config,
		ConfigDir:   d.ConfigDir,
		Library:     &m.library,
		Theme:       m.theme,
	}
	m.panels = map[PanelID]Panel{
		PFlavor:  newFlavorPanel(),
		PFlags:   newFlagsPanel(),
		PMatches: newMatchesPanel(),
		PLibrary: newLibraryPanel(),
		PPattern: newPatternPanel(),
		PText:    newTextPanel(),
		PDetails: newDetailsPanel(),
	}
	return m
}

// Init asks the terminal for its background colour (to pick the light or
// dark theme), starts loading the initial flavor, listens for runner events
// and initialises the panels.
func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tea.RequestBackgroundColor, m.initEval()}
	for id := PanelID(1); id <= panelCount; id++ {
		cmds = append(cmds, m.panels[id].Init(&m.state))
	}
	return tea.Batch(cmds...)
}

// Update handles global messages and keys and forwards the rest: keys and
// pastes go to the focused panel, other messages to every panel. Afterwards
// any change to the evaluation inputs schedules an evaluation.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := m.update(msg)
	return m, tea.Batch(cmd, m.afterUpdate())
}

func (m *Model) update(msg tea.Msg) tea.Cmd {
	if cmd, ok := m.updateEval(msg); ok {
		// Runner events are also broadcast so panels can react to them.
		if _, ev := msg.(runnerEventMsg); !ev {
			return cmd
		}
		return tea.Batch(cmd, m.broadcast(msg))
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.state.Width, m.state.Height = msg.Width, msg.Height
		return nil
	case tea.BackgroundColorMsg:
		m.theme = newTheme(msg.IsDark(), m.state.Config.Theme)
		m.state.Theme = m.theme
		return nil
	case tea.PasteMsg:
		// Only the editor being edited receives a paste.
		if m.editing() {
			return m.panels[m.state.Focus].Update(msg, &m.state)
		}
		return nil
	case tea.KeyPressMsg:
		if m.manualEnter(msg) {
			return func() tea.Msg { return evalNowMsg{} }
		}
		if b, ok := m.globalBinding(msg.String()); ok {
			return b.run(m, msg.String())
		}
		return m.panels[m.state.Focus].Update(msg, &m.state)
	}
	return m.broadcast(msg)
}

// broadcast sends msg to every panel.
func (m *Model) broadcast(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	for id := PanelID(1); id <= panelCount; id++ {
		cmds = append(cmds, m.panels[id].Update(msg, &m.state))
	}
	return tea.Batch(cmds...)
}

// manualEnter reports whether key is enter asking for a manual evaluation:
// large input, in the pattern editor or on the (read-only) test string.
func (m *Model) manualEnter(k tea.KeyPressMsg) bool {
	s := &m.state
	if !s.Manual || k.String() != "enter" {
		return false
	}
	return (s.Focus == PPattern && s.Editing) || s.Focus == PText
}

// editing reports whether keys go straight to an editor panel.
func (m *Model) editing() bool {
	return m.state.Editing && m.panels[m.state.Focus].IsEditor()
}

// focus moves focus to id; focusing an editor panel starts editing.
func (m *Model) focus(id PanelID) tea.Cmd {
	m.state.Focus = id
	m.state.Editing = m.panels[id].IsEditor()
	return nil
}

// switchTab moves the focused panel's tab by delta, wrapping. The Pattern
// panel's tab selects the evaluation mode.
func (m *Model) switchTab(delta int) {
	id := m.state.Focus
	n := len(m.panels[id].Tabs())
	if n < 2 {
		return
	}
	t := (m.state.Tab[id] + delta + n) % n
	m.state.Tab[id] = t
	if id == PPattern {
		m.state.Mode = patternModes[t]
	}
}

// resizeLeft changes the left column width by delta cells, clamped.
func (m *Model) resizeLeft(delta int) {
	m.state.LeftWidth = clamp(leftWidth(&m.state)+delta, minLeft, maxLeft)
}

// openHelp opens the keybindings popup (Task 15).
func (m *Model) openHelp() tea.Cmd { return nil }

// openPrompt opens the command prompt (Task 15).
func (m *Model) openPrompt() tea.Cmd { return nil }

// View renders the panels and the key bar, or a notice when the terminal is
// smaller than 80x24.
func (m *Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}

func (m *Model) render() string {
	s := &m.state
	if s.Width <= 0 || s.Height <= 0 {
		return ""
	}
	if s.Width < minTermW || s.Height < minTermH {
		return tooSmall(m.theme, s.Width, s.Height)
	}
	l := computeLayout(s, m.panels)
	box := func(id PanelID) string {
		r := l.panels[id]
		p := m.panels[id]
		return renderBox(m.theme, boxSpec{
			title:   m.title(id, m.borderToken(id)),
			counter: p.Counter(s),
			body:    p.View(s, r.w-2, r.h-2),
			w:       r.w,
			h:       r.h,
			border:  m.borderToken(id),
		})
	}
	left := lipgloss.JoinVertical(lipgloss.Left, box(PFlavor), box(PFlags), box(PMatches), box(PLibrary))
	right := lipgloss.JoinVertical(lipgloss.Left, box(PPattern), box(PText), box(PDetails))
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	return body + "\n" + m.keyBar(s.Width)
}

// borderToken is the border colour: red for a pattern error, green for the
// focused panel, grey otherwise.
func (m *Model) borderToken(id PanelID) string {
	switch {
	case id == PPattern && m.state.Err != nil:
		return "error"
	case id == m.state.Focus:
		return "focus"
	}
	return "border"
}

// title renders "[n]─Tab ─ Tab": the active tab of a tabbed panel green and
// bold, other tabs dim; a single title bold, green when focused.
func (m *Model) title(id PanelID, borderTok string) string {
	t := m.theme
	tabs := m.panels[id].Tabs()
	active := m.state.Tab[id]
	var parts []string
	for i, name := range tabs {
		var st lipgloss.Style
		switch {
		case len(tabs) == 1 && id != m.state.Focus:
			st = t.fg("title").Bold(true)
		case i == active:
			st = t.fg("focus").Bold(true)
		default:
			st = t.fg("dim")
		}
		parts = append(parts, st.Render(name))
	}
	sep := t.fg(borderTok).Render(" ─ ")
	num := t.fg("title").Render("[" + string(rune('0'+id)) + "]")
	return num + t.fg(borderTok).Render("─") + strings.Join(parts, sep)
}
