package pcre

import (
	"context"
	"errors"
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
	t.Cleanup(func() { e.Close() })
	return e
}

func TestPCREPing(t *testing.T) {
	info := newEngine(t).Info()
	if !strings.HasPrefix(info.Version, "10.") {
		t.Errorf("Version = %q, want 10.x", info.Version)
	}
	if info.Flavor != engine.PCRE || info.EngineName != "PCRE2" {
		t.Errorf("info = %+v", info)
	}
}

func TestPCRELookbehind(t *testing.T) {
	res, err := newEngine(t).Eval(context.Background(), engine.Request{
		Pattern: `(?<=\$)\d+(\.\d{2})?`, Flags: "gu", Text: "Total: $42.50, tax $3.10", Mode: engine.ModeMatch,
	})
	if err != nil {
		t.Fatal(err)
	}
	var spans []engine.Span
	for _, m := range res.Matches {
		spans = append(spans, m.Span)
	}
	want := []engine.Span{{Start: 8, End: 13}, {Start: 20, End: 24}}
	if !reflect.DeepEqual(spans, want) {
		t.Errorf("spans = %v, want %v", spans, want)
	}
	if res.Unit != engine.UnitBytes {
		t.Errorf("unit = %v", res.Unit)
	}
}

func TestPCRECompileErrorOffsetInUserPattern(t *testing.T) {
	// PHP sees "a\/(" (the '/' is escaped for the delimiter); the message
	// keeps PHP's offset, the structured offset points into the user's text.
	_, err := newEngine(t).Eval(context.Background(), engine.Request{Pattern: `a/(`, Mode: engine.ModeMatch})
	var ce *engine.CompileError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v", err)
	}
	if ce.Message != "Compilation failed: missing closing parenthesis at offset 4" || ce.Offset != 3 {
		t.Errorf("got %q offset %d", ce.Message, ce.Offset)
	}
}

func TestPCRECompileErrorOffsetAfterEscapedSlash(t *testing.T) {
	// The user's "\/" is already escaped and kept; only the bare '/' gets a
	// backslash, so PHP sees "\/a\/(" and the error offset 6 maps back to 5.
	_, err := newEngine(t).Eval(context.Background(), engine.Request{Pattern: `\/a/(`, Mode: engine.ModeMatch})
	var ce *engine.CompileError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v", err)
	}
	if ce.Message != "Compilation failed: missing closing parenthesis at offset 6" || ce.Offset != 5 {
		t.Errorf("got %q offset %d", ce.Message, ce.Offset)
	}
}

func TestParityPCRE(t *testing.T) {
	parity.RunParity(t, engine.PCRE, newEngine(t))
}
