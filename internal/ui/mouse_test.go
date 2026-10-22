package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/dor1202/lazyregex/internal/config"
	"github.com/dor1202/lazyregex/internal/engine"
)

// cellOf returns the screen cell of the first occurrence of sub (the
// column is in display cells, not bytes).
func cellOf(t *testing.T, screen, sub string) (x, y int) {
	t.Helper()
	for y, line := range strings.Split(screen, "\n") {
		if i := strings.Index(line, sub); i >= 0 {
			return ansi.StringWidth(line[:i]), y
		}
	}
	t.Fatalf("%q not on screen:\n%s", sub, screen)
	return 0, 0
}

func click(x, y int) tea.MouseClickMsg {
	return tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft}
}

// clickOn clicks the first cell of sub (plus dx cells) on the current screen.
func clickOn(t *testing.T, tm *teatest.TestModel, sub string, dx int) {
	t.Helper()
	x, y := cellOf(t, snap(t, tm).screen, sub)
	tm.Send(click(x+dx, y))
}

// typeText sends each rune of s as a key press.
func typeText(tm *teatest.TestModel, s string) {
	for _, r := range s {
		tm.Send(key(r))
	}
}

func mouseDeps(t *testing.T, pattern, text string) Deps {
	rec := &recorder{}
	r := setup(t, rec, 2*time.Second, nil, nil)
	d := depsFor(r, engine.Python, pattern, text)
	d.Library = config.Library{Entries: []config.Entry{
		{Name: "digits", Flavor: engine.Go, Flags: "", Pattern: `\d+`, Text: "a1b22"},
	}}
	return d
}

func TestClickFocusesPanel(t *testing.T) {
	tm := run(t, mouseDeps(t, "abc", "hello"))
	defer done(t, tm)
	waitFor(t, tm, "ready", ready(engine.Python))

	// Inside [3]'s body: focus, no editing.
	clickOn(t, tm, "[3]", 0)
	x, y := cellOf(t, snap(t, tm).screen, "[3]")
	tm.Send(click(x+2, y+3))
	s := waitFor(t, tm, "focus on [3]", func(s snapshot) bool { return s.state.Focus == PMatches })
	if s.state.Editing {
		t.Error("clicking a list panel started editing")
	}

	// [6] is an editor: clicking it focuses it and starts editing.
	clickOn(t, tm, "hello", 2)
	s = waitFor(t, tm, "focus on [6]", func(s snapshot) bool { return s.state.Focus == PText })
	if !s.state.Editing {
		t.Error("clicking an editor did not start editing")
	}
	// The cursor is where the click was: between "he" and "llo".
	typeText(tm, "X")
	waitFor(t, tm, "typed at the click", func(s snapshot) bool { return s.state.Text == "heXllo" })

	// The border of [7] focuses it too.
	clickOn(t, tm, "[7]", -1)
	waitFor(t, tm, "focus on [7]", func(s snapshot) bool { return s.state.Focus == PDetails })
}

func TestClickFlavorRowSwitches(t *testing.T) {
	tm := run(t, mouseDeps(t, "abc", "hello"))
	defer done(t, tm)
	waitFor(t, tm, "ready", ready(engine.Python))

	x, y := cellOf(t, snap(t, tm).screen, "  go ")
	tm.Send(click(x+4, y))
	s := waitFor(t, tm, "go selected", func(s snapshot) bool { return s.state.Flavor == engine.Go })
	if s.state.Focus != PFlavor || s.state.Pattern != "abc" {
		t.Errorf("focus %v pattern %q, want [1] and the pattern untouched", s.state.Focus, s.state.Pattern)
	}
	if !strings.Contains(s.screen, "│● go ") {
		t.Errorf("go row not marked:\n%s", s.screen)
	}

	// A library row loads the entry.
	clickOn(t, tm, "digits", 1)
	waitFor(t, tm, "entry loaded", func(s snapshot) bool {
		return s.state.Pattern == `\d+` && s.state.Text == "a1b22" && s.state.Focus == PLibrary
	})
}

func TestClickTabSwitches(t *testing.T) {
	tm := run(t, mouseDeps(t, "abc", "hello"))
	defer done(t, tm)
	waitFor(t, tm, "ready", ready(engine.Python))

	clickOn(t, tm, "Groups", 1)
	waitFor(t, tm, "Groups tab", func(s snapshot) bool {
		return s.state.Tab[PMatches] == 1 && s.state.Focus == PMatches
	})
	clickOn(t, tm, "Compare", 0)
	waitFor(t, tm, "Compare tab", func(s snapshot) bool {
		return s.state.Tab[PDetails] == tabCompare && s.state.Focus == PDetails
	})
	clickOn(t, tm, "Replace", 3)
	s := waitFor(t, tm, "Replace tab", func(s snapshot) bool { return s.state.Mode == engine.ModeReplace })
	if s.state.Focus != PPattern || !s.state.Editing {
		t.Errorf("focus %v editing %v, want the pattern editor", s.state.Focus, s.state.Editing)
	}
	// The "[n]" part of a title only focuses.
	clickOn(t, tm, "[3]", 1)
	s = waitFor(t, tm, "focus on [3]", func(s snapshot) bool { return s.state.Focus == PMatches })
	if s.state.Tab[PMatches] != 1 {
		t.Error("clicking the panel number changed its tab")
	}
}

