package ui

import (
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dor1202/lazyregex/internal/engine"
)

// Mouse support (when State.Mouse is on): a click focuses the panel under
// the pointer, selects a row, switches a tab or places an editor's cursor;
// clicking a pattern character also selects its explanation row. The wheel
// scrolls the panel under the pointer and dragging the column border
// resizes the columns. Mouse messages are routed by hit-testing the last
// rendered frame, never broadcast.

// wheelLines is how many lines one wheel step scrolls a text panel.
const wheelLines = 3

// hitKind is what a screen cell belongs to.
type hitKind int

const (
	hitFrame  hitKind = iota // a panel's border or title outside its tabs
	hitBody                  // a panel's content area
	hitTab                   // a tab label on a tabbed panel's title
	hitBorder                // the border between the two columns
)

// Hit is what a screen cell shows.
type Hit struct {
	Panel    PanelID
	Kind     hitKind
	Row, Col int // hitBody: the content cell
	Tab      int // hitTab: the tab index
	// Offset is, on [5]'s pattern or replacement line, the byte offset
	// under the cell (-1 elsewhere); OnChar is set when the cell shows a
	// character of that text (not a delimiter or the space after it).
	Offset int
	OnChar bool
}

// tabSpan is a tab label's cells [x0, x1) on its panel's top border.
type tabSpan struct{ x0, x1, tab int }

// textCell maps one content cell of [5]'s pattern or replacement line.
type textCell struct {
	off  int  // byte offset in the text
	char bool // the cell shows a character of the text
}

// hitMap maps screen cells to panels, rows, tabs and pattern offsets. It
// is rebuilt on every View from the layout used to draw that frame.
type hitMap struct {
	ok      bool // a layout was drawn (not "terminal too small")
	panels  [panelCount + 1]rect
	tabs    [panelCount + 1][]tabSpan
	border  int  // x of the left column's right edge; border+1 is the right column's left edge
	bodyH   int  // rows above the key bar
	popup   rect // the help popup, when open (w > 0)
	pattern [2][]textCell
}

// buildHits records the frame just drawn with layout l.
func (m *Model) buildHits(l layout, popup rect) hitMap {
	h := hitMap{ok: true, panels: l.panels, bodyH: l.bodyH, popup: popup}
	h.border = l.panels[PFlavor].w - 1
	for id := PanelID(1); id <= panelCount; id++ {
		tabs := m.panels[id].Tabs()
		if len(tabs) < 2 {
			continue
		}
		r := l.panels[id]
		// The title is "╭─" + "[n]─" + tabs joined by " ─ ", cut at
		// r.w-3 cells (see title and renderBox).
		end := r.x + 2 + r.w - 3
		x := r.x + 2 + 4
		for i, name := range tabs {
			w := ansi.StringWidth(name)
			if x >= end {
				break
			}
			h.tabs[id] = append(h.tabs[id], tabSpan{x, min(x+w, end), i})
			x += w + 3
		}
	}
	if p, ok := m.panels[PPattern].(*patternPanel); ok {
		h.pattern = p.cells
	}
	return h
}

// in reports whether (x, y) is inside r.
func (r rect) in(x, y int) bool { return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h }

// At returns what cell (x, y) of the last frame shows; ok is false outside
// the panels (the key bar, or no layout).
func (h hitMap) At(x, y int) (Hit, bool) {
	if !h.ok || y < 0 || y >= h.bodyH {
		return Hit{}, false
	}
	if x == h.border || x == h.border+1 {
		return Hit{Kind: hitBorder, Offset: -1}, true
	}
	for id := PanelID(1); id <= panelCount; id++ {
		r := h.panels[id]
		if !r.in(x, y) {
			continue
		}
		hit := Hit{Panel: id, Kind: hitFrame, Offset: -1}
		switch {
		case y == r.y:
			for _, t := range h.tabs[id] {
				if x >= t.x0 && x < t.x1 {
					hit.Kind, hit.Tab = hitTab, t.tab
				}
			}
		case y < r.y+r.h-1 && x > r.x && x < r.x+r.w-1:
			hit.Kind, hit.Row, hit.Col = hitBody, y-r.y-1, x-r.x-1
			if id == PPattern && hit.Row < len(h.pattern) && hit.Col < len(h.pattern[hit.Row]) {
				c := h.pattern[hit.Row][hit.Col]
				hit.Offset, hit.OnChar = c.off, c.char
			}
		}
		return hit, true
	}
	return Hit{}, false
}

