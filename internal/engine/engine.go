// Package engine defines the types shared by every regex flavor adapter, the
// evaluation runner and the UI.
package engine

import "context"

// Flavor identifies a regex dialect.
type Flavor string

// Supported flavors.
const (
	Python Flavor = "python"
	JS     Flavor = "js"
	PCRE   Flavor = "pcre"
	Go     Flavor = "go"
)

// Flavors lists every flavor in panel order.
var Flavors = []Flavor{Python, JS, PCRE, Go}

// Mode is the kind of evaluation requested.
type Mode string

// Evaluation modes.
const (
	ModeMatch   Mode = "match"
	ModeReplace Mode = "replace"
	ModeSplit   Mode = "split"
)

// Unit is the unit in which a flavor reports match offsets.
type Unit int

// Offset units: Python uses code points, JS UTF-16 code units, PCRE and Go bytes.
const (
	UnitBytes Unit = iota
	UnitCodePoints
	UnitUTF16
)

// Span is a half-open range [Start, End) in the engine's native Unit.
type Span struct{ Start, End int }

// Group describes one capture group of a match.
type Group struct {
	Index   int
	Name    string // "" if unnamed
	Matched bool   // false => Span is meaningless
	Span    Span
}

// Match is one successful match. Groups excludes group 0, which is Span.
type Match struct {
	Span   Span
	Groups []Group
}

// Request is a single evaluation.
type Request struct {
	Pattern     string
	Flags       string // flavor flag letters, e.g. "gm"; "g" is lazyregex's global
	Text        string
	Mode        Mode
	Replacement string // replace mode
}

// Result is the outcome of an evaluation.
type Result struct {
	Matches []Match
	Output  string   // replace result
	Parts   []string // split result
	// PartsNil is parallel to Parts: true where the native value was
	// None/undefined (an unmatched optional group in a Python split).
	PartsNil []bool
	Unit     Unit
}

// CompileError reports an invalid pattern.
type CompileError struct {
	Message string // verbatim engine message
	Offset  int    // position in the pattern, -1 if the engine gives none
}

// Error implements error.
func (e *CompileError) Error() string { return e.Message }

// Info describes an engine for display.
type Info struct {
	Flavor     Flavor
	EngineName string
	Version    string
	Flags      []FlagInfo
}

// FlagInfo describes one flag an engine supports.
type FlagInfo struct {
	Letter  byte
	Name    string
	Default bool
}

// Engine evaluates requests against one flavor's real regex implementation.
type Engine interface {
	Info() Info
	// Eval returns a *CompileError for bad patterns and ctx.Err() on
	// cancellation or timeout.
	Eval(ctx context.Context, req Request) (Result, error)
	Close() error
}
