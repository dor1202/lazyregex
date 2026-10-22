package ui

import (
	"context"
	"errors"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dor1202/lazyregex/internal/engine"
	"github.com/dor1202/lazyregex/internal/runner"
)

// Evaluation timing.
const (
	evalDebounce  = 75 * time.Millisecond  // live evaluation after the last edit
	allDebounce   = 300 * time.Millisecond // EvalAll for the flavor ✗ markers
	manualTimeout = 30 * time.Second       // manual (large input) evaluation
)

// Messages of the evaluation loop.
type (
	// runnerEventMsg is a runner state change, broadcast to every panel.
	runnerEventMsg runner.Event
	// evalTickMsg fires evalDebounce after an edit; only the latest counts.
	evalTickMsg struct{ seq int }
	// allTickMsg fires allDebounce after an edit; only the latest counts.
	allTickMsg struct{ seq int }
	// evalNowMsg asks for a manual evaluation (enter on large input).
	evalNowMsg struct{}
	// evalDoneMsg carries one evaluation's outcome; seq drops stale ones.
	evalDoneMsg struct {
		seq           int
		flavor        engine.Flavor
		pattern, text string
		res           engine.Result
		err           error
		took          time.Duration
	}
	// allDoneMsg carries the outcomes of the other Ready flavors.
	allDoneMsg struct {
		seq int
		out map[engine.Flavor]runner.Outcome
	}
)

// evalInput is everything an evaluation depends on; a change schedules one.
type evalInput struct {
	flavor               engine.Flavor
	flags                string
	ready                bool // the flavor's engine is loaded (its flags are known)
	mode                 engine.Mode
	pattern, replacement string
	text                 string
}

// evalLoop is the shell's evaluation bookkeeping.
type evalLoop struct {
	last    evalInput
	editSeq int // debounce generation of live evaluations
	evalSeq int // the latest started evaluation; older results are dropped
	allSeq  int // the latest EvalAll generation
	initial engine.Flavor
	warmed  bool                   // WarmUp has been called
	loaded  map[engine.Flavor]bool // flavors seen Ready at least once
}

// waitForEvent delivers the next runner event as a message. The shell
// re-arms it after every event; it is the only reader of the channel.
func waitForEvent(ch <-chan runner.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return runnerEventMsg(ev)
	}
}

// initEval starts loading the initial flavor and listening for events.
func (m *Model) initEval() tea.Cmd {
	r := m.state.Runner
	if r == nil {
		return nil
	}
	m.ev.initial = m.state.Flavor
	r.Load(m.state.Flavor)
	return waitForEvent(r.Events())
}

func (m *Model) currentInput() evalInput {
	s := &m.state
	flags, ready := flagsOf(s, s.Flavor)
	return evalInput{
		flavor: s.Flavor, flags: flags, ready: ready, mode: s.Mode,
		pattern: s.Pattern, replacement: s.Replacement, text: s.Text,
	}
}

// afterUpdate schedules a debounced evaluation (or, for large input, marks
// the result stale) and a debounced EvalAll when the inputs changed.
func (m *Model) afterUpdate() tea.Cmd {
	in := m.currentInput()
	if in == m.ev.last {
		return nil
	}
	m.ev.last = in
	var cmd tea.Cmd
	if m.state.Manual {
		m.state.Stale = true
	} else {
		m.ev.editSeq++
		seq := m.ev.editSeq
		cmd = tea.Tick(evalDebounce, func(time.Time) tea.Msg { return evalTickMsg{seq} })
	}
	return tea.Batch(cmd, m.scheduleAll())
}

func (m *Model) scheduleAll() tea.Cmd {
	m.ev.allSeq++
	seq := m.ev.allSeq
	return tea.Tick(allDebounce, func(time.Time) tea.Msg { return allTickMsg{seq} })
}

// updateEval handles the evaluation loop's messages; ok is false for any
// other message.
func (m *Model) updateEval(msg tea.Msg) (cmd tea.Cmd, ok bool) {
	s := &m.state
	switch msg := msg.(type) {
	case runnerEventMsg:
		return m.onRunnerEvent(runner.Event(msg)), true
	case evalTickMsg:
		if msg.seq == m.ev.editSeq {
			return m.startEval(false), true
		}
		return nil, true
	case evalNowMsg:
		return m.startEval(true), true
	case evalDoneMsg:
		m.finishEval(msg)
		return nil, true
	case allTickMsg:
		if msg.seq == m.ev.allSeq {
			return m.startAll(), true
		}
		return nil, true
	case allDoneMsg:
		if msg.seq == m.ev.allSeq {
			s.All = map[engine.Flavor]runner.Outcome{}
			for f, o := range msg.out {
				if !errors.Is(o.Err, context.Canceled) && !errors.Is(o.Err, runner.ErrClosed) {
					s.All[f] = o
				}
			}
		}
		return nil, true
	}
	return nil, false
}

