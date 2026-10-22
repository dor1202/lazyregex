// Package pcre adapts the PCRE2 C library, compiled to a WASI module with a
// shim that reproduces PHP 8.4's preg_* functions, to engine.Engine.
package pcre

import (
	"context"
	_ "embed"

	"github.com/dor1202/lazyregex/internal/engine"
	"github.com/dor1202/lazyregex/internal/engine/protocol"
	"github.com/dor1202/lazyregex/internal/engine/wasmhost"
)

// Built by `make wasm-pcre` from wasm/pcre (see wasm/versions.env).
//
//go:embed pcre.wasm.gz
var moduleGZ []byte

type pcreEngine struct{ host *wasmhost.Host }

// New starts the PCRE engine. cacheDir, if non-empty, holds wazero's
// compilation cache. Matching follows PHP: the pattern is used as
// '/'.$pattern.'/'.$modifiers with every '/' not already escaped given a
// backslash, match mode is preg_match/preg_match_all, replace is
// preg_replace and split preg_split.
func New(ctx context.Context, cacheDir string) (engine.Engine, error) {
	h, err := wasmhost.New(ctx, wasmhost.Spec{Name: "pcre", ModuleGZ: moduleGZ}, cacheDir)
	if err != nil {
		return nil, err
	}
	return &pcreEngine{host: h}, nil
}

func (e *pcreEngine) Info() engine.Info {
	return engine.Info{
		Flavor:     engine.PCRE,
		EngineName: "PCRE2",
		Version:    e.host.Version(),
		Flags: []engine.FlagInfo{
			{Letter: 'g', Name: "global", Default: true},
			{Letter: 'i', Name: "case-insensitive"},
			{Letter: 'm', Name: "multi-line"},
			{Letter: 's', Name: "dot matches newline"},
			{Letter: 'x', Name: "extended"},
			{Letter: 'n', Name: "no auto-capture"},
			{Letter: 'U', Name: "ungreedy"},
			{Letter: 'u', Name: "UTF-8 and Unicode properties", Default: true},
		},
	}
}

func (e *pcreEngine) Eval(ctx context.Context, req engine.Request) (engine.Result, error) {
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
	return protocol.ToResult(resp, engine.UnitBytes)
}

func (e *pcreEngine) Close() error { return e.host.Close() }
