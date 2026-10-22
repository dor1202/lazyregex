package ui

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dor1202/lazyregex/internal/engine"
)

// invalidUTF8Notice is shown above the text when input bytes were replaced.
const invalidUTF8Notice = "⚠ input had invalid UTF-8 (replaced with �)"

// textPanel is [6]: the test string editor with numbered lines and the
// matches highlighted. In Replace and Split mode it splits into the text
// and the Result side by side. Large (manual mode) input is read-only.
type textPanel struct {
	editorBase
	ed        *editor
	top, left int // scroll position: first line, first content cell
	want      int // a line to scroll to on the next render (-1 = none)
	shownCur  int // the cursor of the last render (-1 = not editing)

	// The last render's geometry, for mouse clicks.
	noticeRows int // rows above the text (the invalid UTF-8 notice)
	textW      int // cells of the text side (the rest is the Result)
	gutter     int // cells of the line numbers and the space after them

	cached string // the text lineStarts belongs to
	starts []int  // byte offset of each line
}

func newTextPanel() *textPanel { return &textPanel{ed: newEditor(true), want: -1, shownCur: -1} }

func (*textPanel) Tabs() []string { return []string{"Test String"} }

func (*textPanel) Counter(s *State) string {
	n := strings.Count(s.Text, "\n") + 1
	return fmt.Sprintf("%d %s · %d chars", n, plural(n, "line", "lines"), utf8.RuneCountInString(s.Text))
}

func (p *textPanel) Keys(s *State) []KeyHelp {
	switch {
	case s.Manual:
		return []KeyHelp{{"enter", "Evaluate"}}
	case s.Editing:
		return []KeyHelp{{"ctrl+z", "Undo"}, {"ctrl+y", "Redo"}}
	}
	return p.editorBase.Keys(s)
}

func (p *textPanel) Update(msg tea.Msg, s *State) tea.Cmd {
	switch msg := msg.(type) {
	case jumpToMatchMsg:
		if msg.match >= 0 && msg.match < len(s.Matches) {
			p.want = p.lineOf(s.Text, s.Matches[msg.match].Span.Start)
		}
		return nil
	case tea.KeyPressMsg, tea.PasteMsg:
	default:
		return nil
	}
	if !s.Editing {
		if k, ok := msg.(tea.KeyPressMsg); ok {
			switch k.String() {
			case "up", "k":
				p.top = max(0, p.top-1)
			case "down", "j":
				p.top++
			}
		}
		return p.editorBase.Update(msg, s)
	}
	if s.Manual {
		return nil // read-only
	}
	p.ed.Sync(s.Text)
	if p.ed.Update(msg) {
		s.Text = p.ed.Value()
	}
	row, _ := p.ed.RowCol()
	p.want = row
	return nil
}

// lineStarts returns the byte offset of every line of text (cached).
func (p *textPanel) lineStarts(text string) []int {
	if p.starts == nil || text != p.cached {
		p.cached = text
		p.starts = append(p.starts[:0], 0)
		for i := 0; ; {
			j := strings.IndexByte(text[i:], '\n')
			if j < 0 {
				break
			}
			i += j + 1
			p.starts = append(p.starts, i)
		}
	}
	return p.starts
}

// lineOf is the line containing byte offset b.
func (p *textPanel) lineOf(text string, b int) int {
	starts := p.lineStarts(text)
	return max(0, sort.SearchInts(starts, b+1)-1)
}

