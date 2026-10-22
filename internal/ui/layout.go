package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Layout limits.
const (
	minTermW   = 80
	minTermH   = 24
	minLeft    = 28 // left column width clamp, in cells
	maxLeft    = 60
	leftPct    = 34 // automatic left column width, % of the terminal width
	detailsPct = 40 // [7] height, % of the body height
	resizeStep = 4  // cells per +/_ press
	minBox     = 3  // smallest box: two borders and one content row
)

// rect is a screen rectangle in cells.
type rect struct{ x, y, w, h int }

// layout is the placement of every panel; index by PanelID.
type layout struct {
	panels [panelCount + 1]rect
	bodyH  int
}

func clamp(v, lo, hi int) int { return max(lo, min(v, hi)) }

// autoLeftWidth is the default left column width for a terminal width.
func autoLeftWidth(termW int) int {
	return clamp((termW*leftPct+50)/100, minLeft, maxLeft)
}

// leftWidth is the effective left column width.
func leftWidth(s *State) int {
	if s.LeftWidth == 0 {
		return autoLeftWidth(s.Width)
	}
	return clamp(s.LeftWidth, minLeft, maxLeft)
}

// computeLayout places the panels: the left column stacks [1] [2] [3] [4]
// with [3] taking the remaining height; the right column stacks [5] (auto
// height), [6] (flexible) and [7] (40% of the height). The last row is left
// for the key bar.
func computeLayout(s *State, panels map[PanelID]Panel) layout {
	var l layout
	l.bodyH = s.Height - 1
	lw := leftWidth(s)
	rw := s.Width - lw

	want := func(id PanelID, w int) int { return panels[id].Height(s, w-2) + 2 }

	// Left column.
	left := []PanelID{PFlavor, PFlags, PMatches, PLibrary}
	hs := map[PanelID]int{
		PFlavor:  want(PFlavor, lw),
		PFlags:   want(PFlags, lw),
		PLibrary: want(PLibrary, lw),
	}
	fitFlexible(hs, l.bodyH, []PanelID{PLibrary, PFlags, PFlavor})
	hs[PMatches] = l.bodyH - hs[PFlavor] - hs[PFlags] - hs[PLibrary]
	y := 0
	for _, id := range left {
		l.panels[id] = rect{0, y, lw, hs[id]}
		y += hs[id]
	}

	// Right column.
	hs = map[PanelID]int{
		PPattern: want(PPattern, rw),
		PDetails: max(minBox, (l.bodyH*detailsPct+50)/100),
	}
	fitFlexible(hs, l.bodyH, []PanelID{PDetails, PPattern})
	hs[PText] = l.bodyH - hs[PPattern] - hs[PDetails]
	y = 0
	for _, id := range []PanelID{PPattern, PText, PDetails} {
		l.panels[id] = rect{lw, y, rw, hs[id]}
		y += hs[id]
	}
	return l
}

// fitFlexible shrinks the fixed heights in hs, in the order given, until a
// flexible box of minBox rows fits in total.
func fitFlexible(hs map[PanelID]int, total int, order []PanelID) {
	sum := func() int {
		n := 0
		for _, h := range hs {
			n += h
		}
		return n
	}
	for _, id := range order {
		hs[id] = max(minBox, hs[id])
	}
	for _, id := range order {
		if over := sum() + minBox - total; over > 0 {
			hs[id] = max(minBox, hs[id]-over)
		}
	}
}

// boxSpec describes one bordered panel.
type boxSpec struct {
	title   string // may contain ANSI styling
	counter string // plain text, drawn on the bottom border
	body    string
	w, h    int
	border  string // colour token of the border
}

// renderBox draws a rounded box of exactly w×h cells: the title on the top
// border, the counter at the bottom right, and the body padded or truncated
// by display width (wide runes and ANSI sequences are measured correctly).
func renderBox(t *Theme, b boxSpec) string {
	bs := t.fg(b.border)
	iw, ih := b.w-2, b.h-2
	var sb strings.Builder

	// Top: ╭─title───╮
	title := b.title
	if ansi.StringWidth(title) > iw-1 {
		title = ansi.Truncate(title, iw-1, "…")
	}
	sb.WriteString(bs.Render("╭─"))
	sb.WriteString(title)
	sb.WriteString(bs.Render(strings.Repeat("─", max(0, iw-1-ansi.StringWidth(title))) + "╮"))
	sb.WriteByte('\n')

	// Body.
	lines := strings.Split(b.body, "\n")
	side := bs.Render("│")
	for i := range ih {
		line := ""
		if i < len(lines) {
			line = fitWidth(lines[i], iw)
		} else {
			line = strings.Repeat(" ", iw)
		}
		sb.WriteString(side)
		sb.WriteString(line)
		sb.WriteString(side)
		sb.WriteByte('\n')
	}

	// Bottom: ╰────2 of 4─╯
	counter := b.counter
	if ansi.StringWidth(counter) > iw-1 {
		counter = ""
	}
	fill := iw - ansi.StringWidth(counter)
	if counter != "" {
		fill--
	}
	bottom := "╰" + strings.Repeat("─", fill) + counter
	if counter != "" {
		bottom += "─"
	}
	sb.WriteString(bs.Render(bottom + "╯"))
	return sb.String()
}

// fitWidth truncates s to w cells (with an ellipsis) or pads it with spaces.
func fitWidth(s string, w int) string {
	sw := ansi.StringWidth(s)
	if sw > w {
		s = ansi.Truncate(s, w, "…")
		sw = ansi.StringWidth(s)
	}
	return s + strings.Repeat(" ", max(0, w-sw))
}

// padRight pads s with spaces to at least w cells (by display width).
func padRight(s string, w int) string {
	return s + strings.Repeat(" ", max(0, w-ansi.StringWidth(s)))
}

// tooSmall is shown instead of the layout below minTermW×minTermH.
func tooSmall(t *Theme, w, h int) string {
	msg := t.fg("error").Render("terminal too small (need 80x24)")
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, msg)
}
