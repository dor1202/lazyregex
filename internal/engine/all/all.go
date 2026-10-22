// Package all collects the constructors of every bundled regex engine.
package all

import (
	"context"

	"github.com/dor1202/lazyregex/internal/engine"
	"github.com/dor1202/lazyregex/internal/engine/goengine"
	"github.com/dor1202/lazyregex/internal/engine/js"
	"github.com/dor1202/lazyregex/internal/engine/pcre"
	"github.com/dor1202/lazyregex/internal/engine/python"
	"github.com/dor1202/lazyregex/internal/runner"
)

// Factories returns the Go, PCRE, JS and Python engine constructors.
func Factories() map[engine.Flavor]runner.Factory {
	return map[engine.Flavor]runner.Factory{
		engine.Go:     func(context.Context, string) (engine.Engine, error) { return goengine.New(), nil },
		engine.PCRE:   pcre.New,
		engine.JS:     js.New,
		engine.Python: python.New,
	}
}
