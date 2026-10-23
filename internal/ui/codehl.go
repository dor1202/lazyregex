package ui

import (
	"github.com/dor1202/lazyregex/internal/engine"
	"github.com/dor1202/lazyregex/internal/syntax"
)

// codeLang is the language of each flavor's snippet.
var codeLang = map[engine.Flavor]string{
	engine.Python: syntax.Python,
	engine.JS:     syntax.JavaScript,
	engine.PCRE:   syntax.PHP,
	engine.Go:     syntax.Go,
}

// codeStyle is the Code tab's chroma style for the theme's light/dark
// decision.
func codeStyle(t *Theme) string {
	if t.dark {
		return syntax.Dark
	}
	return syntax.Light
}

// codeHighlight caches the highlighted lines of one (snippet, style).
type codeHighlight struct {
	code, style string
	lines       []string
}
