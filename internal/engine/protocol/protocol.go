// Package protocol defines the newline-delimited JSON wire format spoken
// between the host and the engine modules (one JSON object per line).
package protocol

import (
	"github.com/dor1202/lazyregex/internal/engine"
)

// Operations.
const (
	OpEval = "eval"
	OpPing = "ping"
)

// Req is one request line.
type Req struct {
	ID          int    `json:"id"`
	Op          string `json:"op"`
	Pattern     string `json:"pattern,omitempty"`
	Flags       string `json:"flags,omitempty"`
	Text        string `json:"text,omitempty"`
	Mode        string `json:"mode,omitempty"`
	Replacement string `json:"replacement,omitempty"`
}

// Error is an engine-reported failure (an invalid pattern).
type Error struct {
	Message string `json:"message"`
	Offset  int    `json:"offset"`
}

// Group is one capture group of a match (group 0 excluded).
type Group struct {
	Index   int    `json:"index"`
	Name    string `json:"name"`
	Matched bool   `json:"matched"`
	Span    [2]int `json:"span"`
}

// Match is one successful match.
type Match struct {
	Span   [2]int  `json:"span"`
	Groups []Group `json:"groups"`
}

// Resp is one response line.
type Resp struct {
	ID       int      `json:"id"`
	OK       bool     `json:"ok"`
	Error    *Error   `json:"error,omitempty"`
	Matches  []Match  `json:"matches,omitempty"`
	Output   string   `json:"output,omitempty"`
	Parts    []string `json:"parts,omitempty"`
	PartsNil []bool   `json:"parts_nil,omitempty"`
	Version  string   `json:"version,omitempty"` // on "ping"
}

// ToResult converts a response into an engine.Result in the given unit.
// A response carrying Error yields a *engine.CompileError.
func ToResult(r Resp, unit engine.Unit) (engine.Result, error) {
	if r.Error != nil {
		return engine.Result{}, &engine.CompileError{Message: r.Error.Message, Offset: r.Error.Offset}
	}
	res := engine.Result{
		Output:   r.Output,
		Parts:    r.Parts,
		PartsNil: r.PartsNil,
		Unit:     unit,
	}
	for _, m := range r.Matches {
		em := engine.Match{Span: engine.Span{Start: m.Span[0], End: m.Span[1]}}
		for _, g := range m.Groups {
			em.Groups = append(em.Groups, engine.Group{
				Index: g.Index, Name: g.Name, Matched: g.Matched,
				Span: engine.Span{Start: g.Span[0], End: g.Span[1]},
			})
		}
		res.Matches = append(res.Matches, em)
	}
	return res, nil
}
