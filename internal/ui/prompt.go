package ui

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/dor1202/lazyregex/internal/engine"
)

// command is one ":" command. The table drives runCommand, tab completion
// and the help popup.
type command struct {
	name, args, desc string
	values           func(s *State) []string // argument completions
	run              func(arg string, s *State) (tea.Cmd, error)
}

// noticeMsg shows a message on the bottom line until the next key press.
type noticeMsg struct {
	text string
	err  bool
}

func notice(text string) tea.Cmd { return func() tea.Msg { return noticeMsg{text: text} } }

var modeNames = []string{string(engine.ModeMatch), string(engine.ModeReplace), string(engine.ModeSplit)}

func flavorNames(*State) []string {
	out := make([]string, len(engine.Flavors))
	for i, f := range engine.Flavors {
		out[i] = string(f)
	}
	return out
}

var commands = []command{
	{name: "flavor", args: "<python|js|pcre|go>", desc: "Switch flavor", values: flavorNames,
		run: func(arg string, s *State) (tea.Cmd, error) {
			f := engine.Flavor(arg)
			if !slices.Contains(engine.Flavors, f) {
				return nil, fmt.Errorf("unknown flavor: %s", arg)
			}
			s.Flavor = f
			return nil, nil
		}},
	{name: "flags", args: "<letters>", desc: "Set the flavor's flags",
		run: func(arg string, s *State) (tea.Cmd, error) {
			if infos, ok := flagInfos(s, s.Flavor); ok {
				for _, c := range arg {
					if !slices.ContainsFunc(infos, func(fi engine.FlagInfo) bool { return rune(fi.Letter) == c }) {
						return nil, fmt.Errorf("unknown %s flag: %c", s.Flavor, c)
					}
				}
			}
			s.Flags[s.Flavor] = arg
			return nil, nil
		}},
	{name: "mode", args: "<match|replace|split>", desc: "Match, replace or split",
		values: func(*State) []string { return modeNames },
		run: func(arg string, s *State) (tea.Cmd, error) {
			i := slices.Index(modeNames, arg)
			if i < 0 {
				return nil, fmt.Errorf("unknown mode: %s", arg)
			}
			s.Mode, s.Tab[PPattern] = patternModes[i], i
			return nil, nil
		}},
	{name: "mouse", args: "[on|off]", desc: "Turn the mouse on or off (toggle)",
		values: func(*State) []string { return []string{"on", "off"} },
		run: func(arg string, s *State) (tea.Cmd, error) {
			switch arg {
			case "":
				s.Mouse = !s.Mouse
			case "on", "off":
				s.Mouse = arg == "on"
			default:
				return nil, errors.New("usage: mouse [on|off]")
			}
			if s.Mouse {
				return notice("mouse on"), nil
			}
			return notice("mouse off"), nil
		}},
	{name: "save", args: "<name>", desc: "Save to the library (n in Library)",
		values: libraryNames,
		run: func(arg string, s *State) (tea.Cmd, error) {
			msg, err := saveEntry(s, arg)
			if err != nil {
				return nil, err
			}
			return notice(msg), nil
		}},
	{name: "open", args: "<name>", desc: "Load a library entry",
		values: libraryNames,
		run:    func(arg string, s *State) (tea.Cmd, error) { return nil, openEntry(s, arg) }},
	{name: "q", desc: "Quit",
		run: func(string, *State) (tea.Cmd, error) { return tea.Quit, nil }},
}

func libraryNames(s *State) []string {
	var out []string
	for _, e := range s.Library.Entries {
		out = append(out, e.Name)
	}
	return out
}

// runCommand runs one prompt line ("flavor go", "save my pattern", …). The
// argument is the rest of the line after the command name.
func runCommand(line string, s *State) (tea.Cmd, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil
	}
	name, arg, _ := strings.Cut(line, " ")
	arg = strings.TrimSpace(arg)
	for _, c := range commands {
		if c.name == name {
			return c.run(arg, s)
		}
	}
	return nil, fmt.Errorf("unknown command: %s", name)
}

