package main

import (
	"bytes"
	"errors"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dor1202/lazyregex/internal/engine"
	"github.com/dor1202/lazyregex/internal/ui"
)

func TestVersionFlag(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{"--version"}, strings.NewReader(""), &out, io.Discard)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	want := "lazyregex dev\n" +
		"python  CPython 3.14.9 (re)\n" +
		"js      QuickJS-ng 0.17.0\n" +
		"pcre    PCRE2 10.44\n" +
		"go      Go regexp (RE2) " + runtime.Version() + "\n"
	if out.String() != want {
		t.Fatalf("got\n%s\nwant\n%s", out.String(), want)
	}
}

// `lazyregex | cat`: the notice goes to stderr and the exit code is 1.
func TestNonInteractivePrintsNotice(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run(nil, strings.NewReader(""), &out, &errb); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if out.Len() != 0 || !strings.Contains(errb.String(), "needs a terminal") {
		t.Fatalf("stdout %q, stderr %q", out.String(), errb.String())
	}
}

func TestInteractiveStartsTUI(t *testing.T) {
	oldI, oldR, oldC, oldT := interactive, runTUI, configDir, openTTY
	t.Cleanup(func() { interactive, runTUI, configDir, openTTY = oldI, oldR, oldC, oldT })
	interactive = func(io.Writer) bool { return true }
	openTTY = func() (io.Reader, func(), error) { return strings.NewReader(""), func() {}, nil }
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("mouse: false\ntimeout: 750ms\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configDir = func() (string, error) { return dir, nil }
	var got *ui.Deps
	var evalErr error
	var n int
	runTUI = func(d ui.Deps, _ io.Reader) error {
		got = &d
		res, err := d.Runner.Eval(engine.Go, engine.Request{Pattern: `\d+`, Flags: "g", Text: "a1b22", Mode: engine.ModeMatch})
		evalErr, n = err, len(res.Matches)
		return nil
	}

	if code := run(nil, strings.NewReader(""), io.Discard, io.Discard); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if got == nil {
		t.Fatal("TUI not started")
	}
	if got.ConfigDir != dir || got.Config.Mouse || got.Config.Timeout != 750*time.Millisecond {
		t.Fatalf("config not loaded from the hooked dir: %+v", got.Config)
	}
	if got.Runner == nil || got.Initial.Flavor != engine.Python {
		t.Fatalf("deps = %+v", got)
	}
	if evalErr != nil || n != 2 {
		t.Fatalf("go engine via runner: %v, %d matches", evalErr, n)
	}
}

func runCLI(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

func TestCLIExplain(t *testing.T) {
	code, out, _ := runCLI(t, "", "--explain", "-f", "python", `^\d+$`)
	if code != 0 || !strings.Contains(out, "a digit, 1 or more") {
		t.Fatalf("exit %d, out %q", code, out)
	}
	code, out, _ = runCLI(t, "", "--explain", "-p", `a|b`)
	if code != 0 || out == "" {
		t.Fatalf("-p: exit %d, out %q", code, out)
	}
}

// On a terminal, control characters in the pattern are shown as control
// pictures, never written raw; piped output stays byte-exact.
func TestCLIExplainControlChars(t *testing.T) {
	old := interactive
	t.Cleanup(func() { interactive = old })
	for _, tty := range []bool{true, false} {
		interactive = func(io.Writer) bool { return tty }
		code, out, _ := runCLI(t, "", "--explain", "-f", "go", "a\x1b\x07")
		if code != 0 {
			t.Fatalf("tty %v: exit %d", tty, code)
		}
		raw := strings.ContainsAny(out, "\x1b\x07")
		if tty && (raw || !strings.Contains(out, "\u241b") || !strings.HasSuffix(out, "\n")) {
			t.Errorf("tty: control characters not mapped: %q", out)
		}
		if !tty && !raw {
			t.Errorf("piped: output changed: %q", out)
		}
	}
}

func TestCLIExplainNeedsPattern(t *testing.T) {
	if code, _, _ := runCLI(t, "", "--explain"); code != 2 {
		t.Fatalf("exit %d", code)
	}
}

func TestCLIUnknownFlavor(t *testing.T) {
	code, _, errs := runCLI(t, "", "-f", "x")
	if code != 2 || !strings.Contains(errs, `unknown flavor "x" (want python, js, pcre, go)`) {
		t.Fatalf("exit %d, stderr %q", code, errs)
	}
}

func TestCLIUnknownFlag(t *testing.T) {
	code, _, errs := runCLI(t, "", "--explain", "-f", "go", "--flags", "iz", "a")
	if code != 2 || !strings.Contains(errs, `unknown flag "z" for go (want gimsU)`) {
		t.Fatalf("exit %d, stderr %q", code, errs)
	}
	if code, _, _ := runCLI(t, "", "--bogus"); code != 2 {
		t.Fatalf("bad option exit %d", code)
	}
}

func TestCLIHelp(t *testing.T) {
	code, out, _ := runCLI(t, "", "-h")
	if code != 0 || !strings.Contains(out, "usage: lazyregex") {
		t.Fatalf("exit %d, out %q", code, out)
	}
}

func TestCLILicenses(t *testing.T) {
	code, out, _ := runCLI(t, "", "--licenses")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"PCRE2", "Python Software Foundation", "QuickJS-ng", "wasi-libc", "MIT License"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(out, "cJSON") {
		t.Error("cJSON is not bundled")
	}
	if strings.Index(out, "lazyregex - MIT") > strings.Index(out, "CPython") {
		t.Error("lazyregex licence must come first")
	}
}

// hookTUI captures the Deps (and keyboard input) instead of starting the UI.
func hookTUI(t *testing.T, tty func() (io.Reader, func(), error)) *ui.Deps {
	t.Helper()
	oldI, oldR, oldC, oldT := interactive, runTUI, configDir, openTTY
	t.Cleanup(func() { interactive, runTUI, configDir, openTTY = oldI, oldR, oldC, oldT })
	interactive = func(io.Writer) bool { return true }
	configDir = func() (string, error) { return t.TempDir(), nil }
	openTTY = tty
	got := new(ui.Deps)
	runTUI = func(d ui.Deps, _ io.Reader) error { *got = d; return nil }
	return got
}

func okTTY() (io.Reader, func(), error) { return strings.NewReader(""), func() {}, nil }

func TestCLIStdinBinary(t *testing.T) {
	got := hookTUI(t, okTTY)
	b := make([]byte, 2_000_000)
	rand.New(rand.NewSource(1)).Read(b)
	var errb bytes.Buffer
	if code := run(nil, bytes.NewReader(b), io.Discard, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if !got.Initial.TextPrepared.Manual || !got.Initial.TextPrepared.InvalidUTF8 {
		t.Fatalf("prepared = Manual %v, InvalidUTF8 %v", got.Initial.TextPrepared.Manual, got.Initial.TextPrepared.InvalidUTF8)
	}
}

func TestCLIPresetFlavorPattern(t *testing.T) {
	got := hookTUI(t, okTTY)
	if code := run([]string{"-f", "js", "--flags", "gi", "-p", `\d+`}, strings.NewReader("a1"), io.Discard, io.Discard); code != 0 {
		t.Fatalf("exit %d", code)
	}
	in := got.Initial
	if in.Flavor != engine.JS || in.Pattern != `\d+` || in.Flags != "gi" || in.TextPrepared.Text != "a1" {
		t.Fatalf("initial = %+v", in)
	}
	run([]string{`x+`}, strings.NewReader(""), io.Discard, io.Discard)
	if got.Initial.Pattern != "x+" {
		t.Fatalf("positional pattern = %q", got.Initial.Pattern)
	}
}

func TestCLINoTerminalForInput(t *testing.T) {
	hookTUI(t, func() (io.Reader, func(), error) { return nil, nil, errors.New("no tty") })
	var errb bytes.Buffer
	if code := run(nil, strings.NewReader("x"), io.Discard, &errb); code != 1 || !strings.Contains(errb.String(), "no terminal for interactive input") {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
}

func TestCLIConfigErrorReported(t *testing.T) {
	got := hookTUI(t, okTTY)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("mouse: [unclosed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configDir = func() (string, error) { return dir, nil }
	var errb bytes.Buffer
	if code := run(nil, strings.NewReader(""), io.Discard, &errb); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.HasPrefix(errb.String(), "lazyregex: ") || got.Runner == nil {
		t.Fatalf("stderr %q", errb.String())
	}
}

func TestCacheDirEmptyWithoutUserCache(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("LocalAppData", "")
	if d := cacheDir(); d != "" {
		t.Fatalf("cacheDir = %q, want empty (no temp-dir fallback)", d)
	}
}

type closeTracker struct {
	io.Reader
	closed bool
}

// The keyboard handle must stay open until the TUI has returned.
func TestCLITTYOpenWhileTUIRuns(t *testing.T) {
	hookTUI(t, nil)
	ct := &closeTracker{Reader: strings.NewReader("")}
	openTTY = func() (io.Reader, func(), error) { return ct, func() { ct.closed = true }, nil }
	var openDuring, sawReader bool
	runTUI = func(d ui.Deps, in io.Reader) error {
		sawReader = in == io.Reader(ct)
		openDuring = !ct.closed
		return nil
	}
	if code := run(nil, strings.NewReader("x"), io.Discard, io.Discard); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !sawReader || !openDuring || !ct.closed {
		t.Fatalf("passed=%v openDuring=%v closedAfter=%v", sawReader, openDuring, ct.closed)
	}
}

// The real openTTY's handle must be usable until its close func runs (it was
// once closed early because input and output are the same file).
func TestOpenTTYHandleSurvives(t *testing.T) {
	in, closeFn, err := openTTY()
	if err != nil {
		t.Skip("no controlling terminal:", err)
	}
	f, ok := in.(*os.File)
	if !ok {
		closeFn()
		t.Skip("not a file")
	}
	if _, err := f.Stat(); err != nil {
		t.Fatalf("handle closed on return: %v", err)
	}
	closeFn()
	if _, err := f.Stat(); err == nil {
		t.Fatal("close func did not close the handle")
	}
}

func TestCLIStartupNotice(t *testing.T) {
	got := hookTUI(t, okTTY)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("mouse: [unclosed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configDir = func() (string, error) { return dir, nil }
	run(nil, strings.NewReader(""), io.Discard, io.Discard)
	if got.Initial.Notice == "" {
		t.Fatal("config error not passed as a startup notice")
	}
}
