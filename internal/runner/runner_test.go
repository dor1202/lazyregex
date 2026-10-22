package runner

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dor1202/lazyregex/internal/engine"
	"github.com/dor1202/lazyregex/internal/engine/goengine"
)

// fakeEngine sleeps for the duration in req.Pattern (time.ParseDuration) and
// honours ctx.
type fakeEngine struct {
	f      engine.Flavor
	closed atomic.Bool
}

func (e *fakeEngine) Info() engine.Info { return engine.Info{Flavor: e.f, EngineName: "fake"} }
func (e *fakeEngine) Close() error      { e.closed.Store(true); return nil }
func (e *fakeEngine) Eval(ctx context.Context, req engine.Request) (engine.Result, error) {
	d, _ := time.ParseDuration(req.Pattern)
	select {
	case <-time.After(d):
		return engine.Result{Output: req.Pattern}, nil
	case <-ctx.Done():
		return engine.Result{}, ctx.Err()
	}
}

func fakeFactory(f engine.Flavor, delay time.Duration, out **fakeEngine) Factory {
	return func(ctx context.Context, _ string) (engine.Engine, error) {
		time.Sleep(delay)
		e := &fakeEngine{f: f}
		if out != nil {
			*out = e
		}
		return e, nil
	}
}

func newFake(timeout time.Duration, fl ...engine.Flavor) *Runner {
	m := map[engine.Flavor]Factory{}
	for _, f := range fl {
		m[f] = fakeFactory(f, 0, nil)
	}
	return New(m, t0, timeout)
}

const t0 = ""

