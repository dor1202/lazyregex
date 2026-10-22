package all

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/dor1202/lazyregex/internal/engine"
)

func TestAllEnginesAgreeOnSimpleCase(t *testing.T) {
	cache := t.TempDir()
	f := Factories()
	if len(f) != len(engine.Flavors) {
		t.Fatalf("got %d factories, want %d", len(f), len(engine.Flavors))
	}
	want := []engine.Span{{Start: 1, End: 2}, {Start: 3, End: 5}}
	for _, fl := range engine.Flavors {
		t.Run(string(fl), func(t *testing.T) {
			factory, ok := f[fl]
			if !ok {
				t.Fatal("no factory")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()
			e, err := factory(ctx, cache)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = e.Close() }()
			res, err := e.Eval(ctx, engine.Request{Pattern: `\d+`, Flags: "g", Text: "a1b22", Mode: engine.ModeMatch})
			if err != nil {
				t.Fatal(err)
			}
			var got []engine.Span
			for _, m := range res.Matches {
				got = append(got, m.Span)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %v, want %v", got, want)
			}
		})
	}
}
