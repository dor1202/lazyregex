package parity_test

import (
	"testing"

	"github.com/dor1202/lazyregex/internal/engine"
	"github.com/dor1202/lazyregex/internal/engine/goengine"
	"github.com/dor1202/lazyregex/parity"
)

func TestParityGo(t *testing.T) {
	parity.RunParity(t, engine.Go, goengine.New())
}
