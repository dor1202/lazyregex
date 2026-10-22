// Package runner owns the regex engines for the UI: it loads them lazily in
// the background, warms the others up, cancels superseded evaluations,
// enforces timeouts and reports state changes.
package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dor1202/lazyregex/internal/engine"
)

// State is the lifecycle state of one flavor's engine.
type State int

// Engine states.
const (
	NotLoaded State = iota
	Loading
	Ready
	Restarting
	Failed
)

func (s State) String() string {
	return [...]string{"not loaded", "loading", "ready", "restarting", "failed"}[s]
}

// Event reports a state change of one flavor.
type Event struct {
	Flavor engine.Flavor
	State  State
}

// Outcome is the result of one flavor in EvalAll.
type Outcome struct {
	Result engine.Result
	Err    error
}

// Factory creates an engine (typically one of the packages' New).
type Factory func(ctx context.Context, cacheDir string) (engine.Engine, error)

// Errors returned by the runner.
var (
	ErrTimeout = errors.New("timed out — possible catastrophic backtracking")
	ErrClosed  = errors.New("runner closed")
)

// eventBuffer is the capacity of the Events channel.
const eventBuffer = 64

// DefaultCacheDir returns <user cache dir>/lazyregex/wazero ("" if the user
// cache directory is unknown, which disables compilation caching).
func DefaultCacheDir() string {
	d, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(d, "lazyregex", "wazero")
}

type slot struct {
	factory   Factory
	state     State
	eng       engine.Engine
	info      engine.Info
	err       error
	ready     chan struct{}      // closed when loading finishes (success or failure)
	cancel    context.CancelFunc // cancels the in-flight Eval, if any
	allCancel context.CancelFunc // cancels the in-flight EvalAll entry, if any
}

// Runner manages one engine per flavor. All methods are safe for concurrent
// use.
type Runner struct {
	cacheDir string
	timeout  time.Duration

	ctx    context.Context
	cancel context.CancelFunc

	mu     sync.Mutex
	slots  map[engine.Flavor]*slot
	closed bool
	loads  sync.WaitGroup
	evals  sync.WaitGroup

	evMu     sync.Mutex
	evClosed bool
	events   chan Event
}

// New creates a Runner. timeout is the default per-evaluation timeout; an
// empty cacheDir selects DefaultCacheDir.
func New(factories map[engine.Flavor]Factory, cacheDir string, timeout time.Duration) *Runner {
	if cacheDir == "" {
		cacheDir = DefaultCacheDir()
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &Runner{
		cacheDir: cacheDir,
		timeout:  timeout,
		ctx:      ctx,
		cancel:   cancel,
		slots:    map[engine.Flavor]*slot{},
		events:   make(chan Event, eventBuffer),
	}
	for f, fac := range factories {
		r.slots[f] = &slot{factory: fac, ready: make(chan struct{})}
	}
	return r
}

// Events returns the state-change channel. The runner never blocks on it:
// the channel is buffered and, when the consumer falls behind, the oldest
// event is dropped to make room. Consumers that need the truth should call
// State. The channel is closed by Close.
func (r *Runner) Events() <-chan Event { return r.events }

func (r *Runner) emit(f engine.Flavor, s State) {
	r.evMu.Lock()
	defer r.evMu.Unlock()
	if r.evClosed {
		return
	}
	ev := Event{f, s}
	for {
		select {
		case r.events <- ev:
			return
		default:
		}
		select {
		case <-r.events: // drop oldest
		default:
		}
	}
}

// State returns the flavor's current state.
func (r *Runner) State(f engine.Flavor) State {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s := r.slots[f]; s != nil {
		return s.state
	}
	return NotLoaded
}

// Err returns the load error of a Failed flavor, else nil.
func (r *Runner) Err(f engine.Flavor) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s := r.slots[f]; s != nil {
		return s.err
	}
	return nil
}

// Info returns the engine's info; false until the engine is loaded.
func (r *Runner) Info(f engine.Flavor) (engine.Info, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.slots[f]
	if s == nil || s.eng == nil {
		return engine.Info{}, false
	}
	return s.info, true
}

// Load starts loading the flavor's engine in the background. It is a no-op
// if the flavor is unknown, already loading or loaded, or has failed.
func (r *Runner) Load(f engine.Flavor) {
	r.mu.Lock()
	r.startLoadLocked(f)
	r.mu.Unlock()
}

func (r *Runner) startLoadLocked(f engine.Flavor) {
	s := r.slots[f]
	if s == nil || r.closed || s.state != NotLoaded {
		return
	}
	s.state = Loading
	r.emit(f, Loading)
	r.loads.Add(1)
	go func() {
		defer r.loads.Done()
		eng, err := s.factory(r.ctx, r.cacheDir)
		r.mu.Lock()
		defer r.mu.Unlock()
		if err == nil && r.closed {
			_ = eng.Close() // runner already closed; the engine is discarded and ErrClosed reported
			err = ErrClosed
			eng = nil
		}
		if err != nil {
			s.state, s.err = Failed, err
		} else {
			s.state, s.eng, s.info = Ready, eng, eng.Info()
		}
		close(s.ready)
		r.emit(f, s.state)
	}()
}