func (p *textPanel) View(s *State, w, h int) string {
	t := s.Theme
	var out []string
	p.noticeRows, p.textW = 0, w
	if s.InvalidUTF8 {
		out = append(out, t.fg("anchor").Render(invalidUTF8Notice))
		h--
		p.noticeRows = 1
	}
	if h <= 0 {
		return strings.Join(out, "\n")
	}
	if s.Mode == engine.ModeReplace || s.Mode == engine.ModeSplit {
		lw := (w - 1) / 2
		p.textW = lw
		left := p.textLines(s, lw, h)
		right := resultLines(s, w-lw-1, h)
		sep := t.fg("border").Render("│")
		for i := range h {
			l, r := "", ""
			if i < len(left) {
				l = left[i]
			}
			if i < len(right) {
				r = right[i]
			}
			out = append(out, fitWidth(l, lw)+sep+r)
		}
		return strings.Join(out, "\n")
	}
	return strings.Join(append(out, p.textLines(s, w, h)...), "\n")
}

// textLines renders up to h numbered, highlighted lines of w cells.
func (p *textPanel) textLines(s *State, w, h int) []string {
	t := s.Theme
	text := s.Text
	starts := p.lineStarts(text)
	n := len(starts)
	gw := len(fmt.Sprint(n))
	cw := max(1, w-gw-1) // content width
	p.gutter = gw + 1

	p.ed.Sync(text) // changed outside the editor: undoable
	editing := s.Focus == PText && s.Editing && !s.Manual
	cursor := -1
	if editing {
		cursor = p.ed.Cursor()
		row, ls := p.ed.RowCol()
		if cursor != p.shownCur { // the cursor moved: bring it into view
			p.want = row
		}
		p.shownCur = cursor
		// Keep the cursor column visible.
		col := displayWidth(text[ls:cursor])
		if col < p.left {
			p.left = col
		} else if col >= p.left+cw {
			p.left = col - cw + 1
		}
	} else {
		p.left, p.shownCur = 0, -1
	}
	if p.want >= 0 {
		if p.want < p.top {
			p.top = p.want
		} else if p.want >= p.top+h {
			p.top = p.want - h + 1
		}
		p.want = -1
	}
	p.top = clamp(p.top, 0, max(0, n-h))

	// Lines spanned by the selected match get a reversed gutter.
	selFrom, selTo := -1, -1
	if s.SelMatch >= 0 && s.SelMatch < len(s.Matches) {
		sp := s.Matches[s.SelMatch].Span
		selFrom = p.lineOf(text, sp.Start)
		selTo = p.lineOf(text, max(sp.Start, sp.End-1))
	}

	out := make([]string, 0, h)
	for i := p.top; i < min(n, p.top+h); i++ {
		ls := starts[i]
		le := lineEnd(text, ls)
		gutter := t.fg("dim").Render(fmt.Sprintf("%*d", gw, i+1))
		if i >= selFrom && i <= selTo {
			gutter = t.fg("text").Reverse(true).Render(fmt.Sprintf("%*d", gw, i+1))
		}
		// Paint only the visible cells, then make sure the line fits.
		from, to, pad := cellWindow(text, ls, le, p.left, cw)
		content := strings.Repeat(" ", pad) + t.paintText(text, from, to, s.Matches, s.SelMatch, cursor)
		content = ansi.Truncate(content, cw, "")
		out = append(out, gutter+" "+content)
	}
	return out
}

// resultLines renders the Replace output or the Split parts.
func resultLines(s *State, w, h int) []string {
	t := s.Theme
	out := []string{t.fg("dim").Render("Result")}
	switch {
	case s.Err != nil || s.Result == nil:
		return append(out, t.fg("dim").Render("no result"))
	case s.Mode == engine.ModeReplace:
		for _, l := range strings.Split(s.Result.Output, "\n") {
			out = append(out, t.fg("text").Render(quoteValue(l)))
		}
	default:
		gw := len(fmt.Sprint(len(s.Result.Parts)))
		for i, part := range s.Result.Parts {
			idx := t.fg("dim").Render(fmt.Sprintf("%*d ", gw, i))
			if i < len(s.Result.PartsNil) && s.Result.PartsNil[i] {
				out = append(out, idx+t.fg("dim").Render("None"))
				continue
			}
			out = append(out, idx+t.fg("text").Render(quoteValue(part)))
		}
	}
	return out[:min(len(out), h)]
}