func waitState(t *testing.T, r *Runner, f engine.Flavor, s State) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for r.State(f) != s {
		if time.Now().After(deadline) {
			t.Fatalf("state of %s = %v, want %v", f, r.State(f), s)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestRunnerLazyLoad(t *testing.T) {
	r := newFake(time.Second, engine.Python, engine.JS)
	defer r.Close()
	if r.State(engine.Python) != NotLoaded {
		t.Fatal("expected NotLoaded")
	}
	if _, ok := r.Info(engine.Python); ok {
		t.Fatal("Info before load")
	}
	r.Load(engine.Python)
	waitState(t, r, engine.Python, Ready)
	if i, ok := r.Info(engine.Python); !ok || i.Flavor != engine.Python {
		t.Fatalf("info %v %v", i, ok)
	}
	if r.State(engine.JS) != NotLoaded {
		t.Fatal("JS should still be NotLoaded")
	}
	if _, err := r.Eval(engine.JS, engine.Request{Pattern: "0s"}); err != nil {
		t.Fatal(err)
	}
	if r.State(engine.JS) != Ready {
		t.Fatal("Eval should load")
	}
}

func TestRunnerWarmUp(t *testing.T) {
	r := newFake(time.Second, engine.Python, engine.JS, engine.PCRE)
	defer r.Close()
	r.WarmUp(engine.Python)
	waitState(t, r, engine.JS, Ready)
	waitState(t, r, engine.PCRE, Ready)
	if r.State(engine.Python) != NotLoaded {
		t.Fatal("excepted flavor loaded")
	}
}

func TestRunnerCancelsSupersededEval(t *testing.T) {
	r := newFake(5*time.Second, engine.Python)
	defer r.Close()
	r.Load(engine.Python)
	waitState(t, r, engine.Python, Ready)
	errc := make(chan error, 1)
	go func() {
		_, err := r.Eval(engine.Python, engine.Request{Pattern: "1s"})
		errc <- err
	}()
	time.Sleep(50 * time.Millisecond)
	res, err := r.Eval(engine.Python, engine.Request{Pattern: "10ms"})
	if err != nil || res.Output != "10ms" {
		t.Fatalf("second: %v %v", res, err)
	}
	select {
	case err := <-errc:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("first err = %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("first eval not cancelled")
	}
}

func collect(r *Runner, f engine.Flavor, n int) <-chan []State {
	out := make(chan []State, 1)
	go func() {
		var got []State
		timeout := time.After(2 * time.Second)
		for len(got) < n {
			select {
			case ev := <-r.Events():
				if ev.Flavor == f {
					got = append(got, ev.State)
				}
			case <-timeout:
				out <- got
				return
			}
		}
		out <- got
	}()
	return out
}

func TestRunnerTimeout(t *testing.T) {
	r := newFake(50*time.Millisecond, engine.PCRE)
	defer r.Close()
	ev := collect(r, engine.PCRE, 4) // Loading, Ready, Restarting, Ready
	start := time.Now()
	_, err := r.Eval(engine.PCRE, engine.Request{Pattern: "1s"})
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v", err)
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Fatal("timeout too slow")
	}
	got := <-ev
	want := []State{Loading, Ready, Restarting, Ready}
	if len(got) != 4 {
		t.Fatalf("events %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("events %v, want %v", got, want)
		}
	}
	if r.State(engine.PCRE) != Ready {
		t.Fatal("not Ready after timeout")
	}
}

func TestRunnerEvalTimeout(t *testing.T) {
	r := newFake(10*time.Second, engine.Python)
	defer r.Close()
	_, err := r.EvalTimeout(engine.Python, engine.Request{Pattern: "1s"}, 30*time.Millisecond)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v", err)
	}
	// A longer explicit timeout overrides a short default.
	r2 := newFake(10*time.Millisecond, engine.Python)
	defer r2.Close()
	if _, err := r2.EvalTimeout(engine.Python, engine.Request{Pattern: "50ms"}, time.Second); err != nil {
		t.Fatal(err)
	}
	// Supersession still applies.
	errc := make(chan error, 1)
	go func() {
		_, err := r.EvalTimeout(engine.Python, engine.Request{Pattern: "1s"}, 5*time.Second)
		errc <- err
	}()
	time.Sleep(50 * time.Millisecond)
	if _, err := r.EvalTimeout(engine.Python, engine.Request{Pattern: "0s"}, time.Second); err != nil {
		t.Fatal(err)
	}
	if err := <-errc; !errors.Is(err, context.Canceled) {
		t.Fatalf("superseded err = %v", err)
	}
}

func TestRunnerEvalAllOnlyReady(t *testing.T) {
	r := newFake(time.Second, engine.Python, engine.JS)
	defer r.Close()
	r.Load(engine.Python)
	waitState(t, r, engine.Python, Ready)
	out := r.EvalAll(map[engine.Flavor]engine.Request{
		engine.Python: {Pattern: "0s"},
		engine.JS:     {Pattern: "0s"},
	})
	if len(out) != 1 {
		t.Fatalf("got %v", out)
	}
	if o, ok := out[engine.Python]; !ok || o.Err != nil {
		t.Fatalf("python outcome %+v", o)
	}
}

func TestRunnerLoadFailure(t *testing.T) {
	boom := errors.New("boom")
	r := New(map[engine.Flavor]Factory{
		engine.JS: func(context.Context, string) (engine.Engine, error) { return nil, boom },
	}, t0, time.Second)
	defer r.Close()
	r.Load(engine.JS)
	waitState(t, r, engine.JS, Failed)
	if !errors.Is(r.Err(engine.JS), boom) {
		t.Fatalf("Err = %v", r.Err(engine.JS))
	}
	if _, err := r.Eval(engine.JS, engine.Request{}); !errors.Is(err, boom) {
		t.Fatalf("Eval err = %v", err)
	}
	if _, err := r.Eval(engine.Python, engine.Request{}); err == nil {
		t.Fatal("unknown flavor should error")
	}
}

func TestRunnerEventsNeverBlock(t *testing.T) {
	r := newFake(20*time.Millisecond, engine.Python)
	defer r.Close()
	done := make(chan struct{})
	go func() {
		for i := 0; i < 60; i++ {
			_, _ = r.Eval(engine.Python, engine.Request{Pattern: "1s"}) // times out each time
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("runner blocked on undrained events")
	}
}

func TestRunnerCloseCancelsAndClosesEngines(t *testing.T) {
	var eng *fakeEngine
	r := New(map[engine.Flavor]Factory{engine.Python: fakeFactory(engine.Python, 0, &eng)}, t0, 5*time.Second)
	r.Load(engine.Python)
	waitState(t, r, engine.Python, Ready)
	errc := make(chan error, 1)
	go func() {
		_, err := r.Eval(engine.Python, engine.Request{Pattern: "10s"})
		errc <- err
	}()
	time.Sleep(50 * time.Millisecond)
	r.Close()
	if err := <-errc; err == nil {
		t.Fatal("in-flight eval should fail")
	}
	if !eng.closed.Load() {
		t.Fatal("engine not closed")
	}
	if _, err := r.Eval(engine.Python, engine.Request{Pattern: "0s"}); err == nil {
		t.Fatal("Eval after Close should fail")
	}
	r.Close() // idempotent
}

func TestRunnerCloseWaitsForLoad(t *testing.T) {
	var eng *fakeEngine
	r := New(map[engine.Flavor]Factory{engine.Go: fakeFactory(engine.Go, 100*time.Millisecond, &eng)}, t0, time.Second)
	r.Load(engine.Go)
	time.Sleep(10 * time.Millisecond)
	r.Close()
	if eng == nil || !eng.closed.Load() {
		t.Fatal("engine loaded during Close must be closed")
	}
}

func TestRunnerEmptyMatchesTerminate(t *testing.T) {
	r := New(map[engine.Flavor]Factory{
		engine.Go: func(context.Context, string) (engine.Engine, error) { return goengine.New(), nil },
	}, t0, time.Second)
	defer r.Close()
	start := time.Now()
	res, err := r.Eval(engine.Go, engine.Request{Pattern: "a*", Flags: "g", Text: "baaa", Mode: engine.ModeMatch})
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 100*time.Millisecond {
		t.Fatalf("too slow: %v", time.Since(start))
	}
	if len(res.Matches) != 2 || res.Matches[0].Span != (engine.Span{Start: 0, End: 0}) || res.Matches[1].Span != (engine.Span{Start: 1, End: 4}) {
		t.Fatalf("matches %+v", res.Matches)
	}
}

func TestRunnerEvalAllIndependentOfEval(t *testing.T) {
	r := newFake(5*time.Second, engine.Python)
	defer r.Close()
	r.Load(engine.Python)
	waitState(t, r, engine.Python, Ready)
	mainc := make(chan error, 1)
	go func() {
		_, err := r.Eval(engine.Python, engine.Request{Pattern: "200ms"})
		mainc <- err
	}()
	time.Sleep(30 * time.Millisecond)
	out := r.EvalAll(map[engine.Flavor]engine.Request{engine.Python: {Pattern: "100ms"}})
	if o := out[engine.Python]; o.Err != nil {
		t.Fatalf("EvalAll err = %v", o.Err)
	}
	if err := <-mainc; err != nil {
		t.Fatalf("main Eval err = %v", err)
	}
	// And a main Eval started during EvalAll does not cancel it.
	allc := make(chan map[engine.Flavor]Outcome, 1)
	go func() { allc <- r.EvalAll(map[engine.Flavor]engine.Request{engine.Python: {Pattern: "200ms"}}) }()
	time.Sleep(30 * time.Millisecond)
	if _, err := r.Eval(engine.Python, engine.Request{Pattern: "10ms"}); err != nil {
		t.Fatal(err)
	}
	if o := (<-allc)[engine.Python]; o.Err != nil {
		t.Fatalf("EvalAll cancelled by Eval: %v", o.Err)
	}
}