func (m *Model) onRunnerEvent(ev runner.Event) tea.Cmd {
	s := &m.state
	cmds := []tea.Cmd{waitForEvent(s.Runner.Events())}
	switch ev.State {
	case runner.Ready:
		if ev.Flavor == m.ev.initial && !m.ev.warmed {
			m.ev.warmed = true
			s.Runner.WarmUp(ev.Flavor)
		}
		// Only a first load gives a flavor a new marker. A timeout emits
		// Restarting then Ready again; re-running EvalAll then would time
		// out again, forever, and drop each result as stale.
		if m.ev.loaded == nil {
			m.ev.loaded = map[engine.Flavor]bool{}
		}
		if !m.ev.loaded[ev.Flavor] {
			m.ev.loaded[ev.Flavor] = true
			if ev.Flavor != s.Flavor {
				cmds = append(cmds, m.scheduleAll())
			}
		}
	case runner.Failed:
		if ev.Flavor == s.Flavor {
			s.Err = loadError(s.Runner, ev.Flavor)
		}
	}
	return tea.Batch(cmds...)
}

func loadError(r *runner.Runner, f engine.Flavor) error {
	return fmt.Errorf("%s engine failed to load: %w", f, r.Err(f))
}

// startEval evaluates the current input in the background. manual selects
// the large-input timeout. Until the flavor's engine is loaded it only
// starts the load; the Ready event then changes the input and re-schedules.
func (m *Model) startEval(manual bool) tea.Cmd {
	s := &m.state
	r := s.Runner
	if r == nil {
		return nil
	}
	m.ev.evalSeq++ // whatever is in flight is now stale
	seq := m.ev.evalSeq
	s.Evaluating = false
	if s.Pattern == "" {
		s.Result, s.Matches, s.Err, s.Stale = nil, nil, nil, false
		return nil
	}
	f := s.Flavor
	flags, ready := flagsOf(s, f)
	if !ready {
		if r.State(f) == runner.Failed {
			s.Err = loadError(r, f)
		} else {
			r.Load(f)
		}
		return nil
	}
	req := engine.Request{
		Pattern: s.Pattern, Flags: flags, Text: s.Text,
		Mode: s.Mode, Replacement: s.Replacement,
	}
	if manual {
		s.Evaluating, s.Stale = true, false
	}
	return func() tea.Msg {
		start := time.Now()
		var res engine.Result
		var err error
		if manual {
			res, err = r.EvalTimeout(f, req, manualTimeout)
		} else {
			res, err = r.Eval(f, req)
		}
		return evalDoneMsg{seq: seq, flavor: f, pattern: req.Pattern, text: req.Text, res: res, err: err, took: time.Since(start)}
	}
}

// finishEval stores a result unless it is stale or was superseded.
func (m *Model) finishEval(msg evalDoneMsg) {
	s := &m.state
	if msg.seq != m.ev.evalSeq {
		return
	}
	s.Evaluating = false
	if errors.Is(msg.err, context.Canceled) || errors.Is(msg.err, runner.ErrClosed) {
		return
	}
	s.EvalFlavor, s.EvalPattern, s.EvalTime, s.ResultText = msg.flavor, msg.pattern, msg.took, msg.text
	if msg.err != nil {
		s.Err, s.Result, s.Matches = msg.err, nil, nil
		return
	}
	res := msg.res
	s.Err, s.Result, s.Matches = nil, &res, byteMatches(msg.text, res)
	s.SelMatch = clamp(s.SelMatch, 0, max(0, len(res.Matches)-1))
}

// startAll evaluates the pattern on every other Ready flavor, for the ✗
// markers. The live text is sent because Task 15's Compare tab shows each
// flavor's match count from these outcomes; large (manual) input is not.
func (m *Model) startAll() tea.Cmd {
	s := &m.state
	if s.Runner == nil {
		return nil
	}
	if s.Pattern == "" {
		s.All = nil
		return nil
	}
	text := s.Text
	if s.Manual {
		text = ""
	}
	reqs := map[engine.Flavor]engine.Request{}
	for _, f := range engine.Flavors {
		if f == s.Flavor {
			continue // the main evaluation covers it
		}
		if flags, ok := flagsOf(s, f); ok {
			reqs[f] = engine.Request{Pattern: s.Pattern, Flags: flags, Text: text, Mode: engine.ModeMatch}
		}
	}
	if len(reqs) == 0 {
		return nil
	}
	seq, r := m.ev.allSeq, s.Runner
	return func() tea.Msg { return allDoneMsg{seq: seq, out: r.EvalAll(reqs)} }
}

// compileError returns err as a *CompileError, or nil.
func compileError(err error) *engine.CompileError {
	var ce *engine.CompileError
	if errors.As(err, &ce) {
		return ce
	}
	return nil
}

// flavorFails reports whether the pattern fails to compile in f: from the
// main evaluation for the current flavor, from EvalAll for the others.
func flavorFails(s *State, f engine.Flavor) bool {
	if f == s.Flavor {
		return s.EvalFlavor == f && s.EvalPattern == s.Pattern && compileError(s.Err) != nil
	}
	o, ok := s.All[f]
	return ok && compileError(o.Err) != nil
}
