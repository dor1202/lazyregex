package wasmhost

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dor1202/lazyregex/internal/engine/protocol"
)

var echoGZ []byte

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	if _, err := exec.LookPath("go"); err == nil {
		dir, err := os.MkdirTemp("", "echo")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		defer func() { _ = os.RemoveAll(dir) }()
		out := filepath.Join(dir, "echo.wasm")
		cmd := exec.Command("go", "build", "-o", out, "./testdata/echo")
		cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm", "CGO_ENABLED=0")
		if b, err := cmd.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "build echo: %v\n%s", err, b)
			return 1
		}
		raw, err := os.ReadFile(out)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		_, _ = zw.Write(raw)
		_ = zw.Close()
		echoGZ = buf.Bytes()
	}
	return m.Run()
}

func newHost(t *testing.T, cacheDir string) *Host {
	t.Helper()
	if echoGZ == nil {
		t.Skip("go toolchain not on PATH")
	}
	h, err := New(context.Background(), Spec{Name: "echo", ModuleGZ: echoGZ}, cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Close() })
	return h
}

func eval(h *Host, ctx context.Context, pattern, text string) (protocol.Resp, error) {
	return h.Call(ctx, protocol.Req{Op: protocol.OpEval, Pattern: pattern, Text: text})
}

func TestHostEval(t *testing.T) {
	h := newHost(t, t.TempDir())
	resp, err := eval(h, context.Background(), "", "abc")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Output != "ABC" || !resp.OK {
		t.Fatalf("resp = %+v", resp)
	}
	if h.Version() != "echo-1" {
		t.Fatalf("version = %q", h.Version())
	}
}

func TestHostLargeRequest(t *testing.T) {
	h := newHost(t, t.TempDir())
	text := strings.Repeat("a", 5<<20)
	resp, err := eval(h, context.Background(), "", text)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Output) != len(text) {
		t.Fatalf("output len %d", len(resp.Output))
	}
}

func TestHostTimeoutRestarts(t *testing.T) {
	h := newHost(t, t.TempDir())
	before := runtime.NumGoroutine()
	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		start := time.Now()
		_, err := eval(h, ctx, "hang", "")
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err = %v", err)
		}
		if d := time.Since(start); d > time.Second {
			t.Fatalf("took %v", d)
		}
		resp, err := eval(h, context.Background(), "", "x")
		if err != nil || resp.Output != "X" {
			t.Fatalf("after restart: %+v, %v", resp, err)
		}
	}
	if h.Version() != "echo-1" {
		t.Fatalf("version = %q", h.Version())
	}
	// Dead instances must not leak goroutines.
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > before+2 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if after := runtime.NumGoroutine(); after > before+2 {
		t.Fatalf("goroutines grew from %d to %d", before, after)
	}
}

func TestHostCrashReportsStderrAndRestarts(t *testing.T) {
	h := newHost(t, t.TempDir())
	_, err := eval(h, context.Background(), "crash", "")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v", err)
	}
	resp, err := eval(h, context.Background(), "", "y")
	if err != nil || resp.Output != "Y" {
		t.Fatalf("after crash: %+v, %v", resp, err)
	}
}

func TestHostCompileCache(t *testing.T) {
	if echoGZ == nil {
		t.Skip("go toolchain not on PATH")
	}
	dir := t.TempDir()
	start := time.Now()
	h1 := newHost(t, dir)
	first := time.Since(start)
	_ = h1.Close()
	start = time.Now()
	newHost(t, dir)
	second := time.Since(start)
	t.Logf("first New: %v, second New: %v", first, second)
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("cache dir empty: %v", err)
	}
}

// killByTimeout times out a Call so that the next Call has to restart.
func killByTimeout(t *testing.T, h *Host) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := eval(h, ctx, "hang", ""); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("hang: err = %v", err)
	}
}

func TestHostRestartNotChargedToCall(t *testing.T) {
	if echoGZ == nil {
		t.Skip("go toolchain not on PATH")
	}
	spec := Spec{Name: "echo", ModuleGZ: echoGZ, Env: map[string]string{"ECHO_START_DELAY": "500ms"}}
	h, err := New(context.Background(), spec, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = h.Close() }()
	killByTimeout(t, h)
	// The restart takes longer than this Call's whole budget.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	resp, err := eval(h, ctx, "", "x")
	if err != nil || resp.Output != "X" {
		t.Fatalf("after restart: %+v, %v", resp, err)
	}
}

func TestHostCallAlreadyCanceledSkipsRestart(t *testing.T) {
	h := newHost(t, t.TempDir())
	killByTimeout(t, h)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := eval(h, ctx, "", "x"); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if h.inst != nil {
		t.Fatal("restarted for a Call that was already over")
	}
	resp, err := eval(h, context.Background(), "", "y")
	if err != nil || resp.Output != "Y" {
		t.Fatalf("after cancel: %+v, %v", resp, err)
	}
}

// A start whose ping fails (here: New's context is already canceled) must
// not leave the started instance running.
func TestHostFailedStartLeavesNoInstance(t *testing.T) {
	if echoGZ == nil {
		t.Skip("go toolchain not on PATH")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if h, err := New(ctx, Spec{Name: "echo", ModuleGZ: echoGZ}, t.TempDir()); !errors.Is(err, context.Canceled) || h != nil {
		t.Fatalf("New = %v, %v; want nil, context.Canceled", h, err)
	}
	// New discards the Host, so check the same start path on a live one.
	h := newHost(t, t.TempDir())
	killByTimeout(t, h)
	h.mu.Lock()
	err := h.ensureLocked(ctx)
	inst := h.inst
	h.mu.Unlock()
	if !errors.Is(err, context.Canceled) || inst != nil {
		t.Fatalf("ensureLocked = %v, inst = %v; want context.Canceled and no instance", err, inst)
	}
}
