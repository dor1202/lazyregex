package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/dor1202/lazyregex/internal/config"
	"github.com/dor1202/lazyregex/internal/engine"
	"github.com/dor1202/lazyregex/internal/engine/goengine"
	"github.com/dor1202/lazyregex/internal/runner"
)

// --- fakes -----------------------------------------------------------------

// call is one evaluation seen by a scripted engine.
type call struct {
	flavor   engine.Flavor
	req      engine.Request
	at       time.Time
	deadline time.Time
}

// recorder collects the calls of every scripted engine.
type recorder struct {
	mu    sync.Mutex
	calls []call
}

func (r *recorder) add(c call) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, c)
}

// of returns the calls made to flavor f.
func (r *recorder) of(f engine.Flavor) []call {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []call
	for _, c := range r.calls {
		if c.flavor == f {
			out = append(out, c)
		}
	}
	return out
}

type evalFunc func(ctx context.Context, req engine.Request) (engine.Result, error)

// scripted is a fake engine whose Eval is fn, with flags g (default), i, m.
type scripted struct {
	f   engine.Flavor
	rec *recorder
	fn  evalFunc
}

func (e scripted) Info() engine.Info {
	return engine.Info{Flavor: e.f, EngineName: "fake " + string(e.f), Version: "1.0", Flags: []engine.FlagInfo{
		{Letter: 'g', Name: "global", Default: true},
		{Letter: 'i', Name: "ignore case"},
		{Letter: 'm', Name: "multiline"},
	}}
}
func (e scripted) Close() error { return nil }
func (e scripted) Eval(ctx context.Context, req engine.Request) (engine.Result, error) {
	d, _ := ctx.Deadline()
	if e.rec != nil {
		e.rec.add(call{flavor: e.f, req: req, at: time.Now(), deadline: d})
	}
	if e.fn == nil {
		return engine.Result{}, ctx.Err()
	}
	return e.fn(ctx, req)
}

func scriptedFactory(f engine.Flavor, rec *recorder, fn evalFunc) runner.Factory {
	return func(context.Context, string) (engine.Engine, error) { return scripted{f, rec, fn}, nil }
}

func goFactory(context.Context, string) (engine.Engine, error) { return goengine.New(), nil }

// setup builds a runner: every flavor scripted with fns[f] (nil = empty
// results), except those in over.
func setup(t *testing.T, rec *recorder, timeout time.Duration, fns map[engine.Flavor]evalFunc, over map[engine.Flavor]runner.Factory) *runner.Runner {
	t.Helper()
	m := map[engine.Flavor]runner.Factory{}
	for _, f := range engine.Flavors {
		m[f] = scriptedFactory(f, rec, fns[f])
		if fac, ok := over[f]; ok {
			m[f] = fac
		}
	}
	r := runner.New(m, t.TempDir(), timeout)
	t.Cleanup(r.Close)
	return r
}

func depsFor(r *runner.Runner, f engine.Flavor, pattern, text string) Deps {
	return Deps{
		Runner:  r,
		Config:  config.Defaults(),
		Initial: InitialInput{Flavor: f, Pattern: pattern, Text: text},
	}
}

// matchAll is a scripted Python-like evaluation (code point spans).
func matchAll(re string) evalFunc {
	rx := regexp.MustCompile(re)
	return func(_ context.Context, req engine.Request) (engine.Result, error) {
		res := engine.Result{Unit: engine.UnitCodePoints}
		for _, loc := range rx.FindAllStringIndex(req.Text, -1) {
			res.Matches = append(res.Matches, engine.Match{Span: engine.Span{Start: loc[0], End: loc[1]}})
		}
		return res, nil
	}
}

// --- driving the program ----------------------------------------------------

// probe wraps the model so tests can read the screen and state from inside
// the program's goroutine (no data race).
type probe struct{ *Model }

type snapshot struct {
	screen string // ANSI-stripped
	raw    string
	state  State
}

type probeMsg struct{ reply chan snapshot }

