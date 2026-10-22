// Package lazyregex only carries the licence texts so the command can embed
// them (go:embed cannot reach outside a package directory).
package lazyregex

import "embed"

// Own is lazyregex's own licence.
//
//go:embed LICENSE
var Own string

// Third holds the licences of the bundled third-party code (the WASM engines
// and the wasi-libc they are linked against), in LICENSES/.
//
//go:embed LICENSES/*.txt
var Third embed.FS
