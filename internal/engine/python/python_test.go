package python

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dor1202/lazyregex/internal/engine"
	"github.com/dor1202/lazyregex/parity"
)

// cacheDir is a compilation cache shared by the tests, so that only the
// first New pays for compiling python.wasm.
var cacheDir string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "python-engine-cache")
	if err != nil {
		panic(err)
	}
	cacheDir = dir
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func newEngine(t *testing.T) engine.Engine {
	t.Helper()
	e, err := New(context.Background(), cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { e.Close() })
	return e
}

func TestPythonPing(t *testing.T) {
	info := newEngine(t).Info()
	if !strings.HasPrefix(info.Version, "3.14.") {
		t.Errorf("Version = %q, want 3.14.x", info.Version)
	}
	if info.Flavor != engine.Python || info.EngineName != "CPython re" {
		t.Errorf("info = %+v", info)
	}
	var letters string
	for _, f := range info.Flags {
		letters += string(f.Letter)
		if f.Default != (f.Letter == 'g') {
			t.Errorf("flag %c default = %v", f.Letter, f.Default)
		}
	}
	if letters != "gimsxa" {
		t.Errorf("flags = %q, want gimsxa", letters)
	}
}

func TestPythonNamedGroups(t *testing.T) {
	res, err := newEngine(t).Eval(context.Background(), engine.Request{
		Pattern: `(?P<date>\d{4}-\d{2}-\d{2}) (?P<level>INFO|WARN|ERROR)`,
		Flags:   "g", Text: "2026-10-20 INFO  x", Mode: engine.ModeMatch,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 1 || len(res.Matches[0].Groups) != 2 {
		t.Fatalf("matches = %+v", res.Matches)
	}
	want := []engine.Group{
		{Index: 1, Name: "date", Matched: true, Span: engine.Span{Start: 0, End: 10}},
		{Index: 2, Name: "level", Matched: true, Span: engine.Span{Start: 11, End: 15}},
	}
	for i, g := range res.Matches[0].Groups {
		if g != want[i] {
			t.Errorf("group %d = %+v, want %+v", i, g, want[i])
		}
	}
}

func TestPythonCodePoints(t *testing.T) {
	res, err := newEngine(t).Eval(context.Background(), engine.Request{
		Pattern: `😀(b)`, Flags: "g", Text: "a😀b", Mode: engine.ModeMatch,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 1 || len(res.Matches[0].Groups) != 1 {
		t.Fatalf("matches = %+v", res.Matches)
	}
	m := res.Matches[0]
	if m.Span != (engine.Span{Start: 1, End: 3}) || m.Groups[0].Span != (engine.Span{Start: 2, End: 3}) {
		t.Errorf("match = %+v", m)
	}
	if res.Unit != engine.UnitCodePoints {
		t.Errorf("unit = %v", res.Unit)
	}
}

func TestPythonErrorVerbatim(t *testing.T) {
	_, err := newEngine(t).Eval(context.Background(), engine.Request{
		Pattern: `(`, Flags: "g", Text: "x", Mode: engine.ModeMatch,
	})
	var ce *engine.CompileError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v, want CompileError", err)
	}
	if ce.Message != "missing ), unterminated subpattern at position 0" || ce.Offset != 0 {
		t.Errorf("got %q offset %d", ce.Message, ce.Offset)
	}
}

// A bad replacement template fails after the pattern compiled: the message
// is still verbatim, but there is no position in the pattern to report.
func TestPythonReplacementErrorHasNoOffset(t *testing.T) {
	_, err := newEngine(t).Eval(context.Background(), engine.Request{
		Pattern: `(a)b`, Flags: "g", Text: "ab", Mode: engine.ModeReplace, Replacement: `\2`,
	})
	var ce *engine.CompileError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v, want CompileError", err)
	}
	if ce.Message != "invalid group reference 2 at position 1" || ce.Offset != -1 {
		t.Errorf("got %q offset %d", ce.Message, ce.Offset)
	}
}

// The case-running code is shared with the fixture generator so the engine
// and the native fixtures cannot drift apart.
func TestShimMatchesGenerator(t *testing.T) {
	shared := func(path string) []byte {
		t.Helper()
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		parts := bytes.Split(b, []byte("# --- shared ---\n"))
		if len(parts) != 3 {
			t.Fatalf("%s: want exactly two '# --- shared ---' markers, got %d", path, len(parts)-1)
		}
		return parts[1]
	}
	shim := shared("shim.py")
	gen := shared(filepath.Join(parity.Dir(), "gen", "gen_python.py"))
	if !bytes.Equal(shim, gen) {
		t.Error("shim.py and parity/gen/gen_python.py differ between the '# --- shared ---' markers")
	}
}

func TestParityPython(t *testing.T) {
	parity.RunParity(t, engine.Python, newEngine(t))
}

// Startup costs, logged for the TUI's loading state (go test -v -run Startup).
func TestPythonStartupTimes(t *testing.T) {
	dir := t.TempDir() // empty, unlike cacheDir
	start := time.Now()
	e1, err := New(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	cold := time.Since(start)
	e1.Close()
	start = time.Now()
	e2, err := New(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer e2.Close()
	warm := time.Since(start)

	// Catastrophic backtracking: only the deadline stops it.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	_, err = e2.Eval(ctx, engine.Request{Pattern: `(a+)+$`, Flags: "g", Text: strings.Repeat("a", 40) + "!", Mode: engine.ModeMatch})
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want deadline exceeded", err)
	}
	start = time.Now()
	res, err := e2.Eval(context.Background(), engine.Request{Pattern: `b`, Flags: "g", Text: "abc", Mode: engine.ModeMatch})
	if err != nil || len(res.Matches) != 1 {
		t.Fatalf("after timeout: %+v, %v", res, err)
	}
	restart := time.Since(start)
	t.Logf("New with empty cache: %v; with warm cache: %v; first Eval after a timeout (restart): %v", cold, warm, restart)
}
