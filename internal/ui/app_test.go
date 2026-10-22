package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/dor1202/lazyregex/internal/config"
	"github.com/dor1202/lazyregex/internal/engine"
	"github.com/dor1202/lazyregex/internal/runner"
)

const (
	testW = 160
	testH = 48
)

// fakeEngine returns no matches; the shell never evaluates anyway.
type fakeEngine struct{ f engine.Flavor }

func (e fakeEngine) Info() engine.Info { return engine.Info{Flavor: e.f, EngineName: "fake"} }
func (e fakeEngine) Close() error      { return nil }
func (e fakeEngine) Eval(ctx context.Context, req engine.Request) (engine.Result, error) {
	return engine.Result{}, ctx.Err()
}

func fakeRunner(t *testing.T) *runner.Runner {
	t.Helper()
	m := map[engine.Flavor]runner.Factory{}
	for _, f := range engine.Flavors {
		m[f] = func(context.Context, string) (engine.Engine, error) { return fakeEngine{f}, nil }
	}
	r := runner.New(m, t.TempDir(), time.Second)
	t.Cleanup(r.Close)
	return r
}

func testDeps(t *testing.T) Deps {
	return Deps{
		Runner: fakeRunner(t),
		Config: config.Defaults(),
		Library: config.Library{Entries: []config.Entry{
			{Name: "log line parser", Flavor: engine.Python},
			{Name: "semver", Flavor: engine.JS},
		}},
		Initial: InitialInput{
			Flavor:  engine.Python,
			Pattern: `^(?P<date>\d{4}-\d{2}-\d{2}) (?P<level>[A-Z]+)`,
			Text:    "2026-10-20 INFO  server started\n2026-10-20 WARN  cache miss ratio 0.42\n日本語 😀 wide runes",
		},
	}
}

func startTest(t *testing.T) *teatest.TestModel {
	t.Helper()
	return teatest.NewTestModel(t, New(testDeps(t)), teatest.WithInitialTermSize(testW, testH))
}

func key(code rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: code, Text: string(code)} }

var (
	tabKey      = tea.KeyPressMsg{Code: tea.KeyTab}
	shiftTabKey = tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	escKey      = tea.KeyPressMsg{Code: tea.KeyEscape}
)

// finish quits the program and returns the final model.
func finish(t *testing.T, tm *teatest.TestModel) *Model {
	t.Helper()
	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	return tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second)).(*Model)
}

// screen renders the model and strips ANSI sequences.
func screen(m *Model) string { return ansi.Strip(m.View().Content) }

func TestShellRendersSevenPanels(t *testing.T) {
	m := finish(t, startTest(t))
	out := screen(m)
	for _, want := range []string{
		"[1]─Flavor", "[2]─Flags", "[3]─Matches ─ Groups", "[4]─Library",
		"[5]─Match ─ Replace ─ Split", "[6]─Test String", "[7]─Explanation ─ Code ─ Compare",
		"lazyregex dev",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("screen lacks %q\n%s", want, out)
		}
	}
	lines := strings.Split(out, "\n")
	if len(lines) != testH {
		t.Fatalf("got %d lines, want %d", len(lines), testH)
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w != testW {
			t.Errorf("line %d is %d cells wide, want %d: %q", i, w, testW, l)
		}
	}
}

func TestFocusNumberKeys(t *testing.T) {
	tm := startTest(t)
	tm.Send(escKey) // leave the pattern editor
	tm.Type("3")
	m := finish(t, tm)
	s := m.StateForTest()
	if s.Focus != PMatches {
		t.Fatalf("focus = %v, want PMatches", s.Focus)
	}
	// The focused border uses the focus colour, the others the border colour.
	focus := m.theme.fg("focus").Render("╭─")
	if !strings.Contains(m.View().Content, focus) {
		t.Fatalf("no focus-coloured border in output")
	}
}

