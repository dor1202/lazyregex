package ui

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dor1202/lazyregex/internal/config"
	"github.com/dor1202/lazyregex/internal/engine"
	"github.com/dor1202/lazyregex/internal/engine/goengine"
	"github.com/dor1202/lazyregex/internal/explain"
	"github.com/dor1202/lazyregex/internal/runner"
)

// mockupPattern is the log-line pattern of the approved mockup.
const mockupPattern = `^(?P<date>\d{4}-\d{2}-\d{2}) (?P<level>INFO|WARN|ERROR)\s+(?P<msg>.*)$`

// lookbehind fails in Go; hints rewrites it to a capture.
const (
	lookbehind        = `(?<=\$)\d+(\.\d{2})?`
	lookbehindRewrite = `\$(\d+(\.\d{2})?)`
)

// clipboardText returns the text of a tea.SetClipboard command.
func clipboardText(t *testing.T, cmd tea.Cmd) string {
	t.Helper()
	if cmd == nil {
		t.Fatal("no command")
	}
	v := reflect.ValueOf(cmd())
	if v.Kind() != reflect.String || !strings.Contains(v.Type().Name(), "lipboard") {
		t.Fatalf("not a clipboard message: %T", cmd())
	}
	return v.String()
}

// detailsTab focuses [7] (leaving the pattern editor) and selects tab i.
func detailsTab(tm interface{ Send(tea.Msg) }, i int) {
	tm.Send(escKey)
	tm.Send(key('7'))
	for range i {
		tm.Send(key(']'))
	}
}

func TestExplanationSelectUnderlines(t *testing.T) {
	r := setup(t, nil, time.Second, nil, nil)
	tm := run(t, depsFor(r, engine.Python, mockupPattern, "2026-10-20 INFO  x"))
	waitFor(t, tm, "python ready", ready(engine.Python))

	rows := explain.Explain(mockupPattern, engine.Python, "g")
	level := -1
	for i, r := range rows {
		if r.Token == "(?P<level>…)" {
			level = i
		}
	}
	if level < 0 || rows[level].Span != [2]int{29, 55} {
		t.Fatalf("no (?P<level>…) row spanning [29,55): %+v", rows)
	}

	detailsTab(tm, tabExplanation)
	for range level {
		tm.Send(downKey)
	}
	s := waitFor(t, tm, "the level row selected", func(s snapshot) bool { return s.state.ExplainSel == level })
	if !strings.Contains(s.screen, "named group \"level\"") {
		t.Fatalf("explanation not shown:\n%s", s.screen)
	}
	th := newTheme(true, nil)
	under := func(tok, text string) string { return th.paintStyle(paint{fg: tok, underline: true}).Render(text) }
	// Bytes [29,55) are `(?P<level>INFO|WARN|ERROR)`: each run keeps its
	// syntax colour and gains an underline.
	for _, want := range []string{under("paren", "(?P<"), under(groupToken(2), "level"), under("text", "INFO"), under("quant", "|"), under("text", "ERROR"), under("paren", ")")} {
		if !strings.Contains(s.raw, want) {
			t.Errorf("[5] lacks underlined %q", ansi.Strip(want))
		}
	}
	for _, not := range []string{under(groupToken(1), "date"), under("escape", `\s`)} {
		if strings.Contains(s.raw, not) {
			t.Errorf("%q outside the span is underlined", ansi.Strip(not))
		}
	}
	if got := explainSpan(&s.state); got != [2]int{29, 55} {
		t.Errorf("explainSpan = %v", got)
	}

	// Leaving [7] removes the underline.
	tm.Send(key('1'))
	s = waitFor(t, tm, "focus on [1]", func(s snapshot) bool { return s.state.Focus == PFlavor })
	if strings.Contains(s.raw, under("text", "INFO")) {
		t.Error("underline kept after leaving [7]")
	}
	done(t, tm)
}

