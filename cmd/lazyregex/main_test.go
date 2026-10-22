package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
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
	if !strings.HasPrefix(out.String(), "lazyregex dev") {
		t.Fatalf("got %q", out.String())
	}
}

func TestNonInteractivePrintsNotice(t *testing.T) {
	var out bytes.Buffer
	if code := run(nil, strings.NewReader(""), &out, io.Discard); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out.String(), "needs a terminal") {
		t.Fatalf("got %q", out.String())
	}
}

func TestInteractiveStartsTUI(t *testing.T) {
	oldI, oldR, oldC := interactive, runTUI, configDir
	t.Cleanup(func() { interactive, runTUI, configDir = oldI, oldR, oldC })
	interactive = func(io.Reader, io.Writer) bool { return true }
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("mouse: false\ntimeout: 750ms\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configDir = func() (string, error) { return dir, nil }
	var got *ui.Deps
	var evalErr error
	var n int
	runTUI = func(d ui.Deps) error {
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
	if got.Runner == nil || got.Initial.Flavor != engine.Go {
		t.Fatalf("deps = %+v", got)
	}
	if evalErr != nil || n != 2 {
		t.Fatalf("go engine via runner: %v, %d matches", evalErr, n)
	}
}