// clicker is a panel that reacts to a click in its content area.
type clicker interface{ click(s *State, h Hit) tea.Cmd }

// scroller is a panel that scrolls by wheel steps itself; other non-editor
// panels treat a wheel step as ↑/↓.
type scroller interface{ scroll(s *State, delta int) }

// mouse handles a mouse message (the mouse is on).
func (m *Model) mouse(msg tea.MouseMsg) tea.Cmd {
	ev := msg.Mouse()
	switch msg.(type) {
	case tea.MouseReleaseMsg:
		m.dragging = false
		return nil
	case tea.MouseMotionMsg:
		if m.dragging {
			m.state.LeftWidth = clamp(ev.X+m.dragOffset, minLeft, maxLeft)
		}
		return nil
	case tea.MouseWheelMsg:
		return m.wheel(ev)
	case tea.MouseClickMsg:
		if ev.Button != tea.MouseLeft {
			return nil
		}
		return m.click(ev)
	}
	return nil
}

// click acts on a left click. With the prompt or the help popup open, a
// click only closes it (a click inside the popup does nothing).
func (m *Model) click(ev tea.Mouse) tea.Cmd {
	m.note = nil
	switch {
	case m.prompt != nil:
		m.prompt = nil
		return nil
	case m.help != nil:
		if !m.hits.popup.in(ev.X, ev.Y) {
			m.help = nil
		}
		return nil
	}
	hit, ok := m.hits.At(ev.X, ev.Y)
	if !ok {
		return nil
	}
	switch hit.Kind {
	case hitBorder:
		m.dragging = true
		m.dragOffset = leftWidth(&m.state) - ev.X
		return nil
	case hitTab:
		m.focus(hit.Panel)
		m.setTab(hit.Panel, hit.Tab)
		return nil
	}
	m.focus(hit.Panel)
	if c, ok := m.panels[hit.Panel].(clicker); ok && hit.Kind == hitBody {
		return c.click(&m.state, hit)
	}
	return nil
}

// wheel scrolls the panel under the pointer (or moves the help popup's
// selection while it is open).
func (m *Model) wheel(ev tea.Mouse) tea.Cmd {
	var delta int
	switch ev.Button {
	case tea.MouseWheelUp:
		delta = -1
	case tea.MouseWheelDown:
		delta = +1
	default:
		return nil
	}
	arrow := tea.KeyPressMsg{Code: tea.KeyDown}
	if delta < 0 {
		arrow = tea.KeyPressMsg{Code: tea.KeyUp}
	}
	switch {
	case m.prompt != nil:
		return nil
	case m.help != nil:
		m.help.update(arrow)
		return nil
	}
	hit, ok := m.hits.At(ev.X, ev.Y)
	if !ok || hit.Kind == hitBorder {
		return nil
	}
	p := m.panels[hit.Panel]
	if sc, ok := p.(scroller); ok {
		sc.scroll(&m.state, delta)
		return nil
	}
	if p.IsEditor() {
		return nil
	}
	return p.Update(arrow, &m.state)
}

// selectExplanationAt selects the deepest explanation row whose span
// contains pattern byte offset off and shows the Explanation tab.
func selectExplanationAt(s *State, off int) {
	rows, _ := explanation(s)
	best := -1
	for i, r := range rows {
		if r.Span[0] <= off && off < r.Span[1] && (best < 0 || r.Depth > rows[best].Depth) {
			best = i
		}
	}
	if best >= 0 {
		s.ExplainSel = best
		s.Tab[PDetails] = tabExplanation
	}
}