func TestExplanationUnknownRowsDim(t *testing.T) {
	r := setup(t, nil, time.Second, nil, map[engine.Flavor]runner.Factory{engine.Go: goFactory})
	tm := run(t, depsFor(r, engine.Go, `a(?<=b)c`, "abc"))
	waitFor(t, tm, "go ready", ready(engine.Go))
	detailsTab(tm, tabExplanation)
	s := waitFor(t, tm, "the explanation", contains(`"c" literally`))
	var unknown explain.Row
	for _, r := range explain.Explain(`a(?<=b)c`, engine.Go, "g") {
		if r.Unknown {
			unknown = r
		}
	}
	if !unknown.Unknown {
		t.Fatal("expected an Unknown row for lookbehind in Go")
	}
	th := newTheme(true, nil)
	_, lines := explanation(&State{Pattern: `a(?<=b)c`, Flavor: engine.Go, Runner: r, Flags: map[engine.Flavor]string{}})
	var line string
	for _, l := range lines {
		if strings.HasPrefix(l, unknown.Token) {
			line = l
		}
	}
	if line == "" || !strings.Contains(s.raw, th.fg("dim").Render(line)) {
		t.Errorf("unknown row %q not dim", line)
	}
	done(t, tm)
}

func TestCompareApplyRewrite(t *testing.T) {
	_, goErr := goengine.New().Eval(context.Background(), engine.Request{Pattern: lookbehind, Flags: "g"})
	r := setup(t, nil, time.Second, nil, map[engine.Flavor]runner.Factory{engine.Go: goFactory})
	tm := run(t, depsFor(r, engine.Go, lookbehind, "price $12.50"))
	waitFor(t, tm, "the Go error", contains(goErr.Error()))

	detailsTab(tm, tabCompare)
	s := waitFor(t, tm, "the hint", contains("Why: Go does not support lookbehind"))
	for _, want := range []string{"FLAVOR", "RESULT", "MATCHES", "✗ error", "Try: " + lookbehindRewrite, "read group 1"} {
		if !strings.Contains(s.screen, want) {
			t.Errorf("Compare lacks %q:\n%s", want, s.screen)
		}
	}
	if s.state.Pattern != lookbehind {
		t.Fatal("pattern changed before a")
	}
	tm.Send(key('a'))
	s = waitFor(t, tm, "the rewrite applied", func(s snapshot) bool { return s.state.Pattern == lookbehindRewrite })
	waitFor(t, tm, "the rewrite evaluated", contains("✓ 1 match"))
	if s.state.Flavor != engine.Go {
		t.Errorf("flavor changed to %v", s.state.Flavor)
	}
	// The rewrite is one undoable edit in [5].
	send(tm, key('5'), ctrlZ)
	waitFor(t, tm, "the original pattern back", func(s snapshot) bool { return s.state.Pattern == lookbehind })
	send(tm, ctrlY)
	waitFor(t, tm, "the rewrite redone", func(s snapshot) bool { return s.state.Pattern == lookbehindRewrite })
	done(t, tm)
}

var (
	ctrlZ = tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl}
	ctrlY = tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl}
)

func TestLibraryLoadIsUndoable(t *testing.T) {
	r := setup(t, nil, time.Second, nil, nil)
	d := depsFor(r, engine.Python, "mine", "my big pasted text")
	d.Library = config.Library{Entries: []config.Entry{{Name: "e", Flavor: engine.JS, Flags: "g", Pattern: "entry", Text: "entry text"}}}
	tm := run(t, d)
	waitFor(t, tm, "python ready", ready(engine.Python))
	send(tm, escKey, key('4'), enterKey)
	waitFor(t, tm, "the entry loaded", func(s snapshot) bool { return s.state.Text == "entry text" && s.state.Pattern == "entry" })
	send(tm, key('6'), ctrlZ)
	waitFor(t, tm, "the text back", func(s snapshot) bool { return s.state.Text == "my big pasted text" })
	send(tm, escKey, key('5'), ctrlZ)
	s := waitFor(t, tm, "the pattern back", func(s snapshot) bool { return s.state.Pattern == "mine" })
	if !strings.Contains(s.screen, "1 my big pasted text") {
		t.Errorf("[6] does not show the restored text:\n%s", s.screen)
	}
	done(t, tm)
}

