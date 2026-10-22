// Package goengine adapts Go's standard library regexp package to engine.Engine.
package goengine

import (
	"context"
	"errors"
	"regexp"
	"regexp/syntax"
	"runtime"
	"strings"

	"github.com/dor1202/lazyregex/internal/engine"
)

type goEngine struct{}

// New returns an engine backed by Go's regexp package. Supported flags are
// g (global, on by default), i, m, s and U.
func New() engine.Engine { return goEngine{} }

func (goEngine) Info() engine.Info {
	return engine.Info{
		Flavor:     engine.Go,
		EngineName: "Go regexp (RE2)",
		Version:    runtime.Version(),
		Flags: []engine.FlagInfo{
			{Letter: 'g', Name: "global", Default: true},
			{Letter: 'i', Name: "case-insensitive"},
			{Letter: 'm', Name: "multi-line"},
			{Letter: 's', Name: "dot matches newline"},
			{Letter: 'U', Name: "ungreedy"},
		},
	}
}

func (goEngine) Close() error { return nil }

func (goEngine) Eval(ctx context.Context, req engine.Request) (engine.Result, error) {
	if err := ctx.Err(); err != nil {
		return engine.Result{}, err
	}
	re, err := compile(req.Pattern, req.Flags)
	if err != nil {
		return engine.Result{}, err
	}
	global := strings.Contains(req.Flags, "g")
	res := engine.Result{Unit: engine.UnitBytes}

	switch req.Mode {
	case engine.ModeReplace:
		if global {
			res.Output = re.ReplaceAllString(req.Text, req.Replacement)
		} else if loc := re.FindStringSubmatchIndex(req.Text); loc != nil {
			repl := re.ExpandString(nil, req.Replacement, req.Text, loc)
			res.Output = req.Text[:loc[0]] + string(repl) + req.Text[loc[1]:]
		} else {
			res.Output = req.Text
		}
	case engine.ModeSplit:
		res.Parts = re.Split(req.Text, -1)
		res.PartsNil = make([]bool, len(res.Parts))
	default:
		var all [][]int
		if global {
			all = re.FindAllStringSubmatchIndex(req.Text, -1)
		} else if loc := re.FindStringSubmatchIndex(req.Text); loc != nil {
			all = [][]int{loc}
		}
		names := re.SubexpNames()
		for _, loc := range all {
			m := engine.Match{Span: engine.Span{Start: loc[0], End: loc[1]}}
			for i := 1; i < len(loc)/2; i++ {
				g := engine.Group{Index: i, Name: names[i]}
				if loc[2*i] >= 0 {
					g.Matched = true
					g.Span = engine.Span{Start: loc[2*i], End: loc[2*i+1]}
				}
				m.Groups = append(m.Groups, g)
			}
			res.Matches = append(res.Matches, m)
		}
	}
	return res, ctx.Err()
}

// compile validates the user's pattern verbatim first so error messages never
// mention the internal inline-flag prefix, then compiles with the flags applied.
func compile(pattern, flags string) (*regexp.Regexp, error) {
	if _, err := regexp.Compile(pattern); err != nil {
		return nil, compileError(pattern, err)
	}
	var inline strings.Builder
	for _, f := range "imsU" {
		if strings.ContainsRune(flags, f) {
			inline.WriteRune(f)
		}
	}
	if inline.Len() > 0 {
		pattern = "(?" + inline.String() + ")" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, compileError(pattern, err)
	}
	return re, nil
}

func compileError(pattern string, err error) *engine.CompileError {
	ce := &engine.CompileError{Message: err.Error(), Offset: -1}
	var se *syntax.Error
	if errors.As(err, &se) && se.Expr != "" {
		ce.Offset = strings.Index(pattern, se.Expr)
	}
	return ce
}