func TestNumberKeysTypeInsideEditor(t *testing.T) {
	tm := startTest(t)
	tm.Type("3")
	m := finish(t, tm)
	if s := m.StateForTest(); s.Focus != PPattern || !s.Editing {
		t.Fatalf("focus = %v editing = %v, want pattern editor kept", s.Focus, s.Editing)
	}
}

func TestEscLeavesEditor(t *testing.T) {
	tm := startTest(t)
	tm.Send(escKey)
	m := finish(t, tm)
	if s := m.StateForTest(); s.Focus != PPattern || s.Editing {
		t.Fatalf("focus = %v editing = %v, want pattern focused, not editing", s.Focus, s.Editing)
	}
}

func TestTabCyclesFocus(t *testing.T) {
	tm := startTest(t)
	tm.Send(escKey)
	tm.Type("7")
	tm.Send(tabKey) // wraps to [1]
	tm.Send(tabKey) // [2]
	m := finish(t, tm)
	if s := m.StateForTest(); s.Focus != PFlags {
		t.Fatalf("after tab tab: focus = %v, want PFlags", s.Focus)
	}

	tm = startTest(t)
	tm.Send(shiftTabKey) // [5] editor -> [4], works inside the editor
	m = finish(t, tm)
	if s := m.StateForTest(); s.Focus != PLibrary || s.Editing {
		t.Fatalf("after shift-tab: focus = %v editing = %v, want PLibrary", s.Focus, s.Editing)
	}

	tm = startTest(t)
	tm.Send(escKey)
	tm.Type("4")
	tm.Send(tabKey) // into the pattern editor
	m = finish(t, tm)
	if s := m.StateForTest(); s.Focus != PPattern || !s.Editing {
		t.Fatalf("tab into [5]: focus = %v editing = %v", s.Focus, s.Editing)
	}
}

func TestTabSwitchesPanelTabs(t *testing.T) {
	tm := startTest(t)
	tm.Send(escKey)
	tm.Type("]")
	tm.Type("7")
	tm.Type("[")
	m := finish(t, tm)
	s := m.StateForTest()
	if s.Mode != engine.ModeReplace || s.Tab[PPattern] != 1 {
		t.Fatalf("mode = %v tab = %d, want replace", s.Mode, s.Tab[PPattern])
	}
	if s.Tab[PDetails] != 2 { // wraps backwards to Compare
		t.Fatalf("details tab = %d, want 2", s.Tab[PDetails])
	}
}

func TestKeyBarFollowsFocus(t *testing.T) {
	tm := startTest(t)
	tm.Send(escKey)
	tm.Type("1")
	m := finish(t, tm)
	bar := lastLine(screen(m))
	if !strings.Contains(bar, "Select flavor: enter") {
		t.Fatalf("bar = %q", bar)
	}
	if !strings.Contains(bar, "Keybindings: ?") || !strings.HasSuffix(strings.TrimRight(bar, " "), "lazyregex dev") {
		t.Fatalf("bar lacks globals or version: %q", bar)
	}

	m.state.Focus, m.state.Editing = PPattern, true
	bar = lastLine(screen(m))
	if !strings.Contains(bar, "Leave editor: esc") || strings.Contains(bar, "Select flavor") {
		t.Fatalf("editor bar = %q", bar)
	}
}

func lastLine(s string) string {
	lines := strings.Split(s, "\n")
	return lines[len(lines)-1]
}

func TestResizeLeftColumn(t *testing.T) {
	tm := startTest(t)
	tm.Send(escKey)
	tm.Type("+")
	m := finish(t, tm)
	auto := autoLeftWidth(testW) // 34% of 160 = 54
	if got := m.StateForTest().LeftWidth; got != auto+4 {
		t.Fatalf("after +: LeftWidth = %d, want %d", got, auto+4)
	}
	// The left column's right corner moves with it.
	first := strings.Split(screen(m), "\n")[0]
	if r := []rune(first); r[auto+4-1] != '╮' {
		t.Fatalf("left column does not end at %d: %q", auto+4, first)
	}

	tm = startTest(t)
	tm.Send(escKey)
	for range 10 {
		tm.Type("_")
	}
	m = finish(t, tm)
	if got := m.StateForTest().LeftWidth; got != minLeft {
		t.Fatalf("after many _: LeftWidth = %d, want clamp %d", got, minLeft)
	}
}