func TestLibraryUnknownFlavorRefused(t *testing.T) {
	for _, flavor := range []engine.Flavor{"ruby", ""} {
		r := setup(t, nil, time.Second, nil, nil)
		d := depsFor(r, engine.Python, "mine", "text")
		d.Library = config.Library{Entries: []config.Entry{{Name: "bad", Flavor: flavor, Pattern: "x", Text: "y"}}}
		tm := run(t, d)
		waitFor(t, tm, "python ready", ready(engine.Python))
		want := `library entry "bad": unknown flavor "` + string(flavor) + `"`
		send(tm, escKey, key('4'), enterKey)
		s := waitFor(t, tm, "the notice", contains(want))
		if s.state.Flavor != engine.Python || s.state.Pattern != "mine" || s.state.Text != "text" {
			t.Errorf("%q: state changed: %v %q %q", flavor, s.state.Flavor, s.state.Pattern, s.state.Text)
		}
		if _, err := runCommand("open bad", &s.state); err == nil || err.Error() != want {
			t.Errorf(":open bad: %v", err)
		}
		done(t, tm)
	}
}

func TestCommonPrefixRunes(t *testing.T) {
	if got := commonPrefix([]string{"日本", "日語"}); got != "日" {
		t.Errorf("commonPrefix = %q", got)
	}
}

func TestCompareManualText(t *testing.T) {
	r := setup(t, nil, time.Second, nil, nil)
	d := depsFor(r, engine.Python, "a", "")
	d.Initial.TextPrepared = runner.Prepared{Text: "abc", Manual: true}
	tm := run(t, d)
	waitFor(t, tm, "python ready", ready(engine.Python))
	detailsTab(tm, tabCompare)
	waitFor(t, tm, "the manual note", contains("  js     ✓ compiles   text too large to compare"))
	done(t, tm)
}

func TestCompareRowsAndEnter(t *testing.T) {
	r := setup(t, nil, time.Second, map[engine.Flavor]evalFunc{
		engine.Python: matchAll(`\d+`), engine.JS: matchAll(`\d+`), engine.PCRE: matchAll(`\d+`),
	}, map[engine.Flavor]runner.Factory{engine.Go: goFactory})
	tm := run(t, depsFor(r, engine.Python, `(?<=x)\d+`, "x1 x22 x333 x4444"))
	waitFor(t, tm, "python result", contains("✓ 4 matches"))
	detailsTab(tm, tabCompare)
	s := waitFor(t, tm, "every flavor compared", func(s snapshot) bool {
		return strings.Contains(s.screen, `  js     ✓ compiles   4: "1", "22", "333", …`) &&
			strings.Contains(s.screen, "  go     ✗ error")
	})
	if !strings.Contains(s.screen, `● python ✓ compiles   4: "1", "22", "333", …`) {
		t.Errorf("current flavor row:\n%s", s.screen)
	}
	if strings.Contains(s.screen, "Why:") {
		t.Error("a hint is shown although the current flavor compiles")
	}
	tm.Send(downKey) // the cursor starts on python
	tm.Send(enterKey)
	s = waitFor(t, tm, "flavor js", func(s snapshot) bool { return s.state.Flavor == engine.JS })
	if s.state.Pattern != `(?<=x)\d+` {
		t.Errorf("enter changed the pattern to %q", s.state.Pattern)
	}
	done(t, tm)
}