// WarmUp loads every other flavor in the background.
func (r *Runner) WarmUp(except engine.Flavor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, f := range engine.Flavors {
		if f != except {
			r.startLoadLocked(f)
		}
	}
}

// Eval evaluates req with the default timeout. See EvalTimeout.
func (r *Runner) Eval(f engine.Flavor, req engine.Request) (engine.Result, error) {
	return r.EvalTimeout(f, req, r.timeout)
}

// EvalTimeout evaluates req on flavor f, loading the engine first if needed.
// It cancels any in-flight evaluation of the same flavor (which then returns
// context.Canceled). If the engine does not finish within d the result is
// ErrTimeout, and the flavor goes Restarting then Ready (the engine restarts
// itself on its next Eval). The timeout does not include engine load time.
func (r *Runner) EvalTimeout(f engine.Flavor, req engine.Request, d time.Duration) (engine.Result, error) {
	return r.eval(f, req, d, false)
}

// eval implements EvalTimeout. all selects the EvalAll cancellation scope,
// which is independent of the main (Eval) scope of the same flavor.
func (r *Runner) eval(f engine.Flavor, req engine.Request, d time.Duration, all bool) (engine.Result, error) {
	r.mu.Lock()
	s := r.slots[f]
	if r.closed {
		r.mu.Unlock()
		return engine.Result{}, ErrClosed
	}
	if s == nil {
		r.mu.Unlock()
		return engine.Result{}, errors.New("unsupported flavor: " + string(f))
	}
	slotCancel := &s.cancel
	if all {
		slotCancel = &s.allCancel
	}
	if *slotCancel != nil {
		(*slotCancel)() // supersede the previous evaluation in this scope
	}
	ctx, cancel := context.WithCancel(r.ctx)
	defer cancel()
	*slotCancel = cancel
	r.startLoadLocked(f)
	r.evals.Add(1)
	defer r.evals.Done()
	ready := s.ready
	r.mu.Unlock()

	select {
	case <-ready:
	case <-ctx.Done():
		return engine.Result{}, r.cancelErr()
	}
	r.mu.Lock()
	eng, loadErr := s.eng, s.err
	r.mu.Unlock()
	if loadErr != nil {
		return engine.Result{}, loadErr
	}
	if ctx.Err() != nil { // superseded while loading
		return engine.Result{}, r.cancelErr()
	}

	tctx, tcancel := context.WithTimeout(ctx, d)
	defer tcancel()
	res, err := eng.Eval(tctx, req)
	if err == nil {
		return res, nil
	}
	if ctx.Err() != nil { // superseded or closed, not timed out
		return engine.Result{}, r.cancelErr()
	}
	if errors.Is(tctx.Err(), context.DeadlineExceeded) {
		r.markRestarted(s, f)
		return engine.Result{}, ErrTimeout
	}
	return engine.Result{}, err
}

// cancelErr is what a cancelled evaluation returns: ErrClosed when the whole
// runner is shutting down, otherwise context.Canceled.
func (r *Runner) cancelErr() error {
	if r.ctx.Err() != nil {
		return ErrClosed
	}
	return context.Canceled
}

// markRestarted does the state bookkeeping after a timeout: the engine
// restarts itself on its next Eval, so the flavor passes Restarting and
// returns to Ready.
func (r *Runner) markRestarted(s *slot, f engine.Flavor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s.state != Ready || r.closed {
		return
	}
	r.emit(f, Restarting)
	r.emit(f, Ready)
}

// EvalAll evaluates the request of every flavor that is Ready, concurrently.
// Flavors that are not Ready are absent from the result. EvalAll has its own
// cancellation scope: a newer EvalAll supersedes the entries of an older one
// (they return context.Canceled), but it neither cancels nor is cancelled by
// Eval/EvalTimeout on the same flavor.
func (r *Runner) EvalAll(reqs map[engine.Flavor]engine.Request) map[engine.Flavor]Outcome {
	out := map[engine.Flavor]Outcome{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for f, req := range reqs {
		if r.State(f) != Ready {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := r.eval(f, req, r.timeout, true)
			mu.Lock()
			out[f] = Outcome{res, err}
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out
}

// Close cancels in-flight evaluations, waits for loads and evaluations to
// finish, closes all engines and closes the Events channel. It is idempotent.
func (r *Runner) Close() {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	r.mu.Unlock()
	r.cancel()
	r.loads.Wait()
	r.evals.Wait()
	r.mu.Lock()
	for _, s := range r.slots {
		if s.eng != nil {
			_ = s.eng.Close() // shutdown; Close has no caller to report to
			s.eng = nil
		}
	}
	r.mu.Unlock()
	r.evMu.Lock()
	r.evClosed = true
	close(r.events)
	r.evMu.Unlock()
}
