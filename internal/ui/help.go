package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// mouseHelp describes the mouse (Task 16), listed in the help popup.
var mouseHelp = []KeyHelp{
	{"click", "Focus pane, select row, switch tab"},
	{"click pattern", "Select its explanation row"},
	{"wheel", "Scroll the pane under the pointer"},
	{"drag border", "Resize the columns"},
	{":mouse", "Turn the mouse on or off"},
}

// helpKeys are the popup's own keys (shown in the key bar while it is open).
var helpKeys = []KeyHelp{{"/", "Filter"}, {"↑↓", "Move"}, {"esc", "Close"}}

// helpSection is one titled group of the popup.
type helpSection struct {
	title string
	rows  []KeyHelp
}

// helpPopup is the "?" keybindings popup: the focused panel's keys, then the
// global keys and commands, then the mouse. "/" types a filter that keeps
// the rows containing it.
type helpPopup struct {
	sections  []helpSection
	filter    string
	filtering bool // keys go to the filter
	sel, top  int
}

// localKeys are the focused panel's keys as the key bar shows them.
func (m *Model) localKeys() []KeyHelp {
	s := &m.state
	items := m.panels[s.Focus].Keys(s)
	if !m.editing() && len(m.panels[s.Focus].Tabs()) > 1 {
		items = append(items, KeyHelp{"[ ]", "Switch tab"})
	}
	return items
}

// newHelp builds the popup from the same tables as the key bar.
func (m *Model) newHelp() *helpPopup {
	s := &m.state
	name := s.Focus.String()
	if s.Focus == PDetails {
		name = m.panels[PDetails].Tabs()[s.Tab[PDetails]]
	}
	var global []KeyHelp
	for _, b := range globalBindings {
		global = append(global, b.help)
	}
	for _, c := range commands {
		global = append(global, KeyHelp{strings.TrimSpace(":" + c.name + " " + c.args), c.desc})
	}
	return &helpPopup{sections: []helpSection{
		{"Local · " + name, m.localKeys()},
		{"Global", global},
		{"Mouse (when on)", mouseHelp},
	}}
}

// visible returns the sections narrowed by the filter (empty ones dropped).
func (h *helpPopup) visible() []helpSection {
	f := strings.ToLower(h.filter)
	var out []helpSection
	for _, sec := range h.sections {
		var rows []KeyHelp
		for _, r := range sec.rows {
			if strings.Contains(strings.ToLower(r.Key+" "+r.Desc), f) {
				rows = append(rows, r)
			}
		}
		if len(rows) > 0 {
			out = append(out, helpSection{sec.title, rows})
		}
	}
	return out
}

func (h *helpPopup) count() int {
	n := 0
	for _, sec := range h.visible() {
		n += len(sec.rows)
	}
	return n
}

// update handles a key; done reports that the popup closed.
func (h *helpPopup) update(msg tea.Msg) (done bool) {
	switch msg := msg.(type) {
	case tea.PasteMsg:
		if h.filtering {
			h.filter += strings.ReplaceAll(msg.Content, "\n", " ")
			h.sel = 0
		}
		return false
	case tea.KeyPressMsg:
		k := msg.String()
		switch {
		case k == "esc":
			return true
		case k == "up":
			h.sel = max(0, h.sel-1)
		case k == "down":
			h.sel = min(max(0, h.count()-1), h.sel+1)
		case h.filtering && k == "enter":
			h.filtering = false
		case h.filtering && k == "backspace":
			if r := []rune(h.filter); len(r) > 0 {
				h.filter = string(r[:len(r)-1])
			}
			h.sel = 0
		case h.filtering && isTyped(msg):
			h.filter += msg.Text
			h.sel = 0
		case k == "/":
			h.filtering = true
		case k == "?" || k == "q":
			return true
		case k == "k":
			h.sel = max(0, h.sel-1)
		case k == "j":
			h.sel = min(max(0, h.count()-1), h.sel+1)
		}
	}
	return false
}

// view renders the popup box for a w×h terminal (the box is smaller).
func (h *helpPopup) view(t *Theme, w, ht int) (box string, bw, bh int) {
	secs := h.visible()
	n := h.count()
	h.sel = clamp(h.sel, 0, max(0, n-1))

	keyW := 12 // the key column fits the longest key
	for _, sec := range secs {
		for _, r := range sec.rows {
			keyW = max(keyW, displayWidth(r.Key))
		}
	}
	keyW = min(keyW, 32)
	bw = min(keyW+48, w-8)
	iw := bw - 2
	var lines []string
	selLine, row := 0, 0
	for i, sec := range secs {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, t.fg("title").Bold(true).Render(sec.title))
		for _, r := range sec.rows {
			text := "  " + padRight(sanitize(r.Key), keyW) + " " + sanitize(r.Desc)
			if row == h.sel {
				selLine = len(lines)
				lines = append(lines, t.selectedRow().Render(fitWidth(text, iw)))
			} else {
				lines = append(lines, t.fg("key").Render(padRight(sanitize("  "+r.Key), keyW+2))+" "+t.fg("text").Render(sanitize(r.Desc)))
			}
			row++
		}
	}
	if n == 0 {
		lines = append(lines, t.fg("dim").Render("no keys match"))
	}
	bodyH := min(len(lines), ht-6) // room for the borders, the filter line and margins
	bodyH = max(bodyH, 1)
	top := scrollTo(&h.top, selLine, bodyH, len(lines))
	body := append([]string{}, lines[top:min(len(lines), top+bodyH)]...)
	for len(body) < bodyH {
		body = append(body, "")
	}
	filter := t.fg("key").Render("/ ")
	switch {
	case h.filter != "" || h.filtering:
		filter += t.fg("text").Render(sanitize(h.filter))
		if h.filtering {
			filter += t.paintStyle(paint{reverse: true}).Render(" ")
		}
	default:
		filter += t.fg("dim").Render("▏ type to filter")
	}
	body = append(body, filter)
	bh = len(body) + 2
	box = renderBox(t, boxSpec{
		title:   t.fg("focus").Bold(true).Render("Keybindings"),
		counter: counterOf(h.sel, n),
		body:    strings.Join(body, "\n"),
		w:       bw,
		h:       bh,
		border:  "focus",
	})
	return box, bw, bh
}

// overlay draws box (bw×bh cells) centred over base, a w-cell-wide screen.
func overlay(base, box string, w, bw, bh int) string {
	lines := strings.Split(base, "\n")
	boxLines := strings.Split(box, "\n")
	x := max(0, (w-bw)/2)
	y := max(0, (len(lines)-bh)/2)
	for i, bl := range boxLines {
		j := y + i
		if j >= len(lines) {
			break
		}
		l := lines[j]
		lines[j] = ansi.Cut(l, 0, x) + bl + ansi.Cut(l, x+bw, w)
	}
	return strings.Join(lines, "\n")
}