func TestCompareOnlyWhenVisible(t *testing.T) {
	rec := &recorder{}
	r := setup(t, rec, time.Second, nil, nil)
	tm := run(t, depsFor(r, engine.Python, "a", "abc"))
	waitFor(t, tm, "python ready", ready(engine.Python))
	detailsTab(tm, tabExplanation)
	tm.Send(key('2'))
	tm.Send(downKey)
	tm.Send(spaceKey) // an input change re-runs EvalAll
	// The other flavors are compile-checked for the [1] markers, but with
	// Explanation showing nothing evaluates them on the text.
	eventually(t, "the marker check", func() bool { return len(rec.of(engine.JS)) > 0 && len(rec.of(engine.Go)) > 0 })
	time.Sleep(2 * allDebounce)
	others := func() (n int) {
		for _, f := range []engine.Flavor{engine.JS, engine.PCRE, engine.Go} {
			for _, c := range rec.of(f) {
				if c.req.Text != "" {
					n++
				}
			}
		}
		return n
	}
	if n := others(); n != 0 {
		t.Fatalf("%d evaluations on the text while Compare was hidden", n)
	}
	detailsTab(tm, tabCompare)
	eventually(t, "Compare's evaluation", func() bool { return others() >= 3 })
	for _, f := range []engine.Flavor{engine.JS, engine.PCRE, engine.Go} {
		c := rec.of(f)
		if last := c[len(c)-1].req; last.Text != "abc" || last.Pattern != "a" {
			t.Errorf("%s: last request %+v", f, last)
		}
	}
	done(t, tm)
}

func TestCodeTabSnippet(t *testing.T) {
	r := setup(t, nil, time.Second, nil, map[engine.Flavor]runner.Factory{engine.Go: goFactory})
	tm := run(t, depsFor(r, engine.Go, `(\d+)`, "a1b22"))
	waitFor(t, tm, "go result", contains("✓ 2 matches"))
	detailsTab(tm, tabCode)
	s := waitFor(t, tm, "the snippet", contains("regexp.MustCompile"))
	if !strings.Contains(s.screen, "`(\\d+)`") || !strings.Contains(s.screen, "`a1b22`") || !strings.Contains(s.screen, "package main") {
		t.Errorf("snippet incomplete:\n%s", s.screen)
	}
	done(t, tm)

	// y copies the raw snippet; long text becomes the placeholder (R17).
	r.Load(engine.Go)
	eventually(t, "go loaded", func() bool { return r.State(engine.Go) == runner.Ready })
	st := &State{Flavor: engine.Go, Flags: map[engine.Flavor]string{}, Runner: r, Tab: map[PanelID]int{PDetails: tabCode},
		Pattern: "a\u202eb", Text: "x", Theme: newTheme(true, nil)}
	p := newDetailsPanel()
	got := clipboardText(t, p.Update(key('y'), st))
	if !strings.Contains(got, "a\u202eb") || !strings.Contains(got, "func main()") {
		t.Errorf("clipboard is not the raw snippet: %q", got)
	}
	if view := p.View(st, 100, 40); strings.Contains(view, "\u202e") {
		t.Error("U+202E reaches the screen")
	}
	st.Text = strings.Repeat("y", snippetTextLimit+1)
	got = clipboardText(t, p.Update(key('y'), st))
	if strings.Contains(got, "yyyy") || !strings.Contains(got, snippetPlaceholder) {
		t.Errorf("long text not replaced by %q", snippetPlaceholder)
	}
	st.Text = strings.Repeat("y", snippetTextLimit)
	if got = clipboardText(t, p.Update(key('y'), st)); !strings.Contains(got, st.Text) {
		t.Error("text at the limit should be kept")
	}
}

