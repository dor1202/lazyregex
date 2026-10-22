package main

import (
	"bytes"
	"io"
	"strings"
	"testing"

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
	oldI, oldR := interactive, runTUI
	t.Cleanup(func() { interactive, runTUI = oldI, oldR })
	interactive = func(io.Reader, io.Writer) bool { return true }
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
	if got.Runner == nil || got.Initial.Flavor != engine.Go {
		t.Fatalf("deps = %+v", got)
	}
	if evalErr != nil || n != 2 {
		t.Fatalf("go engine via runner: %v, %d matches", evalErr, n)
	}
}