func TestClickPatternSelectsExplanationRow(t *testing.T) {
	// Rows of a(b|cd)e: 0 a, 1 (…), 2 b|cd, 3 b, 4 cd, 5 e.
	tm := run(t, mouseDeps(t, "a(b|cd)e", "acde"))
	defer done(t, tm)
	waitFor(t, tm, "ready", ready(engine.Python))
	// Start on the Code tab with [1] focused.
	send(tm, escKey, key('7'), key(']'), key('1'))
	waitFor(t, tm, "code tab", func(s snapshot) bool { return s.state.Tab[PDetails] == tabCode })

	for _, c := range []struct {
		sub  string
		dx   int
		want int
	}{
		{`a(b|cd)e"`, 5, 4}, // d
		{`a(b|cd)e"`, 3, 2}, // |
		{`a(b|cd)e"`, 1, 1}, // (
		{`a(b|cd)e"`, 7, 5}, // e
	} {
		clickOn(t, tm, c.sub, c.dx)
		s := waitFor(t, tm, fmt.Sprintf("row %d", c.want), func(s snapshot) bool { return s.state.ExplainSel == c.want })
		if s.state.Tab[PDetails] != tabExplanation {
			t.Errorf("[7] tab %d, want Explanation", s.state.Tab[PDetails])
		}
		if s.state.Focus != PPattern || !s.state.Editing {
			t.Errorf("focus %v editing %v, want the pattern editor", s.state.Focus, s.state.Editing)
		}
	}
	// The cursor is on the clicked character (the e, offset 7).
	typeText(tm, "X")
	waitFor(t, tm, "typed at the click", func(s snapshot) bool { return s.state.Pattern == "a(b|cd)Xe" })

	// A click on the delimiter selects no row and leaves the selection.
	clickOn(t, tm, `r"a(`, 0)
	s := waitFor(t, tm, "cursor at 0", func(s snapshot) bool { return true })
	if s.state.ExplainSel != 5 {
		t.Errorf("delimiter click moved the selection to %d", s.state.ExplainSel)
	}
}

func TestClickPatternWideRunesAndScroll(t *testing.T) {
	tm := run(t, mouseDeps(t, "日本語x", "hello"))
	defer done(t, tm)
	waitFor(t, tm, "ready", ready(engine.Python))

	// The second cell of 本 is still 本 (offset 3).
	x, y := cellOf(t, snap(t, tm).screen, `r"日本語x`)
	tm.Send(click(x+2+3, y))
	typeText(tm, "Y")
	waitFor(t, tm, "typed before 本", func(s snapshot) bool { return s.state.Pattern == "日Y本語x" })
	// The x is 7 cells in (after the Y).
	tm.Send(click(x+2+7, y))
	typeText(tm, "Z")
	waitFor(t, tm, "typed before x", func(s snapshot) bool { return s.state.Pattern == "日Y本語Zx" })

	// A long pattern scrolled to its end: the click maps through the window.
	long := strings.Repeat("a", 300) + "QR"
	tm2 := run(t, mouseDeps(t, long, "hello"))
	defer done(t, tm2)
	waitFor(t, tm2, "ready", ready(engine.Python))
	send(tm2, key('-')) // the cursor starts at the end, so the window shows it
	// Wait for the status to settle: its width moves the window.
	waitFor(t, tm2, "evaluated", func(s snapshot) bool {
		return s.state.EvalPattern == long+"-" && strings.Contains(s.screen, "no matches ·")
	})
	clickOn(t, tm2, "QR-", 1)
	typeText(tm2, "+")
	waitFor(t, tm2, "typed before R", func(s snapshot) bool { return s.state.Pattern == strings.Repeat("a", 300)+"Q+R-" })
}