func TestLibrarySaveOpen(t *testing.T) {
	dir := t.TempDir()
	r := setup(t, nil, time.Second, nil, nil)
	d := depsFor(r, engine.Python, `\d+`, "a1")
	d.ConfigDir = dir
	tm := run(t, d)
	waitFor(t, tm, "python ready", ready(engine.Python))

	// n asks for a name in the prompt.
	send(tm, escKey, key('4'), key('n'))
	waitFor(t, tm, "the save prompt", contains(":save "))
	tm.Type("digits")
	tm.Send(enterKey)
	waitFor(t, tm, "the notice", contains(`saved "digits"`))
	lib, err := config.LoadLibrary(dir)
	if err != nil || len(lib.Entries) != 1 {
		t.Fatalf("library on disk: %+v, %v", lib, err)
	}
	if e := lib.Entries[0]; e.Name != "digits" || e.Flavor != engine.Python || e.Pattern != `\d+` || e.Text != "a1" || e.Flags != "g" {
		t.Fatalf("entry = %+v", e)
	}

	// :save works too and overwrites by name.
	send(tm, key(':'))
	tm.Type("flavor js")
	send(tm, enterKey, key(':'))
	tm.Type("save words")
	tm.Send(enterKey)
	waitFor(t, tm, "two entries", func(s snapshot) bool { return len(s.state.Library.Entries) == 2 })

	// Change everything, then load "digits" with enter.
	send(tm, key(':'))
	tm.Type("mode replace")
	tm.Send(enterKey)
	send(tm, key('5'))
	tm.Type("zz")
	send(tm, escKey, key('4'), key('k'), enterKey)
	s := waitFor(t, tm, "digits loaded", func(s snapshot) bool { return s.state.Pattern == `\d+` })
	if s.state.Flavor != engine.Python || s.state.Text != "a1" || s.state.Flags[engine.Python] != "g" {
		t.Fatalf("loaded state: flavor %v text %q flags %q", s.state.Flavor, s.state.Text, s.state.Flags[engine.Python])
	}
	waitFor(t, tm, "the loaded pattern evaluated", contains(`r"\d+"`))

	// :open loads by name.
	send(tm, key(':'))
	tm.Type("open words")
	tm.Send(enterKey)
	waitFor(t, tm, "words loaded", func(s snapshot) bool { return s.state.Flavor == engine.JS })

	// d asks y/n; n keeps the entry, y deletes it.
	send(tm, key('j'), key('d'))
	waitFor(t, tm, "the question", contains(`delete "words"? (y/n)`))
	tm.Send(key('n'))
	waitFor(t, tm, "the question gone", func(s snapshot) bool { return !strings.Contains(s.screen, "(y/n)") })
	tm.Send(key('d'))
	waitFor(t, tm, "the question again", contains("(y/n)"))
	tm.Send(key('y'))
	waitFor(t, tm, "deleted", contains(`deleted "words"`))
	lib, _ = config.LoadLibrary(dir)
	if len(lib.Entries) != 1 || lib.Entries[0].Name != "digits" {
		t.Fatalf("library on disk after delete: %+v", lib)
	}
	done(t, tm)
}

func TestPromptCommands(t *testing.T) {
	dir := t.TempDir()
	newState := func() *State {
		m := New(Deps{Config: config.Defaults(), ConfigDir: dir, Library: config.Library{Entries: []config.Entry{
			{Name: "kept", Flavor: engine.PCRE, Flags: "gi", Pattern: "k+", Replacement: "K", Text: "kk"},
		}}}).(*Model)
		return &m.state
	}
	for _, tc := range []struct {
		line  string
		err   string
		check func(*State) bool
	}{
		{"flavor go", "", func(s *State) bool { return s.Flavor == engine.Go }},
		{"  flavor   js  ", "", func(s *State) bool { return s.Flavor == engine.JS }},
		{"flavor perl", "unknown flavor: perl", nil},
		{"flags gim", "", func(s *State) bool { return s.Flags[engine.Python] == "gim" }},
		{"mode replace", "", func(s *State) bool { return s.Mode == engine.ModeReplace && s.Tab[PPattern] == 1 }},
		{"mode split", "", func(s *State) bool { return s.Mode == engine.ModeSplit && s.Tab[PPattern] == 2 }},
		{"mode fancy", "unknown mode: fancy", nil},
		{"mouse off", "", func(s *State) bool { return !s.Mouse }},
		{"mouse on", "", func(s *State) bool { return s.Mouse }},
		{"mouse", "", func(s *State) bool { return !s.Mouse }}, // default on: toggles off
		{"mouse maybe", "usage: mouse [on|off]", nil},
		{"save", "usage: save <name>", nil},
		{"save my pattern", "", func(s *State) bool { return s.Library.Entries[1].Name == "my pattern" }},
		{"open kept", "", func(s *State) bool {
			return s.Flavor == engine.PCRE && s.Flags[engine.PCRE] == "gi" && s.Pattern == "k+" && s.Replacement == "K" && s.Text == "kk"
		}},
		{"open nope", `no saved pattern named "nope"`, nil},
		{"bogus arg", "unknown command: bogus", nil},
		{"", "", func(*State) bool { return true }},
	} {
		s := newState()
		_, err := runCommand(tc.line, s)
		switch {
		case tc.err != "":
			if err == nil || err.Error() != tc.err {
				t.Errorf("%q: error %v, want %q", tc.line, err, tc.err)
			}
		case err != nil:
			t.Errorf("%q: %v", tc.line, err)
		case !tc.check(s):
			t.Errorf("%q: state not as expected", tc.line)
		}
	}
	cmd, err := runCommand("q", newState())
	if err != nil || cmd == nil {
		t.Fatalf("q: %v", err)
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("q does not quit")
	}
	if _, err := os.Stat(filepath.Join(dir, "library.yaml")); err != nil {
		t.Errorf(":save did not write the library: %v", err)
	}
}

