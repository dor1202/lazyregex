// Package js adapts QuickJS-ng, an ECMAScript engine compiled to a WASI
// command module, to engine.Engine. Its qjs runner executes shim.js, which
// evaluates requests with the engine's own RegExp.
package js

import (
	"context"
	"embed"
	"io/fs"

	"github.com/dor1202/lazyregex/internal/engine"
	"github.com/dor1202/lazyregex/internal/engine/protocol"
	"github.com/dor1202/lazyregex/internal/engine/wasmhost"
)

// Built by `make wasm-js` from wasm/quickjs (see wasm/versions.env).
//
//go:embed qjs.wasm.gz
var moduleGZ []byte

// Mounted in the guest at /shim.
//
//go:embed shim.js
var shimFS embed.FS

type jsEngine struct{ host *wasmhost.Host }

// New starts the JavaScript engine. cacheDir, if non-empty, holds wazero's
// compilation cache. Matching follows ECMAScript: match mode is
// RegExp.prototype.exec (String.prototype.matchAll with the g flag), replace
// is String.prototype.replace and split String.prototype.split.
func New(ctx context.Context, cacheDir string) (engine.Engine, error) {
	h, err := wasmhost.New(ctx, wasmhost.Spec{
		Name:     "qjs",
		ModuleGZ: moduleGZ,
		Args:     []string{"qjs", "--std", "/shim/shim.js"},
		Mounts:   map[string]fs.FS{"/shim": shimFS},
	}, cacheDir)
	if err != nil {
		return nil, err
	}
	return &jsEngine{host: h}, nil
}

func (e *jsEngine) Info() engine.Info {
	return engine.Info{
		Flavor:     engine.JS,
		EngineName: "QuickJS-ng",
		Version:    e.host.Version(),
		Flags: []engine.FlagInfo{
			{Letter: 'g', Name: "global", Default: true},
			{Letter: 'd', Name: "match indices"},
			{Letter: 'i', Name: "case-insensitive"},
			{Letter: 'm', Name: "multi-line"},
			{Letter: 's', Name: "dot matches newline"},
			{Letter: 'u', Name: "unicode"},
			{Letter: 'v', Name: "unicode sets"},
			{Letter: 'y', Name: "sticky"},
		},
	}
}

func (e *jsEngine) Eval(ctx context.Context, req engine.Request) (engine.Result, error) {
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
	return protocol.ToResult(resp, engine.UnitUTF16)
}

func (e *jsEngine) Close() error { return e.host.Close() }