func TestWheelScrollsText(t *testing.T) {
	var lines []string
	for i := 1; i <= 120; i++ {
		lines = append(lines, fmt.Sprintf("row%03d", i))
	}
	tm := run(t, mouseDeps(t, "x", strings.Join(lines, "\n")))
	defer done(t, tm)
	waitFor(t, tm, "ready", ready(engine.Python))

	x, y := cellOf(t, snap(t, tm).screen, "row001")
	tm.Send(tea.MouseWheelMsg{X: x, Y: y, Button: tea.MouseWheelDown})
	s := waitFor(t, tm, "scrolled", func(s snapshot) bool { return !strings.Contains(s.screen, "row001") })
	if !strings.Contains(s.screen, "row004") || s.state.Focus != PPattern {
		t.Errorf("wheel: want row004 on top and focus kept on [5]; focus %v\n%s", s.state.Focus, s.screen)
	}
	tm.Send(tea.MouseWheelMsg{X: x, Y: y, Button: tea.MouseWheelUp})
	waitFor(t, tm, "scrolled back", contains("row001"))

	// While editing [6] the wheel scrolls away from the cursor and stays.
	tm.Send(click(x+1, y))
	waitFor(t, tm, "editing [6]", func(s snapshot) bool { return s.state.Focus == PText && s.state.Editing })
	for range 10 {
		tm.Send(tea.MouseWheelMsg{X: x, Y: y, Button: tea.MouseWheelDown})
	}
	waitFor(t, tm, "scrolled while editing", func(s snapshot) bool {
		return !strings.Contains(s.screen, "row001") && strings.Contains(s.screen, "row031")
	})

	// The wheel over a list moves its selection.
	fx, fy := cellOf(t, snap(t, tm).screen, "python")
	tm.Send(tea.MouseWheelMsg{X: fx, Y: fy, Button: tea.MouseWheelDown})
	s = waitFor(t, tm, "flavor cursor moved", func(s snapshot) bool { return strings.Contains(s.screen, "2 of 4") })
	if s.state.Flavor != engine.Python {
		t.Error("the wheel switched the flavor")
	}
}

func TestDragResizes(t *testing.T) {
	tm := run(t, mouseDeps(t, "x", "hello"))
	defer done(t, tm)
	waitFor(t, tm, "ready", ready(engine.Python))

	s := snap(t, tm)
	lw := leftWidth(&s.state)
	x, y := cellOf(t, s.screen, "╮╭─[5]")
	if x != lw-1 {
		t.Fatalf("border at %d, want %d", x, lw-1)
	}
	tm.Send(click(x, y+5))
	tm.Send(tea.MouseMotionMsg{X: x - 6, Y: y + 5, Button: tea.MouseLeft})
	tm.Send(tea.MouseMotionMsg{X: x - 10, Y: y + 6, Button: tea.MouseLeft})
	tm.Send(tea.MouseReleaseMsg{X: x - 10, Y: y + 6, Button: tea.MouseLeft})
	s = waitFor(t, tm, "resized", func(s snapshot) bool { return s.state.LeftWidth == lw-10 })
	if bx, _ := cellOf(t, s.screen, "╮╭─[5]"); bx != lw-11 {
		t.Errorf("border drawn at %d, want %d", bx, lw-11)
	}
	if s.state.Focus != PPattern {
		t.Errorf("dragging the border moved focus to %v", s.state.Focus)
	}
	// After the release, motion does nothing; dragging is clamped.
	tm.Send(tea.MouseMotionMsg{X: 2, Y: y + 6, Button: tea.MouseLeft})
	tm.Send(click(lw-10, y+3)) // the right column's left border
	tm.Send(tea.MouseMotionMsg{X: 200, Y: y + 3, Button: tea.MouseLeft})
	tm.Send(tea.MouseReleaseMsg{X: 200, Y: y + 3, Button: tea.MouseLeft})
	waitFor(t, tm, "clamped", func(s snapshot) bool { return s.state.LeftWidth == maxLeft })
}

func TestMouseOffIgnoresMouse(t *testing.T) {
	tm := run(t, mouseDeps(t, "x", "hello"))
	defer done(t, tm)
	s := waitFor(t, tm, "ready", ready(engine.Python))
	if s.mouse != tea.MouseModeCellMotion {
		t.Fatalf("mouse mode %v with the mouse on", s.mouse)
	}

	send(tm, escKey, key(':'))
	typeText(tm, "mouse off")
	send(tm, enterKey)
	s = waitFor(t, tm, "mouse off", func(s snapshot) bool { return !s.state.Mouse })
	if s.mouse != tea.MouseModeNone {
		t.Errorf("mouse mode %v with the mouse off, want none", s.mouse)
	}
	before := s.state
	clickOn(t, tm, "[3]", 3)
	clickOn(t, tm, "Groups", 1)
	tm.Send(tea.MouseWheelMsg{X: 3, Y: 2, Button: tea.MouseWheelDown})
	s = snap(t, tm)
	if s.state.Focus != before.Focus || s.state.Tab[PMatches] != 0 || s.state.Editing != before.Editing {
		t.Errorf("a click acted with the mouse off: focus %v tab %d", s.state.Focus, s.state.Tab[PMatches])
	}
	if strings.Contains(s.screen, "2 of 4") {
		t.Error("the wheel acted with the mouse off")
	}

	// And on again, at runtime.
	send(tm, key(':'))
	typeText(tm, "mouse")
	send(tm, enterKey)
	s = waitFor(t, tm, "mouse on", func(s snapshot) bool { return s.state.Mouse })
	if s.mouse != tea.MouseModeCellMotion {
		t.Errorf("mouse mode %v after :mouse", s.mouse)
	}
	clickOn(t, tm, "[3]", 3)
	waitFor(t, tm, "click works again", func(s snapshot) bool { return s.state.Focus == PMatches })
}

