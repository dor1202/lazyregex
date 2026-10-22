// Package wasmhost runs a regex engine compiled to a WASI command module
// in-process with wazero, speaking newline-delimited JSON over stdin/stdout.
// A Call whose context ends kills the instance; the next Call transparently
// starts a fresh one.
package wasmhost

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"

	"github.com/dor1202/lazyregex/internal/engine/protocol"
)

const stderrRingSize = 64 << 10

// Spec describes an engine module.
type Spec struct {
	Name     string            // e.g. "pcre"
	ModuleGZ []byte            // gzip'd wasm (embedded by adapters)
	Args     []string          // full argv including argv[0]; defaults to {Name}
	Env      map[string]string //
	Mounts   map[string]fs.FS  // guest path -> fs
}

// Host owns one live, restartable instance of a module.
type Host struct {
	spec  Spec
	wasm  []byte
	cache wazero.CompilationCache

	mu     sync.Mutex // serializes Calls and guards inst/nextID/closed
	inst   *instance
	nextID int
	closed bool

	vmu     sync.Mutex
	version string
}

// New compiles the module (using cacheDir as the on-disk compilation cache
// when non-empty), starts an instance and pings it.
func New(ctx context.Context, spec Spec, cacheDir string) (*Host, error) {
	zr, err := gzip.NewReader(bytes.NewReader(spec.ModuleGZ))
	if err != nil {
		return nil, fmt.Errorf("%s: module gzip: %w", spec.Name, err)
	}
	wasm, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("%s: module gzip: %w", spec.Name, err)
	}
	h := &Host{spec: spec, wasm: wasm}
	if cacheDir != "" {
		h.cache, err = wazero.NewCompilationCacheWithDir(cacheDir)
		if err != nil {
			return nil, fmt.Errorf("%s: compilation cache: %w", spec.Name, err)
		}
	} else {
		h.cache = wazero.NewCompilationCache()
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.ensureLocked(ctx); err != nil {
		h.cache.Close(context.Background())
		return nil, err
	}
	return h, nil
}

// Version returns the engine version reported by the latest ping.
func (h *Host) Version() string {
	h.vmu.Lock()
	defer h.vmu.Unlock()
	return h.version
}

// Close kills the instance and releases the compilation cache.
func (h *Host) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil
	}
	h.closed = true
	h.dropLocked()
	return h.cache.Close(context.Background())
}

// Call sends one request and returns its response. If ctx ends first the
// instance is killed and ctx.Err() is returned.
func (h *Host) Call(ctx context.Context, req protocol.Req) (protocol.Resp, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return protocol.Resp{}, errors.New(h.spec.Name + ": host closed")
	}
	if err := h.ensureLocked(ctx); err != nil {
		return protocol.Resp{}, err
	}
	return h.roundTripLocked(ctx, req)
}

// ensureLocked starts and pings an instance if none is live.
func (h *Host) ensureLocked(ctx context.Context) error {
	if h.inst != nil {
		return nil
	}
	inst, err := h.startInstance()
	if err != nil {
		return fmt.Errorf("%s: start: %w", h.spec.Name, err)
	}
	h.inst = inst
	resp, err := h.roundTripLocked(ctx, protocol.Req{Op: protocol.OpPing})
	if err != nil {
		return err
	}
	h.vmu.Lock()
	h.version = resp.Version
	h.vmu.Unlock()
	return nil
}

func (h *Host) dropLocked() {
	if h.inst != nil {
		h.inst.close()
		h.inst = nil
	}
}

func (h *Host) roundTripLocked(ctx context.Context, req protocol.Req) (protocol.Resp, error) {
	inst := h.inst
	h.nextID++
	req.ID = h.nextID
	line, err := json.Marshal(req)
	if err != nil {
		return protocol.Resp{}, err
	}
	line = append(line, '\n')

	writeErr := make(chan error, 1)
	go func() {
		_, err := inst.stdin.Write(line)
		writeErr <- err
	}()
	wrote := false
	for !wrote {
		select {
		case <-ctx.Done():
			h.dropLocked()
			return protocol.Resp{}, ctx.Err()
		case err := <-writeErr:
			if err != nil {
				return protocol.Resp{}, h.failLocked(ctx, "write", err)
			}
			wrote = true
		}
	}
	select {
	case <-ctx.Done():
		h.dropLocked()
		return protocol.Resp{}, ctx.Err()
	case out, ok := <-inst.lines:
		if !ok {
			return protocol.Resp{}, h.failLocked(ctx, "read", io.EOF)
		}
		var resp protocol.Resp
		if err := json.Unmarshal(out, &resp); err != nil {
			h.dropLocked()
			return protocol.Resp{}, fmt.Errorf("%s: bad response: %w", h.spec.Name, err)
		}
		if resp.ID != req.ID {
			h.dropLocked()
			return protocol.Resp{}, fmt.Errorf("%s: response id %d, want %d", h.spec.Name, resp.ID, req.ID)
		}
		return resp, nil
	}
}