func TestPromptCompletionAndErrors(t *testing.T) {
	r := setup(t, nil, time.Second, nil, nil)
	tm := run(t, depsFor(r, engine.Python, "a", "a"))
	waitFor(t, tm, "python ready", ready(engine.Python))
	send(tm, escKey, key(':'), key('f'), key('l'), tabKey)
	s := waitFor(t, tm, "the choices", contains("flavor  flags"))
	if !strings.Contains(lastLine(s.screen), ":fla") {
		t.Errorf("not extended to the common prefix: %q", lastLine(s.screen))
	}
	send(tm, key('v'), tabKey) // "flav" -> "flavor "
	send(tm, key('g'), tabKey) // flavor names: "g" -> "go"
	waitFor(t, tm, "completed", func(s snapshot) bool { return strings.Contains(lastLine(s.screen), ":flavor go") })
	tm.Send(enterKey)
	waitFor(t, tm, "flavor go", func(s snapshot) bool { return s.state.Flavor == engine.Go })

	send(tm, key(':'))
	tm.Type("frobnicate")
	tm.Send(enterKey)
	s = waitFor(t, tm, "the error", contains("unknown command: frobnicate"))
	if !strings.Contains(s.raw, newTheme(true, nil).fg("error").Render("unknown command: frobnicate")) {
		t.Error("error not red")
	}
	// The next key clears the notice; esc closes the prompt unrun.
	send(tm, key(':'), key('q'), escKey)
	s = waitFor(t, tm, "the key bar", contains("Keybindings: ?"))
	if s.state.Pattern != "a" {
		t.Error("prompt keys leaked into the pattern")
	}
	done(t, tm)
}