func TestClickWithOverlayOpen(t *testing.T) {
	tm := run(t, mouseDeps(t, "x", "hello"))
	defer done(t, tm)
	waitFor(t, tm, "ready", ready(engine.Python))

	// Help: a click inside does nothing, a click outside only closes it.
	send(tm, escKey, key('?'))
	s := waitFor(t, tm, "help", contains("Keybindings"))
	clickOn(t, tm, "Mouse (when on)", 0)
	if s = snap(t, tm); !strings.Contains(s.screen, "Mouse (when on)") {
		t.Fatal("a click inside the popup closed it")
	}
	clickOn(t, tm, "[1]", 1)
	s = waitFor(t, tm, "help closed", func(s snapshot) bool { return !strings.Contains(s.screen, "Mouse (when on)") })
	if s.state.Focus != PPattern {
		t.Errorf("the click acted on [1] under the popup: focus %v", s.state.Focus)
	}

	// The prompt: a click cancels it without acting.
	send(tm, key(':'))
	typeText(tm, "mode split")
	clickOn(t, tm, "[3]", 3)
	s = waitFor(t, tm, "prompt closed", func(s snapshot) bool { return !strings.Contains(s.screen, ":mode split") })
	if s.state.Focus != PPattern || s.state.Mode != engine.ModeMatch {
		t.Errorf("the click acted under the prompt: focus %v mode %v", s.state.Focus, s.state.Mode)
	}
}

func TestHitMapAt(t *testing.T) {
	m := New(Deps{Config: config.Defaults()}).(*Model)
	m.state.Width, m.state.Height = 120, 40
	m.View()
	lw := leftWidth(&m.state)
	for _, c := range []struct {
		x, y int
		want Hit
		ok   bool
	}{
		{lw - 1, 10, Hit{Kind: hitBorder}, true},
		{lw, 10, Hit{Kind: hitBorder}, true},
		{3, 1, Hit{Panel: PFlavor, Kind: hitBody, Row: 0, Col: 2, Offset: -1}, true},
		{0, 1, Hit{Panel: PFlavor, Kind: hitFrame, Offset: -1}, true},
		{5, 39, Hit{}, false}, // the key bar
		{500, 5, Hit{}, false},
	} {
		got, ok := m.hits.At(c.x, c.y)
		if ok != c.ok || (ok && (got.Panel != c.want.Panel || got.Kind != c.want.Kind ||
			(c.want.Kind == hitBody && (got.Row != c.want.Row || got.Col != c.want.Col)))) {
			t.Errorf("At(%d,%d) = %+v %v, want %+v %v", c.x, c.y, got, ok, c.want, c.ok)
		}
	}
}

func TestClickSelectsListRows(t *testing.T) {
	rec := &recorder{}
	r := setup(t, rec, 2*time.Second, map[engine.Flavor]evalFunc{engine.Python: matchAll(`\d+`)}, nil)
	tm := run(t, depsFor(r, engine.Python, `x\d+`, "a1b22c333"))
	defer done(t, tm)
	waitFor(t, tm, "matches", contains("✓ 3 matches"))

	clickOn(t, tm, "#2", 3)
	waitFor(t, tm, "match 2 selected", func(s snapshot) bool {
		return s.state.SelMatch == 1 && s.state.Focus == PMatches
	})

	// An explanation row ("\d+" is row 1).
	clickOn(t, tm, "a digit", 0)
	waitFor(t, tm, "row 1 selected", func(s snapshot) bool {
		return s.state.ExplainSel == 1 && s.state.Focus == PDetails
	})

	// A flag row is selected, not toggled.
	clickOn(t, tm, "[ ] m", 2)
	s := waitFor(t, tm, "flag row", contains("3 of 3"))
	if s.state.Focus != PFlags || s.state.Flags[engine.Python] != "" {
		t.Errorf("focus %v flags %q: want [2] focused and no flag toggled", s.state.Focus, s.state.Flags[engine.Python])
	}
}