// failLocked reports an unexpected instance exit, including captured stderr.
func (h *Host) failLocked(ctx context.Context, op string, cause error) error {
	inst := h.inst
	select {
	case <-inst.exited:
	case <-ctx.Done():
	}
	msg := fmt.Sprintf("%s: engine exited unexpectedly (%s: %v)", h.spec.Name, op, cause)
	if e := inst.exitErr; e != nil {
		msg += ": " + e.Error()
	}
	if s := strings.TrimSpace(inst.stderr.String()); s != "" {
		msg += "\nstderr: " + s
	}
	h.dropLocked()
	return errors.New(msg)
}

// instance is one running module with its pipes and goroutines.
type instance struct {
	rt     wazero.Runtime
	cancel context.CancelFunc
	stdin  *io.PipeWriter
	inR    *io.PipeReader
	outR   *io.PipeReader
	lines  chan []byte
	stderr *ring

	exited  chan struct{} // closed when the module's _start returned
	exitErr error         // valid after exited is closed
	stop    chan struct{} // closed by close() to release the reader goroutine
	wg      sync.WaitGroup
	once    sync.Once
}

func (h *Host) startInstance() (*instance, error) {
	runCtx, cancel := context.WithCancel(context.Background())
	rt := wazero.NewRuntimeWithConfig(runCtx, wazero.NewRuntimeConfig().
		WithCloseOnContextDone(true).
		WithCompilationCache(h.cache))
	fail := func(err error) (*instance, error) {
		cancel()
		rt.Close(context.Background())
		return nil, err
	}
	wasi_snapshot_preview1.MustInstantiate(runCtx, rt)
	compiled, err := rt.CompileModule(runCtx, h.wasm)
	if err != nil {
		return fail(err)
	}

	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	inst := &instance{
		rt: rt, cancel: cancel, stdin: inW, inR: inR, outR: outR,
		lines: make(chan []byte), stderr: &ring{},
		exited: make(chan struct{}), stop: make(chan struct{}),
	}

	args := h.spec.Args
	if len(args) == 0 {
		args = []string{h.spec.Name}
	}
	fsc := wazero.NewFSConfig()
	for guest, fsys := range h.spec.Mounts {
		fsc = fsc.WithFSMount(fsys, guest)
	}
	cfg := wazero.NewModuleConfig().
		WithName("").
		WithStdin(inR).WithStdout(outW).WithStderr(inst.stderr).
		WithArgs(args...).
		WithFSConfig(fsc).
		WithSysWalltime().WithSysNanotime()
	for k, v := range h.spec.Env {
		cfg = cfg.WithEnv(k, v)
	}

	inst.wg.Add(2)
	go func() {
		defer inst.wg.Done()
		_, err := rt.InstantiateModule(runCtx, compiled, cfg)
		inst.exitErr = err
		close(inst.exited)
		inR.Close()
		outW.Close()
	}()
	go func() {
		defer inst.wg.Done()
		defer close(inst.lines)
		br := bufio.NewReaderSize(outR, 64<<10)
		for {
			line, err := br.ReadBytes('\n')
			if err != nil {
				return
			}
			select {
			case inst.lines <- line:
			case <-inst.stop:
				return
			}
		}
	}()
	return inst, nil
}

// close kills the module and waits for its goroutines. Safe to call twice.
func (i *instance) close() {
	i.once.Do(func() {
		close(i.stop)
		i.stdin.Close()
		i.inR.Close()
		i.outR.Close()
		i.cancel()
		<-i.exited // let the module finish closing itself before the runtime does
		i.rt.Close(context.Background())
		i.wg.Wait()
	})
}

// ring keeps the last stderrRingSize bytes written to it.
type ring struct {
	mu  sync.Mutex
	buf []byte
}

func (r *ring) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf = append(r.buf, p...)
	if len(r.buf) > stderrRingSize {
		r.buf = append([]byte(nil), r.buf[len(r.buf)-stderrRingSize:]...)
	}
	return len(p), nil
}

func (r *ring) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return string(r.buf)
}