// lineCells maps the w content cells of an editor line to byte offsets of
// text. The line draws prefix cells (a delimiter or label) before text and
// is scrolled left cells; cells from avail on show something else.
func lineCells(text string, prefix, left, avail, w int) []textCell {
	var at []int // the byte offset drawn in each cell of text
	for i, r := range text {
		for range runeCells(r) {
			at = append(at, i)
		}
	}
	cells := make([]textCell, max(w, 0))
	for c := range cells {
		x := left + c - prefix
		switch {
		case c >= avail || x >= len(at):
			cells[c] = textCell{len(text), false}
		case x < 0:
			cells[c] = textCell{0, false}
		default:
			cells[c] = textCell{at[x], true}
		}
	}
	return cells
}

// byteAtCell is the byte offset in text[from:to] of the rune drawn at
// display cell cell (to when the cell is past the end).
func byteAtCell(text string, from, to, cell int) int {
	acc := 0
	for i := from; i < to; {
		r, n := utf8.DecodeRuneInString(text[i:to])
		acc += runeCells(r)
		if cell < acc {
			return i
		}
		i += n
	}
	return to
}

// --- panels ------------------------------------------------------------------

func (p *flavorPanel) click(s *State, h Hit) tea.Cmd {
	if h.Row < len(engine.Flavors) {
		p.sel, p.started = h.Row, true
		s.Flavor = engine.Flavors[h.Row]
	}
	return nil
}

func (p *flagsPanel) click(s *State, h Hit) tea.Cmd {
	if infos, ok := flagInfos(s, s.Flavor); ok && h.Row < len(infos) {
		p.sel = h.Row
	}
	return nil
}

func (p *matchesPanel) click(s *State, h Hit) tea.Cmd {
	if s.Result == nil || s.Mode != engine.ModeMatch || len(s.Matches) == 0 {
		return nil // the panel shows a notice, not rows
	}
	if rs := p.rows(s); p.top+h.Row < len(rs) {
		r := rs[p.top+h.Row]
		s.SelMatch, s.SelRow = r.match, r.group
	}
	return nil
}

func (p *libraryPanel) click(s *State, h Hit) tea.Cmd {
	i := p.top + h.Row
	if i >= len(s.Library.Entries) {
		return nil
	}
	p.sel = i
	if err := loadEntry(s, s.Library.Entries[i]); err != nil {
		return errNotice(err)
	}
	return nil
}

func (p *detailsPanel) click(s *State, h Hit) tea.Cmd {
	switch s.Tab[PDetails] {
	case tabExplanation:
		rows, _ := explanation(s)
		if i := p.top[tabExplanation] + h.Row; i < len(rows) {
			s.ExplainSel = i
		}
	case tabCompare:
		if i := h.Row - 1; i >= 0 && i < len(engine.Flavors) && s.Pattern != "" {
			p.cmpSel, p.started = i, true
		}
	}
	return nil
}

func (p *patternPanel) click(s *State, h Hit) tea.Cmd {
	if h.Offset < 0 {
		return nil
	}
	p.sync(s)
	if h.Row == 1 {
		p.line = 1
		p.repl.SetCursor(h.Offset)
		return nil
	}
	p.line = 0
	p.pat.SetCursor(h.Offset)
	if h.OnChar {
		selectExplanationAt(s, h.Offset)
	}
	return nil
}

func (p *textPanel) click(s *State, h Hit) tea.Cmd {
	row := h.Row - p.noticeRows
	if row < 0 || h.Col >= p.textW || s.Manual {
		return nil
	}
	text := s.Text
	starts := p.lineStarts(text)
	off := len(text)
	if line := p.top + row; line < len(starts) {
		ls := starts[line]
		off = byteAtCell(text, ls, lineEnd(text, ls), p.left+max(0, h.Col-p.gutter))
	}
	p.ed.Sync(text)
	p.ed.SetCursor(off)
	p.shownCur = p.ed.Cursor() // already in view: no scrolling to it
	return nil
}

func (p *textPanel) scroll(_ *State, delta int) {
	p.top = max(0, p.top+delta*wheelLines) // clamped when drawn
}