func TestHelpFilter(t *testing.T) {
	r := setup(t, nil, time.Second, nil, nil)
	tm := run(t, depsFor(r, engine.Python, "a", "a"))
	waitFor(t, tm, "python ready", ready(engine.Python))
	send(tm, escKey, key('3'), key('?'))
	s := waitFor(t, tm, "the popup", contains("╭─Keybindings"))
	l, g, mo := strings.Index(s.screen, "Local · Matches"), strings.Index(s.screen, "Global"), strings.Index(s.screen, "Mouse (when on)")
	if l < 0 || g < l || mo < g {
		t.Fatalf("sections missing or out of order (%d %d %d):\n%s", l, g, mo, s.screen)
	}
	for _, want := range []string{"Copy value", ":save <name>", "/ ▏ type to filter", "1 of "} {
		if !strings.Contains(s.screen, want) {
			t.Errorf("popup lacks %q", want)
		}
	}
	send(tm, key('/'))
	tm.Type("copy")
	s = waitFor(t, tm, "the filter", contains("/ copy"))
	for _, want := range []string{"Copy value", "Copy all as JSON", "1 of 2", "Local · Matches"} {
		if !strings.Contains(s.screen, want) {
			t.Errorf("filtered popup lacks %q", want)
		}
	}
	for _, not := range []string{"Next pane", "Global", "Mouse (when on)"} {
		if strings.Contains(s.screen, not) {
			t.Errorf("filtered popup still shows %q", not)
		}
	}
	tm.Send(escKey)
	s = waitFor(t, tm, "the popup closed", func(s snapshot) bool { return !strings.Contains(s.screen, "╭─Keybindings") })
	if s.state.Focus != PMatches {
		t.Error("focus changed")
	}

	// In Library the local section lists n.
	send(tm, key('4'), key('?'))
	s = waitFor(t, tm, "the library popup", contains("Local · Library"))
	if !strings.Contains(s.screen, "Save current") {
		t.Errorf("Library help lacks n:\n%s", s.screen)
	}
	done(t, tm)
}

// forbidden are the keys that must never be bound (Global Constraints).
var forbidden = []tea.KeyPressMsg{
	{Code: 'm', Mod: tea.ModCtrl}, {Code: 'i', Mod: tea.ModCtrl}, {Code: '[', Mod: tea.ModCtrl},
	{Code: 's', Mod: tea.ModCtrl}, {Code: 'q', Mod: tea.ModCtrl},
}

func TestNoBindingOnForbiddenCtrlKeys(t *testing.T) {
	names := map[string]bool{}
	for _, k := range forbidden {
		names[k.String()] = true
	}
	if len(names) != 5 || !names["ctrl+s"] || !names["ctrl+["] {
		t.Fatalf("forbidden key names: %v", names)
	}
	check := func(where string, keys []KeyHelp) {
		for _, k := range keys {
			for _, part := range strings.Fields(strings.ReplaceAll(k.Key, "/", " ")) {
				if names[part] {
					t.Errorf("%s binds %q (%s)", where, part, k.Desc)
				}
			}
		}
	}
	// The declared keymap: global keys, every panel in every tab and
	// editing state, the prompt and the help popup.
	for _, b := range globalBindings {
		for _, k := range b.keys {
			if names[k] {
				t.Errorf("global binding %q uses %q", b.help.Desc, k)
			}
		}
	}
	m := New(Deps{Config: config.Defaults(), Library: config.Library{Entries: []config.Entry{{Name: "x"}}}}).(*Model)
	m.Update(tea.WindowSizeMsg{Width: testW, Height: testH})
	for id, p := range m.panels {
		for tab := range p.Tabs() {
			for _, editing := range []bool{false, true} {
				m.state.Focus, m.state.Tab[id], m.state.Editing = id, tab, editing
				check(id.String(), p.Keys(&m.state))
			}
		}
	}
	check("prompt", promptKeys)
	check("confirm", confirmKeys)
	check("help", helpKeys)
	check("mouse", mouseHelp)

	// And by behaviour: in every context a forbidden key changes nothing.
	type ctx struct {
		name  string
		setup func(m *Model)
	}
	var ctxs []ctx
	for id := PanelID(1); id <= panelCount; id++ {
		for tab := range m.panels[id].Tabs() {
			ctxs = append(ctxs, ctx{id.String(), func(m *Model) {
				m.focus(id)
				m.state.Tab[id] = tab
			}})
		}
		if m.panels[id].IsEditor() {
			ctxs = append(ctxs, ctx{id.String() + " not editing", func(m *Model) { m.focus(id); m.state.Editing = false }})
		}
	}
	ctxs = append(ctxs,
		ctx{"prompt", func(m *Model) { m.prompt = newPrompt("fl") }},
		ctx{"confirm", func(m *Model) {
			m.prompt = &prompt{question: "delete?", yes: func(*State) (string, error) { t.Error("confirmed"); return "", nil }}
		}},
		ctx{"help", func(m *Model) { m.help = m.newHelp() }},
		ctx{"help filter", func(m *Model) { m.help = m.newHelp(); m.help.filtering = true }},
	)
	for _, c := range ctxs {
		m := New(Deps{Config: config.Defaults(), Initial: InitialInput{Pattern: "a(b)", Text: "ab"},
			Library: config.Library{Entries: []config.Entry{{Name: "x", Pattern: "x"}}}}).(*Model)
		m.Update(tea.WindowSizeMsg{Width: testW, Height: testH})
		c.setup(m)
		before, st := screen(m), m.state
		for _, k := range forbidden {
			if cmd := m.update(k); cmd != nil {
				t.Errorf("%s: %s returned a command", c.name, k)
			}
		}
		after := m.state
		if screen(m) != before || after.Pattern != st.Pattern || after.Text != st.Text || after.Flavor != st.Flavor ||
			after.Focus != st.Focus || after.Editing != st.Editing || after.Mode != st.Mode || len(after.Library.Entries) != 1 {
			t.Errorf("%s: a forbidden key changed something", c.name)
		}
	}
}

