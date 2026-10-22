// Package python adapts CPython, compiled to a WASI command module with its
// own Tools/wasm/wasi build, to engine.Engine. python.wasm runs shim.py,
// which evaluates requests with the interpreter's own re module, loaded with
// the rest of the needed standard library from stdlib.zip.
package python

import (
	"context"
	"embed"
	"io/fs"

	"github.com/dor1202/lazyregex/internal/engine"
	"github.com/dor1202/lazyregex/internal/engine/protocol"
	"github.com/dor1202/lazyregex/internal/engine/wasmhost"
)

// Built by `make wasm-python` from wasm/python (see wasm/versions.env).
//
//go:embed python.wasm.gz
var moduleGZ []byte

// Mounted in the guest at /lib. Built by `make wasm-python` from
// wasm/python/stdlib-allowlist.txt; entries are stored uncompressed so
// zipimport needs no zlib.
//
//go:embed stdlib.zip
var libFS embed.FS

// Mounted in the guest at /shim.
//
//go:embed shim.py
var shimFS embed.FS

type pythonEngine struct{ host *wasmhost.Host }

// New starts the Python engine. cacheDir, if non-empty, holds wazero's
// compilation cache. Matching follows Python's re: match mode is
// Pattern.finditer with the g flag and Pattern.search without, replace is
// Pattern.sub (count=1 without g) and split Pattern.split.
func New(ctx context.Context, cacheDir string) (engine.Engine, error) {
	h, err := wasmhost.New(ctx, wasmhost.Spec{
		Name:     "python",
		ModuleGZ: moduleGZ,
		// -I less its -E (which would ignore PYTHONPATH and PYTHONHOME; the
		// guest sees no host environment anyway, only Env). -W ignore keeps
		// re's FutureWarnings off stderr.
		Args:   []string{"python", "-P", "-s", "-S", "-u", "-W", "ignore", "/shim/shim.py"},
		Env:    map[string]string{"PYTHONPATH": "/lib/stdlib.zip", "PYTHONHOME": "/"},
		Mounts: map[string]fs.FS{"/lib": libFS, "/shim": shimFS},
	}, cacheDir)
	if err != nil {
		return nil, err
	}
	return &pythonEngine{host: h}, nil
}

func (e *pythonEngine) Info() engine.Info {
	return engine.Info{
		Flavor:     engine.Python,
		EngineName: "CPython re",
		Version:    e.host.Version(),
		Flags: []engine.FlagInfo{
			{Letter: 'g', Name: "global", Default: true},
			{Letter: 'i', Name: "IGNORECASE"},
			{Letter: 'm', Name: "MULTILINE"},
			{Letter: 's', Name: "DOTALL"},
			{Letter: 'x', Name: "VERBOSE"},
			{Letter: 'a', Name: "ASCII"},
		},
	}
}

func (e *pythonEngine) Eval(ctx context.Context, req engine.Request) (engine.Result, error) {
	resp, err := e.host.Call(ctx, protocol.Req{
		Op:          protocol.OpEval,
		Pattern:     req.Pattern,
		Flags:       req.Flags,
		Text:        req.Text,
		Mode:        string(req.Mode),
		Replacement: req.Replacement,
	})
	if err != nil {
		return engine.Result{}, err
	}
	return protocol.ToResult(resp, engine.UnitCodePoints)
}

func (e *pythonEngine) Close() error { return e.host.Close() }
