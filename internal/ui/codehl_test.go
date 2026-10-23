package ui

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/dor1202/lazyregex/internal/engine"
	"github.com/dor1202/lazyregex/internal/runner"
	"github.com/dor1202/lazyregex/internal/syntax"
)

// codeState is a Code tab state for flavor f with its engine loaded.
func codeState(t *testing.T, f engine.Flavor, pattern, text string, dark bool) *State {
	t.Helper()
	r := setup(t, nil, time.Second, nil, nil)
	r.Load(f)
	eventually(t, string(f)+" loaded", func() bool { return r.State(f) == runner.Ready })
	return &State{Flavor: f, Flags: map[engine.Flavor]string{}, Runner: r, Tab: map[PanelID]int{PDetails: tabCode},
		Pattern: pattern, Text: text, Theme: newTheme(dark, nil)}
}

// plainCode is how the Code tab drew a snippet before highlighting: each
// line sanitised, tabs as four spaces.
func plainCode(t *testing.T, s *State) []string {
	t.Helper()
	code, ok := snippet(s)
	if !ok {
		t.Fatal("no snippet")
	}
	lines := strings.Split(strings.TrimSuffix(code, "\n"), "\n")
	for i, l := range lines {
		lines[i] = sanitize(strings.ReplaceAll(l, "\t", "    "))
	}
	return lines
}

// sgr matches an SGR sequence; colourBefore a truecolor foreground (after
// any attributes) directly in front of a word.
var (
	sgr          = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	colourBefore = func(word string) *regexp.Regexp {
		return regexp.MustCompile(`\x1b\[38;2;\d+;\d+;\d+m` + regexp.QuoteMeta(word))
	}
)

func TestCodeTabHighlighted(t *testing.T) {
	keyword := map[engine.Flavor]string{engine.Python: "import", engine.JS: "const", engine.PCRE: "<?php", engine.Go: "func"}
	for _, f := range engine.Flavors {
		st := codeState(t, f, `(\w+)@(\w+)\.\w+`, "ann@x.com\tBob@y.org\nzed", true)
		view := newDetailsPanel().View(st, 200, 40)
		if !colourBefore(keyword[f]).MatchString(view) {
			t.Errorf("%s: %q is not coloured:\n%q", f, keyword[f], view)
		}
		if strings.Contains(view, "48;2;") {
			t.Errorf("%s: a token sets a background", f)
		}
		// Without the colour it is exactly the plain rendering.
		if got, want := ansi.Strip(view), strings.Join(plainCode(t, st), "\n"); got != want {
			t.Errorf("%s: stripped view differs from the plain rendering:\n%s\n---\n%s", f, got, want)
		}
		for i, l := range strings.Split(view, "\n") {
			if !strings.HasSuffix(l, syntax.Reset) {
				t.Errorf("%s: line %d does not end with a reset: %q", f, i, l)
			}
		}
	}
}

func TestCodeTabWidth(t *testing.T) {
	for _, f := range engine.Flavors {
		st := codeState(t, f, `(\d+)`+strings.Repeat("x", 80), "a1b22 wide ｗｉｄｅ text", true)
		plain := plainCode(t, st)
		for _, w := range []int{7, 20, 33} {
			lines := strings.Split(newDetailsPanel().View(st, w, 40), "\n")
			if len(lines) != len(plain) {
				t.Fatalf("%s: %d lines, want %d", f, len(lines), len(plain))
			}
			for i, l := range lines {
				if ansi.StringWidth(l) > w {
					t.Errorf("%s w=%d: line %d is %d cells: %q", f, w, i, ansi.StringWidth(l), l)
				}
				if !strings.HasSuffix(l, syntax.Reset) {
					t.Errorf("%s w=%d: line %d bleeds: %q", f, w, i, l)
				}
				if got, want := ansi.Strip(l), strings.TrimRight(fitWidth(plain[i], w), " "); strings.TrimRight(got, " ") != want {
					t.Errorf("%s w=%d: line %d is %q, want %q", f, w, i, got, want)
				}
			}
		}
		// The panel box stays exact and the border is never coloured by code.
		box := renderBox(st.Theme, boxSpec{title: "[7]", body: newDetailsPanel().View(st, 18, 10), w: 20, h: 12, border: "border"})
		for i, l := range strings.Split(box, "\n") {
			if ansi.StringWidth(l) != 20 {
				t.Errorf("%s: box line %d is %d cells", f, i, ansi.StringWidth(l))
			}
		}
	}
}

