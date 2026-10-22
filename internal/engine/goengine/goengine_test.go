package goengine

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/dor1202/lazyregex/internal/engine"
)

func eval(t *testing.T, req engine.Request) (engine.Result, error) {
	t.Helper()
	return New().Eval(context.Background(), req)
}

func TestGoEngineFindAll(t *testing.T) {
	res, err := eval(t, engine.Request{Pattern: `(?P<y>\d{4})-(\d\d)`, Flags: "g", Text: "2026-10 and 1999-01", Mode: engine.ModeMatch})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 2 {
		t.Fatalf("matches = %d", len(res.Matches))
	}
	m := res.Matches[0]
	if m.Span != (engine.Span{Start: 0, End: 7}) {
		t.Errorf("span = %v", m.Span)
	}
	want := []engine.Group{{Index: 1, Name: "y", Matched: true, Span: engine.Span{Start: 0, End: 4}}, {Index: 2, Matched: true, Span: engine.Span{Start: 5, End: 7}}}
	if !reflect.DeepEqual(m.Groups, want) {
		t.Errorf("groups = %v", m.Groups)
	}
	if res.Unit != engine.UnitBytes {
		t.Errorf("unit = %v", res.Unit)
	}
}

func TestGoEngineNonGlobal(t *testing.T) {
	res, err := eval(t, engine.Request{Pattern: `(?P<y>\d{4})-(\d\d)`, Text: "2026-10 and 1999-01", Mode: engine.ModeMatch})
	if err != nil || len(res.Matches) != 1 {
		t.Fatalf("err=%v matches=%d", err, len(res.Matches))
	}
}

func TestGoEngineUnmatchedGroup(t *testing.T) {
	res, err := eval(t, engine.Request{Pattern: `(a)?(b)`, Text: "b", Mode: engine.ModeMatch})
	if err != nil {
		t.Fatal(err)
	}
	g := res.Matches[0].Groups
	if g[0].Matched || !g[1].Matched {
		t.Errorf("groups = %v", g)
	}
}

func TestGoEngineCompileError(t *testing.T) {
	_, err := eval(t, engine.Request{Pattern: `(?<=\$)\d+`, Flags: "i", Text: "x", Mode: engine.ModeMatch})
	var ce *engine.CompileError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v", err)
	}
	if want := "error parsing regexp: invalid named capture: `(?<=\\$)\\d+`"; ce.Message != want {
		t.Errorf("message = %q", ce.Message)
	}
	if ce.Offset != 0 {
		t.Errorf("offset = %d", ce.Offset)
	}
}

func TestGoEngineReplace(t *testing.T) {
	req := engine.Request{Pattern: `(\w+)@(\w+)`, Flags: "g", Text: "a@b c@d", Mode: engine.ModeReplace, Replacement: "$2 at $1"}
	res, err := eval(t, req)
	if err != nil || res.Output != "b at a d at c" {
		t.Fatalf("global: %q %v", res.Output, err)
	}
	req.Flags = ""
	res, err = eval(t, req)
	if err != nil || res.Output != "b at a c@d" {
		t.Fatalf("first only: %q %v", res.Output, err)
	}
}

func TestGoEngineSplit(t *testing.T) {
	res, err := eval(t, engine.Request{Pattern: `,\s*`, Text: "a, b,c", Mode: engine.ModeSplit})
	if err != nil || !reflect.DeepEqual(res.Parts, []string{"a", "b", "c"}) {
		t.Fatalf("%v %v", res.Parts, err)
	}
}

func TestGoEngineFlags(t *testing.T) {
	res, err := eval(t, engine.Request{Pattern: `abc`, Flags: "i", Text: "ABC", Mode: engine.ModeMatch})
	if err != nil || len(res.Matches) != 1 {
		t.Fatalf("%v %v", res.Matches, err)
	}
}

func TestGoEngineCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New().Eval(ctx, engine.Request{Pattern: `a`, Text: "a"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}