func TestDetailsTextSanitized(t *testing.T) {
	const evil = "\u009b\u202e\u2066\x1b[31m"
	bad := []string{"\u009b", "\u202e", "\u2066", "\x1b[31m"}
	clean := func(what, raw string) {
		t.Helper()
		plain := ansi.Strip(raw)
		for _, b := range bad {
			if strings.Contains(plain, b) {
				t.Errorf("%s: %q reaches the terminal", what, b)
			}
		}
	}
	pattern := "(?<=" + evil + ")x" // Go rejects it and quotes the pattern in its error
	r := setup(t, nil, time.Second, map[engine.Flavor]evalFunc{
		engine.Python: func(context.Context, engine.Request) (engine.Result, error) {
			return engine.Result{}, &engine.CompileError{Message: "bad " + evil, Offset: -1}
		},
		engine.JS: func(_ context.Context, req engine.Request) (engine.Result, error) {
			return engine.Result{Unit: engine.UnitBytes, Matches: []engine.Match{{Span: engine.Span{Start: 0, End: len(req.Text)}}}}, nil
		},
	}, map[engine.Flavor]runner.Factory{engine.Go: goFactory})
	d := depsFor(r, engine.Go, pattern, evil+"x")
	d.Library = config.Library{Entries: []config.Entry{{Name: "lib" + evil, Flavor: engine.Go}}}
	tm := run(t, d)
	waitFor(t, tm, "go error", contains("✗ error"))

	detailsTab(tm, tabExplanation)
	clean("explanation", waitFor(t, tm, "explanation", contains("U+202E literally")).raw)
	tm.Send(key(']'))
	code := waitFor(t, tm, "code", contains("regexp.MustCompile"))
	clean("code", code.raw)
	if !strings.Contains(code.raw, "\x1b[38;2;") || !strings.Contains(code.screen, "␛[31m") {
		t.Error("code: the snippet should be highlighted with ESC drawn as ␛")
	}
	tm.Send(key(']'))
	s := waitFor(t, tm, "compare", func(s snapshot) bool {
		return strings.Contains(s.screen, "Why:") && strings.Contains(s.screen, "bad ") && strings.Contains(s.screen, "  js     ✓")
	})
	clean("compare", s.raw)
	clean("library", s.raw)

	tm.Send(key(':'))
	tm.Send(tea.PasteMsg{Content: "open " + evil})
	clean("prompt echo", waitFor(t, tm, "prompt", contains(":open ")).raw)
	tm.Send(enterKey)
	clean("notice", waitFor(t, tm, "notice", contains("no saved pattern named")).raw)
	send(tm, key('?'), key('/'))
	tm.Send(tea.PasteMsg{Content: evil})
	clean("help filter", waitFor(t, tm, "filter", contains("no keys match")).raw)
	done(t, tm)
}