func TestQuit(t *testing.T) {
	tm := startTest(t)
	tm.Send(escKey)
	tm.Type("q")
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

func TestQuitKeyTypesInsideEditor(t *testing.T) {
	tm := startTest(t)
	tm.Type("q") // the pattern editor has focus: q is text, not quit
	m := finish(t, tm)
	if m.StateForTest().Focus != PPattern {
		t.Fatal("focus moved")
	}
}

func TestTooSmall(t *testing.T) {
	m := New(testDeps(t)).(*Model)
	m.Update(tea.WindowSizeMsg{Width: 79, Height: 30})
	out := screen(m)
	if !strings.Contains(out, "terminal too small (need 80x24)") {
		t.Fatalf("got %q", out)
	}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	out = screen(m)
	if strings.Contains(out, "too small") || !strings.Contains(out, "[7]─") {
		t.Fatalf("80x24 should render the shell:\n%s", out)
	}
	for i, l := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(l); w != 80 {
			t.Errorf("line %d is %d wide", i, w)
		}
	}
}

func TestBoxWideRunes(t *testing.T) {
	th := newTheme(true, nil)
	body := "日本語のテキスト\n😀😀😀😀😀😀😀😀😀😀😀😀 emoji\nplain"
	out := ansi.Strip(renderBox(th, boxSpec{
		title: "[6]─Test String", counter: "3 lines", body: body, w: 17, h: 5, border: "border",
	}))
	lines := strings.Split(out, "\n")
	if len(lines) != 5 {
		t.Fatalf("got %d lines", len(lines))
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w != 17 {
			t.Errorf("line %d width %d: %q", i, w, l)
		}
	}
}

func TestLightTheme(t *testing.T) {
	dark, light := newTheme(true, nil), newTheme(false, nil)
	if dark.color("focus") == light.color("focus") {
		t.Fatal("light theme should use different colours")
	}
	over := newTheme(true, map[string]string{"focus": "#ff0000"})
	r, g, b, _ := over.color("focus").RGBA()
	if r>>8 != 0xff || g != 0 || b != 0 {
		t.Fatalf("override not applied: %v", over.color("focus"))
	}
}

func TestBackgroundColorSwitchesTheme(t *testing.T) {
	m := New(testDeps(t)).(*Model)
	if !m.theme.dark {
		t.Fatal("default should be dark")
	}
	m.Update(tea.BackgroundColorMsg{Color: whiteColor{}})
	if m.theme.dark {
		t.Fatal("white background should select the light theme")
	}
}

type whiteColor struct{}

func (whiteColor) RGBA() (r, g, b, a uint32) { return 0xffff, 0xffff, 0xffff, 0xffff }

func TestNoForbiddenBindings(t *testing.T) {
	forbidden := []string{"ctrl+m", "ctrl+i", "ctrl+[", "ctrl+s", "ctrl+q"}
	for _, b := range globalBindings {
		for _, k := range b.keys {
			for _, f := range forbidden {
				if k == f {
					t.Errorf("global binding %q uses forbidden key %q", b.help.Desc, k)
				}
			}
		}
	}
}

// TestScreen160x48 compares the full ANSI-stripped screen with
// testdata/TestScreen160x48.golden (go test -run Screen -update rewrites it).
func TestScreen160x48(t *testing.T) {
	m := New(testDeps(t)).(*Model)
	m.Update(tea.WindowSizeMsg{Width: testW, Height: testH})
	golden.RequireEqual(t, []byte(screen(m)))
}