func (p probe) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if pm, ok := msg.(probeMsg); ok {
		st := p.state
		st.Flags = map[engine.Flavor]string{}
		for k, v := range p.state.Flags {
			st.Flags[k] = v
		}
		st.All = nil
		raw := p.View().Content
		pm.reply <- snapshot{screen: ansi.Strip(raw), raw: raw, state: st}
		return p, nil
	}
	_, cmd := p.Model.Update(msg)
	return p, cmd
}

func run(t *testing.T, d Deps) *teatest.TestModel {
	t.Helper()
	return teatest.NewTestModel(t, probe{New(d).(*Model)}, teatest.WithInitialTermSize(testW, testH))
}

// done quits the program and waits for it to finish.
func done(t *testing.T, tm *teatest.TestModel) {
	t.Helper()
	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

// snap reads the current screen and state.
func snap(t *testing.T, tm *teatest.TestModel) snapshot {
	t.Helper()
	ch := make(chan snapshot, 1)
	tm.Send(probeMsg{ch})
	select {
	case s := <-ch:
		return s
	case <-time.After(5 * time.Second):
		t.Fatal("program did not answer the probe")
	}
	return snapshot{}
}

// waitTimeout bounds waitFor and eventually.
var waitTimeout = 5 * time.Second

// waitFor polls until cond holds (waitTimeout at most).
func waitFor(t *testing.T, tm *teatest.TestModel, what string, cond func(snapshot) bool) snapshot {
	t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for {
		s := snap(t, tm)
		if cond(s) {
			return s
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s; screen:\n%s", what, s.screen)
		}
		time.Sleep(15 * time.Millisecond)
	}
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func contains(sub string) func(snapshot) bool {
	return func(s snapshot) bool { return strings.Contains(s.screen, sub) }
}

func ready(f engine.Flavor) func(snapshot) bool {
	return func(s snapshot) bool {
		return strings.Contains(s.screen, "[5]") && !strings.Contains(s.screen, "loading "+string(f)+" engine")
	}
}

var (
	enterKey = tea.KeyPressMsg{Code: tea.KeyEnter}
	downKey  = tea.KeyPressMsg{Code: tea.KeyDown}
	spaceKey = tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
)

func send(tm *teatest.TestModel, keys ...tea.Msg) {
	for _, k := range keys {
		tm.Send(k)
	}
}

// segments splits styled output into its unstyled runs.
func segments(s string) []string {
	var out []string
	for _, p := range regexp.MustCompile(`\x1b\[[0-9;:]*m`).Split(s, -1) {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// --- tests -------------------------------------------------------------------

func TestTypingEvaluatesDebounced(t *testing.T) {
	rec := &recorder{}
	r := setup(t, rec, time.Second, map[engine.Flavor]evalFunc{engine.Python: matchAll(`\d+`)}, nil)
	tm := run(t, depsFor(r, engine.Python, "", "ab 12 c 345"))
	waitFor(t, tm, "python ready", ready(engine.Python))

	before := time.Now()
	tm.Type(`\d+`)
	eventually(t, "an evaluation", func() bool { return len(rec.of(engine.Python)) > 0 })
	time.Sleep(150 * time.Millisecond) // well past the debounce, before EvalAll's
	calls := rec.of(engine.Python)
	if len(calls) != 1 {
		t.Fatalf("got %d evaluations, want exactly 1: %+v", len(calls), calls)
	}
	if calls[0].req.Pattern != `\d+` || calls[0].req.Flags != "g" {
		t.Fatalf("request = %+v", calls[0].req)
	}
	if d := calls[0].at.Sub(before); d < evalDebounce {
		t.Fatalf("evaluated %v after typing, before the %v debounce", d, evalDebounce)
	}
	s := waitFor(t, tm, "match count", contains("✓ 2 matches"))
	if len(s.state.Matches) != 2 {
		t.Fatalf("state has %d matches", len(s.state.Matches))
	}
	done(t, tm)
}

func TestSwitchFlavorKeepsPattern(t *testing.T) {
	const pat = `(?<=\$)\d+`
	_, goErr := goengine.New().Eval(context.Background(), engine.Request{Pattern: pat, Flags: "g"})
	if goErr == nil {
		t.Fatal("Go should reject lookbehind")
	}
	r := setup(t, nil, time.Second, map[engine.Flavor]evalFunc{engine.Python: matchAll(`\d+`)},
		map[engine.Flavor]runner.Factory{engine.Go: goFactory})
	tm := run(t, depsFor(r, engine.Python, pat, "$12"))
	waitFor(t, tm, "python result", contains("✓ 1 match"))

	send(tm, escKey, key('1'), key('j'), key('j'), key('j'), enterKey)
	s := waitFor(t, tm, "the Go error", contains(goErr.Error()))
	if s.state.Flavor != engine.Go {
		t.Fatalf("flavor = %v", s.state.Flavor)
	}
	if s.state.Pattern != pat {
		t.Fatalf("pattern changed to %q", s.state.Pattern)
	}
	if !strings.Contains(s.screen, "`"+pat+"`") {
		t.Fatalf("pattern line not shown unchanged:\n%s", s.screen)
	}
	// Go reports the error at byte 0, the "(".
	th := newTheme(true, nil)
	if !strings.Contains(s.raw, th.paintStyle(paint{fg: "error", curly: true}).Render("(")) {
		t.Fatal("error position not underlined")
	}
	done(t, tm)
}

func TestCompileErrorUnderlineUnits(t *testing.T) {
	th := newTheme(true, nil)
	curly := func(s string) string { return th.paintStyle(paint{fg: "error", curly: true}).Render(s) }
	for _, tc := range []struct {
		f       engine.Flavor
		pattern string
		offset  int
		want    string
	}{
		{engine.Python, "日本語(x", 3, "("}, // code points
		{engine.JS, "😀(x", 2, "("},       // UTF-16 units
		{engine.PCRE, "é(x", 2, "("},     // bytes
		{engine.Go, "ü*(x", 3, "("},      // bytes
		{engine.Python, "日本語", 3, " "},   // at the end
	} {
		u := errorUnderline(tc.pattern, tc.f, tc.offset)
		out := th.highlightPattern(tc.pattern, tc.f, "g", u)
		if !strings.Contains(out, curly(tc.want)) {
			t.Errorf("%s %q offset %d: underline %v, output %q", tc.f, tc.pattern, tc.offset, u, out)
		}
	}
	if u := errorUnderline("abc", engine.Python, -1); u[0] >= 0 {
		t.Errorf("offset -1 should not underline: %v", u)
	}

	// End to end: a Python error reported in code points of a non-ASCII pattern.
	const msg = "missing ), unterminated subpattern at position 3"
	r := setup(t, nil, time.Second, map[engine.Flavor]evalFunc{
		engine.Python: func(context.Context, engine.Request) (engine.Result, error) {
			return engine.Result{}, &engine.CompileError{Message: msg, Offset: 3}
		},
	}, nil)
	tm := run(t, depsFor(r, engine.Python, "日本語(x", "text"))
	s := waitFor(t, tm, "the error", contains(msg))
	if !strings.Contains(s.raw, curly("(")) {
		t.Fatal("error position not underlined")
	}
	if !strings.Contains(s.raw, th.fg("error").Render("╭─")) {
		t.Fatal("pattern border should be red")
	}
	done(t, tm)
}

func TestFlagsPerFlavor(t *testing.T) {
	rec := &recorder{}
	r := setup(t, rec, time.Second, nil, nil)
	tm := run(t, depsFor(r, engine.Python, "x", "x"))
	waitFor(t, tm, "python ready", ready(engine.Python))

	send(tm, escKey, key('2'), key('j'), spaceKey) // python: g + i
	waitFor(t, tm, "[x] i", contains("[x] i  ignore case"))
	send(tm, key('1'), key('j'), enterKey) // js
	eventually(t, "a js evaluation", func() bool { return len(rec.of(engine.JS)) > 0 })
	send(tm, key('2'), key('j'), spaceKey) // js: g + m
	eventually(t, "js with m", func() bool {
		c := rec.of(engine.JS)
		return len(c) > 0 && c[len(c)-1].req.Flags == "gm"
	})
	send(tm, key('1'), key('k'), enterKey) // back to python
	s := waitFor(t, tm, "python flags restored", func(s snapshot) bool {
		return s.state.Flavor == engine.Python && strings.Contains(s.screen, "[x] i  ignore case") &&
			strings.Contains(s.screen, "[ ] m  multiline")
	})
	if s.state.Flags[engine.Python] != "gi" || s.state.Flags[engine.JS] != "gm" {
		t.Fatalf("flags = %v", s.state.Flags)
	}
	eventually(t, "python evaluated with gi", func() bool {
		c := rec.of(engine.Python)
		return len(c) > 0 && c[len(c)-1].req.Flags == "gi"
	})
	done(t, tm)
}

func TestReplaceTabShowsResult(t *testing.T) {
	r := setup(t, nil, time.Second, nil, map[engine.Flavor]runner.Factory{engine.Go: goFactory})
	tm := run(t, depsFor(r, engine.Go, `(\d+)`, "a1b22"))
	waitFor(t, tm, "go result", contains("✓ 2 matches"))

	send(tm, escKey, key(']'), enterKey, downKey)
	tm.Type("<$1>")
	s := waitFor(t, tm, "the replace result", contains("a<1>b<22>"))
	if s.state.Mode != engine.ModeReplace || s.state.Replacement != "<$1>" || s.state.Pattern != `(\d+)` {
		t.Fatalf("mode %v replacement %q pattern %q", s.state.Mode, s.state.Replacement, s.state.Pattern)
	}
	for _, want := range []string{"replace <$1>", "Result", "✓ replaced"} {
		if !strings.Contains(s.screen, want) {
			t.Errorf("screen lacks %q", want)
		}
	}

	send(tm, escKey, key(']'), enterKey) // Split
	waitFor(t, tm, "the split parts", func(s snapshot) bool {
		return strings.Contains(s.screen, "✓ 3 parts") && strings.Contains(s.screen, "0 a")
	})
	done(t, tm)
}

func TestUnmatchedGroupRendersNotMatched(t *testing.T) {
	r := setup(t, nil, time.Second, nil, map[engine.Flavor]runner.Factory{engine.Go: goFactory})
	tm := run(t, depsFor(r, engine.Go, `(a)|(?P<bee>b)`, "xb"))
	waitFor(t, tm, "go result", contains("✓ 1 match"))
	send(tm, escKey, key('3'), enterKey)
	s := waitFor(t, tm, "expanded groups", contains("▾ #1"))
	for _, want := range []string{"1–2", "not matched", "bee"} {
		if !strings.Contains(s.screen, want) {
			t.Errorf("screen lacks %q:\n%s", want, s.screen)
		}
	}
	done(t, tm)
}

func TestHighlightUTF16Offsets(t *testing.T) {
	th := newTheme(true, nil)
	res := engine.Result{Unit: engine.UnitUTF16, Matches: []engine.Match{{
		Span:   engine.Span{Start: 1, End: 4}, // "😀b" in UTF-16 units
		Groups: []engine.Group{{Index: 1, Matched: true, Span: engine.Span{Start: 1, End: 3}}},
	}}}
	got := segments(th.highlightText("a😀bc", res, -1))
	want := []string{"a", "😀", "b", "c"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("segments = %q, want %q", got, want)
	}
	out := th.highlightText("a😀bc", res, -1)
	if !strings.Contains(out, th.paintStyle(paint{fg: "group1.fg", bg: "group1.bg"}).Render("😀")) {
		t.Error("group 1 not in its colours")
	}
	if !strings.Contains(out, th.paintStyle(paint{fg: "text", bg: "match"}).Render("b")) {
		t.Error("match remainder not in the match colour")
	}

	// End to end through a fake JS engine reporting UTF-16 spans.
	r := setup(t, nil, time.Second, map[engine.Flavor]evalFunc{
		engine.JS: func(context.Context, engine.Request) (engine.Result, error) {
			return engine.Result{Unit: engine.UnitUTF16, Matches: []engine.Match{{Span: engine.Span{Start: 1, End: 3}}}}, nil
		},
	}, nil)
	tm := run(t, depsFor(r, engine.JS, "😀", "x😀y"))
	s := waitFor(t, tm, "js result", contains("✓ 1 match"))
	sel := th.paintStyle(paint{fg: "text", bg: "match", underline: true}).Render("😀")
	if !strings.Contains(s.raw, sel) {
		t.Fatal("the emoji is not highlighted as the selected match")
	}
	if !strings.Contains(s.screen, "#1   1–3") {
		t.Errorf("matches panel should show the native UTF-16 span 1–3:\n%s", s.screen)
	}
	done(t, tm)
}

func TestManualModeNeedsEnter(t *testing.T) {
	rec := &recorder{}
	r := setup(t, rec, time.Second, map[engine.Flavor]evalFunc{engine.Python: matchAll(`\d+`)}, nil)
	d := depsFor(r, engine.Python, "", "")
	d.Initial.TextPrepared = runner.Prepared{Text: "abc 123", Manual: true}
	tm := run(t, d)
	waitFor(t, tm, "python ready", ready(engine.Python))
	tm.Type(`\d+`)
	waitFor(t, tm, "the notice", contains("large input: press enter to evaluate"))
	time.Sleep(200 * time.Millisecond)
	if n := len(rec.of(engine.Python)); n != 0 {
		t.Fatalf("%d evaluations before enter", n)
	}
	tm.Send(enterKey)
	s := waitFor(t, tm, "the result", contains("✓ 1 match"))
	calls := rec.of(engine.Python)
	if len(calls) != 1 {
		t.Fatalf("%d evaluations, want 1", len(calls))
	}
	if left := calls[0].deadline.Sub(calls[0].at); left < 20*time.Second {
		t.Fatalf("manual evaluation deadline %v, want the 30s manual timeout", left)
	}
	if strings.Contains(s.screen, "press enter") || s.state.Pattern != `\d+` {
		t.Fatalf("notice still shown or pattern changed:\n%s", s.screen)
	}
	done(t, tm)
}

func TestInvalidUTF8Warning(t *testing.T) {
	r := setup(t, nil, time.Second, nil, nil)
	d := depsFor(r, engine.Python, "o", "")
	d.Initial.TextPrepared = runner.PrepareText([]byte("ok\xffok"))
	tm := run(t, d)
	s := waitFor(t, tm, "the warning", contains("⚠ input had invalid UTF-8 (replaced with �)"))
	if !strings.Contains(s.screen, "1 ok�ok") {
		t.Fatalf("text line missing:\n%s", s.screen)
	}
	done(t, tm)
}

func TestLoadingStateShown(t *testing.T) {
	block := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(block) }) }
	slow := func(ctx context.Context, dir string) (engine.Engine, error) {
		<-block
		return scripted{engine.Python, nil, matchAll("a")}, nil
	}
	r := setup(t, nil, time.Second, nil, map[engine.Flavor]runner.Factory{engine.Python: slow})
	t.Cleanup(unblock) // runs before r.Close, which waits for the load
	tm := run(t, depsFor(r, engine.Python, "a", "banana"))
	waitFor(t, tm, "the loading notice", contains("loading python engine…"))
	unblock()
	s := waitFor(t, tm, "the result", contains("✓ 3 matches"))
	if strings.Contains(s.screen, "loading python engine") {
		t.Fatal("loading notice still shown")
	}
	// Once the initial flavor is ready the others warm up.
	eventually(t, "warm-up", func() bool { return r.State(engine.Go) == runner.Ready && r.State(engine.JS) == runner.Ready })
	done(t, tm)
}

func TestTimeoutShown(t *testing.T) {
	hang := func(ctx context.Context, _ engine.Request) (engine.Result, error) {
		<-ctx.Done()
		return engine.Result{}, ctx.Err()
	}
	r := setup(t, nil, 50*time.Millisecond, map[engine.Flavor]evalFunc{engine.Python: hang}, nil)
	tm := run(t, depsFor(r, engine.Python, "(a+)+$", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaab"))
	s := waitFor(t, tm, "the timeout", contains("timed out — possible catastrophic backtracking"))
	if !strings.Contains(s.raw, newTheme(true, nil).fg("error").Render("timed out — possible catastrophic backtracking")) {
		t.Fatal("timeout not shown in red")
	}
	done(t, tm)
}

func TestFlavorErrorMarker(t *testing.T) {
	r := setup(t, nil, time.Second, map[engine.Flavor]evalFunc{engine.Python: matchAll("b")},
		map[engine.Flavor]runner.Factory{engine.Go: goFactory})
	tm := run(t, depsFor(r, engine.Python, `(?<=a)b`, "ab"))
	s := waitFor(t, tm, "the go marker", func(s snapshot) bool {
		return strings.Contains(flavorRow(s.screen, "go"), "✗")
	})
	for _, f := range []string{"python", "js", "pcre"} {
		if row := flavorRow(s.screen, f); strings.Contains(row, "✗") {
			t.Errorf("%s marked: %q", f, row)
		}
	}
	done(t, tm)
}

// flavorRow returns the [1] row of flavor f.
func flavorRow(screen, f string) string {
	for _, l := range strings.Split(screen, "\n") {
		left := string([]rune(l)[:autoLeftWidth(testW)])
		if i := strings.Index(left, " "+f+" "); i >= 0 && i < 6 {
			return left
		}
	}
	return ""
}

func TestStaleResultDropped(t *testing.T) {
	m := New(depsFor(setup(t, nil, time.Second, nil, nil), engine.Python, "a", "a")).(*Model)
	m.ev.evalSeq = 2
	one := engine.Result{Matches: []engine.Match{{Span: engine.Span{Start: 0, End: 1}}}}
	m.Update(evalDoneMsg{seq: 1, pattern: "a", text: "a", res: one})
	if m.state.Result != nil {
		t.Fatal("stale result applied")
	}
	m.Update(evalDoneMsg{seq: 2, pattern: "a", text: "a", err: context.Canceled})
	if m.state.Result != nil || m.state.Err != nil {
		t.Fatal("cancelled evaluation should be ignored")
	}
	m.Update(evalDoneMsg{seq: 2, pattern: "a", text: "a", res: one})
	if m.state.Result == nil || len(m.state.Matches) != 1 {
		t.Fatal("current result not applied")
	}
}

func TestPasteGoesToFocusedEditorOnly(t *testing.T) {
	r := setup(t, nil, time.Second, nil, nil)
	tm := run(t, depsFor(r, engine.Python, "p", "t"))
	tm.Send(tea.PasteMsg{Content: "AB\nC"})
	s := waitFor(t, tm, "pasted pattern", func(s snapshot) bool { return s.state.Pattern == "pABC" })
	if s.state.Text != "t" {
		t.Fatalf("text changed to %q", s.state.Text)
	}
	send(tm, tabKey, tea.PasteMsg{Content: "x\ny"}) // [6]
	s = waitFor(t, tm, "pasted text", func(s snapshot) bool { return s.state.Text == "tx\ny" })
	if s.state.Pattern != "pABC" {
		t.Fatalf("pattern changed to %q", s.state.Pattern)
	}
	send(tm, escKey, key('1'), tea.PasteMsg{Content: "zz"}) // no editor
	s = snap(t, tm)
	if s.state.Pattern != "pABC" || s.state.Text != "tx\ny" {
		t.Fatalf("paste outside an editor changed %q / %q", s.state.Pattern, s.state.Text)
	}
	done(t, tm)
}

func TestEditorUndoRedo(t *testing.T) {
	e := newEditor(true)
	e.SetValue("a\tb\r\x01\uFFFD")
	if e.Value() != "a\tb\r\x01\uFFFD" {
		t.Fatalf("text not kept verbatim: %q", e.Value())
	}
	e.SetValue("")
	for _, c := range "ab" {
		e.Update(key(c))
	}
	e.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	e.Update(key('X'))
	if e.Value() != "aXb" {
		t.Fatalf("value %q", e.Value())
	}
	ctrl := func(c rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: c, Mod: tea.ModCtrl} }
	steps := []struct {
		k    tea.KeyPressMsg
		want string
	}{{ctrl('z'), "ab"}, {ctrl('z'), ""}, {ctrl('z'), ""}, {ctrl('y'), "ab"}, {ctrl('y'), "aXb"}, {ctrl('y'), "aXb"}}
	for i, st := range steps {
		e.Update(st.k)
		if e.Value() != st.want {
			t.Fatalf("step %d: %q, want %q", i, e.Value(), st.want)
		}
	}
	e.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	e.Update(key('z'))
	e.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	e.Update(key('Q'))
	if e.Value() != "aQX\nzb" { // redo leaves the cursor after X
		t.Fatalf("multi-line edit: %q", e.Value())
	}
	single := newEditor(false)
	single.Update(tea.PasteMsg{Content: "a\r\nb"})
	single.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if single.Value() != "ab" {
		t.Fatalf("single-line editor: %q", single.Value())
	}
}

func TestPatternSyntaxColours(t *testing.T) {
	th := newTheme(true, nil)
	out := th.highlightPattern(`^(?P<date>\d{4})|x+$`, engine.Python, "g", [2]int{-1, -1})
	for _, c := range []struct{ tok, text string }{
		{"anchor", "^"}, {"paren", "(?P<"}, {"group1", "date"}, {"escape", `\d`},
		{"quant", "{4}"}, {"quant", "|"}, {"quant", "+"}, {"anchor", "$"},
	} {
		if !strings.Contains(out, th.paintStyle(paint{fg: c.tok}).Render(c.text)) {
			t.Errorf("%q not in %s colour: %q", c.text, c.tok, out)
		}
	}
	if got := ansi.Strip(out); got != `^(?P<date>\d{4})|x+$` {
		t.Errorf("text changed: %q", got)
	}
}

func TestCopyAllJSON(t *testing.T) {
	text := "xb"
	res := engine.Result{Unit: engine.UnitBytes, Matches: []engine.Match{{
		Span: engine.Span{Start: 1, End: 2},
		Groups: []engine.Group{
			{Index: 1},
			{Index: 2, Name: "bee", Matched: true, Span: engine.Span{Start: 1, End: 2}},
		},
	}}}
	s := &State{Result: &res, ResultText: text, Matches: byteMatches(text, res)}
	var got []map[string]any
	if err := json.Unmarshal([]byte(matchesJSON(s)), &got); err != nil {
		t.Fatal(err)
	}
	g := got[0]["groups"].([]any)
	if got[0]["value"] != "b" || g[0].(map[string]any)["value"] != nil || g[1].(map[string]any)["value"] != "b" {
		t.Fatalf("json = %v", got)
	}
}

func TestJumpToMatchScrollsText(t *testing.T) {
	var lines []string
	for i := 1; i <= 100; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	lines[79] = "needle here"
	r := setup(t, nil, time.Second, nil, map[engine.Flavor]runner.Factory{engine.Go: goFactory})
	tm := run(t, depsFor(r, engine.Go, "needle", strings.Join(lines, "\n")))
	s := waitFor(t, tm, "go result", contains("✓ 1 match"))
	if strings.Contains(s.screen, " 80 needle here") {
		t.Fatal("line 80 should start off screen")
	}
	send(tm, escKey, key('3'), key('o'))
	waitFor(t, tm, "line 80 on screen", contains(" 80 needle here"))
	done(t, tm)
}

func TestEvalAllTimeoutDoesNotLoop(t *testing.T) {
	rec := &recorder{}
	hang := func(ctx context.Context, _ engine.Request) (engine.Result, error) {
		<-ctx.Done()
		return engine.Result{}, ctx.Err()
	}
	r := setup(t, rec, 50*time.Millisecond, map[engine.Flavor]evalFunc{
		engine.Python: matchAll("x"), engine.JS: hang,
	}, nil)
	tm := run(t, depsFor(r, engine.Python, "x", "xx"))
	eventually(t, "the JS EvalAll", func() bool { return len(rec.of(engine.JS)) > 0 })
	time.Sleep(800 * time.Millisecond) // let any restart events settle
	before := len(rec.of(engine.JS))
	time.Sleep(time.Second) // a loop would run ~3 times a second
	if after := len(rec.of(engine.JS)); after != before {
		t.Fatalf("EvalAll keeps re-running after a timeout: %d -> %d JS calls", before, after)
	}
	done(t, tm)
}

func TestLongLineRendersFast(t *testing.T) {
	text := strings.Repeat("ab", 100_000) // one 200k-rune line
	var ms []engine.Match
	for i := 0; i < len(text); i += 2 {
		ms = append(ms, engine.Match{Span: engine.Span{Start: i, End: i + 1}})
	}
	res := engine.Result{Unit: engine.UnitBytes, Matches: ms}
	s := &State{
		Text: text, Result: &res, Matches: ms, ResultText: text, Mode: engine.ModeMatch,
		Theme: newTheme(true, nil), Focus: PText, Editing: true,
	}
	const w, h = 100, 20
	p := newTextPanel()
	for _, editing := range []bool{false, true} { // editing puts the cursor (and view) at the end
		s.Editing = editing
		start := time.Now()
		out := p.View(s, w, h)
		if d := time.Since(start); d > 300*time.Millisecond {
			t.Errorf("editing=%v: rendering took %v", editing, d)
		}
		for _, l := range strings.Split(out, "\n") {
			if lw := ansi.StringWidth(l); lw > w {
				t.Errorf("editing=%v: line is %d cells, wider than %d", editing, lw, w)
			}
		}
		if editing && !strings.HasSuffix(ansi.Strip(out), "ab ") {
			t.Errorf("view should end at the cursor: %q", ansi.Strip(out)[len(ansi.Strip(out))-20:])
		}
	}
}

func TestControlCharactersNeutralised(t *testing.T) {
	const evil = "a\u009b[2Jb‮c⁦d‏e\x1b[31mf\u0085g"
	bad := []string{"\u009b", "‮", "⁦", "‏", "\x1b[31m", "\u0085"}
	clean := func(what, out string) {
		t.Helper()
		plain := ansi.Strip(out)
		for _, b := range bad {
			if strings.Contains(plain, b) {
				t.Errorf("%s: %q reaches the terminal: %q", what, b, plain)
			}
		}
	}
	th := newTheme(true, nil)
	out := th.highlightText(evil, engine.Result{}, -1)
	clean("highlightText", out)
	if w, n := ansi.StringWidth(ansi.Strip(out)), len([]rune(evil)); w != n {
		t.Errorf("width %d, want one cell per rune (%d)", w, n)
	}
	clean("highlightPattern", th.highlightPattern(evil, engine.Python, "g", [2]int{-1, -1}))
	clean("listRow", listRow(th, evil, 80, true))
	clean("error line", strings.Join(errorLines(&State{Err: &engine.CompileError{Message: evil}}), "\n"))
	clean("value", quoteValue(evil))

	// End to end: piped text, its matches and the replace result.
	r := setup(t, nil, time.Second, nil, map[engine.Flavor]runner.Factory{engine.Go: goFactory})
	tm := run(t, depsFor(r, engine.Go, `b.c`, evil))
	waitFor(t, tm, "go result", contains("✓ 1 match"))
	send(tm, escKey, key('3'), enterKey)
	s := waitFor(t, tm, "expanded", contains("▾ #1"))
	clean("screen", s.raw)
	if !strings.Contains(s.screen, "e␛[31mf") { // ESC drawn as a picture, never raw
		t.Errorf("ESC not neutralised:\n%s", s.screen)
	}
	done(t, tm)
}