func TestCodeTabControlCharsNeutralised(t *testing.T) {
	const evil = "\u009b\u202e\u2066\x1b[31m\x07\x7f"
	for _, f := range engine.Flavors {
		st := codeState(t, f, "a"+evil+"b", "x"+evil, true)
		view := newDetailsPanel().View(st, 200, 40)
		// Only the highlighter's SGR sequences may be escapes.
		rest := sgr.ReplaceAllString(view, "")
		for _, r := range rest {
			if (r < 0x20 && r != '\n') || r == 0x7f || (r >= 0x80 && r <= 0x9f) || r == 0x202e || r == 0x2066 {
				t.Errorf("%s: %U reaches the terminal", f, r)
			}
		}
		if !strings.Contains(rest, "␛[31m") {
			t.Errorf("%s: ESC is not drawn as a control picture: %q", f, rest)
		}
		// y still copies the raw snippet.
		if got := clipboardText(t, newDetailsPanel().Update(key('y'), st)); !strings.Contains(got, "\x1b[31m") || strings.Contains(got, "38;2;") {
			t.Errorf("%s: clipboard is not the raw, uncoloured snippet: %q", f, got)
		}
	}
}

func TestCodeTabLightDark(t *testing.T) {
	dark, light := newTheme(true, nil), newTheme(false, nil)
	if codeStyle(dark) == codeStyle(light) || syntax.Style(codeStyle(dark)) == nil || syntax.Style(codeStyle(light)) == nil {
		t.Fatalf("styles: dark %q, light %q", codeStyle(dark), codeStyle(light))
	}
	st := codeState(t, engine.Go, `\d+`, "a1", true)
	p := newDetailsPanel()
	d := p.View(st, 200, 40)
	st.Theme = light
	l := p.View(st, 200, 40)
	if d == l || ansi.Strip(d) != ansi.Strip(l) {
		t.Error("light and dark should differ only in colour")
	}
}

func TestCodeTabCached(t *testing.T) {
	st := codeState(t, engine.Go, `\d+`, "a1", true)
	p := newDetailsPanel()
	p.View(st, 200, 40)
	first := &p.code.lines[0]
	p.View(st, 100, 10)
	if &p.code.lines[0] != first {
		t.Error("the same snippet and style were highlighted again")
	}
	st.Pattern = `\w+`
	p.View(st, 200, 40)
	if &p.code.lines[0] == first {
		t.Error("a new snippet was not highlighted")
	}
}

// TestCodeTabDownsampled checks that the renderer converts the truecolor
// highlighting to the terminal's colour profile.
func TestCodeTabDownsampled(t *testing.T) {
	r := setup(t, nil, time.Second, nil, map[engine.Flavor]runner.Factory{engine.Go: goFactory})
	tm := teatest.NewTestModel(t, New(depsFor(r, engine.Go, `(\d+)`, "a1b22")),
		teatest.WithInitialTermSize(testW, testH), teatest.WithProgramOptions(tea.WithColorProfile(colorprofile.ANSI256)))
	detailsTab(tm, tabCode)
	var out []byte
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		out = b
		return bytes.Contains(b, []byte("MustCompile"))
	}, teatest.WithDuration(5*time.Second))
	if bytes.Contains(out, []byte("38;2;")) {
		t.Error("truecolor reaches a 256-colour terminal")
	}
	if !bytes.Contains(out, []byte("38;5;")) {
		t.Error("no 256-colour foreground in the output")
	}
	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
