package js

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/dor1202/lazyregex/internal/engine"
	"github.com/dor1202/lazyregex/parity"
)

func newEngine(t *testing.T) engine.Engine {
	t.Helper()
	e, err := New(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })
	return e
}

func TestJSPing(t *testing.T) {
	info := newEngine(t).Info()
	if !strings.HasPrefix(info.Version, "0.") {
		t.Errorf("Version = %q, want 0.x", info.Version)
	}
	if info.Flavor != engine.JS || info.EngineName != "QuickJS-ng" {
		t.Errorf("info = %+v", info)
	}
	var letters string
	for _, f := range info.Flags {
		letters += string(f.Letter)
		if f.Default != (f.Letter == 'g') {
			t.Errorf("flag %c default = %v", f.Letter, f.Default)
		}
	}
	if letters != "gdimsuvy" {
		t.Errorf("flags = %q, want gdimsuvy", letters)
	}
}

func TestJSUnicodeIndices(t *testing.T) {
	res, err := newEngine(t).Eval(context.Background(), engine.Request{
		Pattern: `.`, Flags: "gu", Text: "a😀b", Mode: engine.ModeMatch,
	})
	if err != nil {
		t.Fatal(err)
	}
	var spans []engine.Span
	for _, m := range res.Matches {
		spans = append(spans, m.Span)
	}
	want := []engine.Span{{Start: 0, End: 1}, {Start: 1, End: 3}, {Start: 3, End: 4}}
	if !reflect.DeepEqual(spans, want) {
		t.Errorf("spans = %v, want %v", spans, want)
	}
	if res.Unit != engine.UnitUTF16 {
		t.Errorf("unit = %v", res.Unit)
	}
}

func TestJSNamedGroup(t *testing.T) {
	res, err := newEngine(t).Eval(context.Background(), engine.Request{
		Pattern: `(?<y>\d{4})`, Flags: "g", Text: "in 2024", Mode: engine.ModeMatch,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 1 || len(res.Matches[0].Groups) != 1 {
		t.Fatalf("matches = %+v", res.Matches)
	}
	g := res.Matches[0].Groups[0]
	if g.Name != "y" || g.Index != 1 || g.Span != (engine.Span{Start: 3, End: 7}) {
		t.Errorf("group = %+v", g)
	}
}

func TestJSUserDFlag(t *testing.T) {
	res, err := newEngine(t).Eval(context.Background(), engine.Request{
		Pattern: `b`, Flags: "gd", Text: "abcb", Mode: engine.ModeMatch,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 2 {
		t.Errorf("matches = %+v", res.Matches)
	}
}

func TestParityJS(t *testing.T) {
	parity.RunParity(t, engine.JS, newEngine(t))
}