// prompt is the one-line input on the bottom row: a ":" command line, or a
// y/n question when yes is set.
type prompt struct {
	ed       *editor
	question string
	yes      func(s *State) (string, error)
	choices  []string // candidates of the last ambiguous completion
}

// promptKeys and confirmKeys are the prompt's keys (key bar and help).
var (
	promptKeys  = []KeyHelp{{"enter", "Run"}, {"tab", "Complete"}, {"esc", "Cancel"}}
	confirmKeys = []KeyHelp{{"y", "Yes"}, {"n", "No"}, {"esc", "Cancel"}}
)

func newPrompt(text string) *prompt {
	p := &prompt{ed: newEditor(false)}
	p.ed.SetValue(text)
	return p
}

// update handles a key or paste; done reports that the prompt closed.
func (p *prompt) update(msg tea.Msg, s *State) (cmd tea.Cmd, done bool) {
	k, isKey := msg.(tea.KeyPressMsg)
	if p.yes != nil {
		if !isKey {
			return nil, false
		}
		switch k.String() {
		case "y", "Y":
			text, err := p.yes(s)
			if err != nil {
				return errNotice(err), true
			}
			return notice(text), true
		case "n", "N", "esc":
			return nil, true
		}
		return nil, false
	}
	if isKey {
		switch k.String() {
		case "esc":
			return nil, true
		case "enter":
			c, err := runCommand(p.ed.Value(), s)
			if err != nil {
				return errNotice(err), true
			}
			return c, true
		case "tab":
			p.complete(s)
			return nil, false
		}
	}
	p.choices = nil
	p.ed.Update(msg)
	return nil, false
}

func errNotice(err error) tea.Cmd {
	return func() tea.Msg { return noticeMsg{text: err.Error(), err: true} }
}

// complete extends the line with the command name, flavor, mode, mouse
// setting or library name it starts; several candidates extend it to their
// common prefix and are listed.
func (p *prompt) complete(s *State) {
	line := p.ed.Value()
	name, arg, hasArg := strings.Cut(line, " ")
	var cands []string
	var prefix, word string
	if !hasArg {
		word = name
		for _, c := range commands {
			cands = append(cands, c.name)
		}
	} else {
		i := slices.IndexFunc(commands, func(c command) bool { return c.name == name })
		if i < 0 || commands[i].values == nil {
			return
		}
		prefix, word = name+" ", strings.TrimLeft(arg, " ")
		cands = commands[i].values(s)
	}
	var hits []string
	for _, c := range cands {
		if strings.HasPrefix(c, word) {
			hits = append(hits, c)
		}
	}
	p.choices = nil
	switch len(hits) {
	case 0:
		return
	case 1:
		v := prefix + hits[0]
		if !hasArg {
			v += " "
		}
		p.ed.SetValue(v)
	default:
		p.ed.SetValue(prefix + commonPrefix(hits))
		p.choices = hits
	}
}

func commonPrefix(xs []string) string {
	p := xs[0]
	for _, x := range xs[1:] {
		for !strings.HasPrefix(x, p) {
			_, n := utf8.DecodeLastRuneInString(p)
			p = p[:len(p)-n]
		}
	}
	return p
}

// view renders the prompt line, w cells wide.
func (p *prompt) view(t *Theme, w int) string {
	if p.yes != nil {
		return fitWidth(t.fg("anchor").Render(sanitize(p.question)), w)
	}
	line := p.ed.Value()
	out := t.fg("key").Render(":") + t.renderRuns(line, 0, len(line), func(int) paint { return paint{fg: "text"} }, p.ed.Cursor())
	if len(p.choices) > 0 {
		out += "  " + t.fg("dim").Render(sanitize(strings.Join(p.choices, "  ")))
	}
	return fitWidth(window(out, displayWidth(line[:p.ed.Cursor()])+1, w), w)
}
