// Package ui is the lazygit-style terminal interface: seven panels around a
// shared State, a bottom key bar, and global focus/tab/resize keys.
package ui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dor1202/lazyregex/internal/config"
	"github.com/dor1202/lazyregex/internal/engine"
	"github.com/dor1202/lazyregex/internal/runner"
)

// PanelID identifies one of the seven panels; its value is the panel's
// number key.
type PanelID int

// Panels in number-key order.
const (
	PFlavor PanelID = iota + 1
	PFlags
	PMatches
	PLibrary
	PPattern
	PText
	PDetails
)

// panelCount is the number of panels; IDs run 1..panelCount.
const panelCount = 7

func (p PanelID) String() string {
	return [...]string{"none", "Flavor", "Flags", "Matches", "Library", "Pattern", "Test String", "Details"}[p]
}

// KeyHelp is one key shown in the key bar and the help popup.
type KeyHelp struct{ Key, Desc string }

// Panel is one bordered pane. The shell draws the border, the title (built
// from Tabs) and the counter; the panel draws only its content.
type Panel interface {
	// Init runs once when the program starts.
	Init(s *State) tea.Cmd
	// Tabs returns the tab names; a single-tab panel returns one name. The
	// shell renders "[n]─" + names joined by " ─ ", the active tab
	// (State.Tab[id]) green.
	Tabs() []string
	// Update receives key presses while the panel has focus (after the
	// global keys) and every non-key message the shell does not consume.
	Update(msg tea.Msg, s *State) tea.Cmd
	// View renders the content area of w×h cells. Lines may be shorter or
	// longer than w; the shell pads and truncates by display width.
	View(s *State, w, h int) string
	// Keys are shown in the key bar and the help popup's "Local" section.
	Keys(s *State) []KeyHelp
	// Counter is drawn on the bottom border ("2 of 4"); "" draws none.
	Counter(s *State) string
	// Height is the preferred content height in rows for an auto-height
	// panel (Flavor, Flags, Library, Pattern), or 0 for a flexible one.
	Height(s *State, w int) int
	// IsEditor reports whether the panel is a text editor: while it has
	// focus and State.Editing is set, only tab, shift+tab, esc (and ctrl+c)
	// are global and every other key goes to the panel.
	IsEditor() bool
}

// State is the model state shared by all panels.
type State struct {
	Flavor      engine.Flavor
	Flags       map[engine.Flavor]string // flag letters per flavor
	Mode        engine.Mode              // follows the Pattern panel's tab
	Pattern     string
	Replacement string
	Text        string
	Manual      bool // text is too large for live evaluation
	InvalidUTF8 bool // invalid input bytes were replaced with U+FFFD
	Result      *engine.Result
	Err         error
	SelMatch    int
	SelRow      int
	Focus       PanelID
	Tab         map[PanelID]int
	Mouse       bool
	LeftWidth   int // left column width in cells; 0 = automatic (34%)

	// Additions to the plan's sketch.
	Editing   bool           // keys go to the focused editor panel (5 or 6)
	Width     int            // terminal size
	Height    int            //
	Runner    *runner.Runner // engines, for panels that evaluate
	Config    config.Config
	ConfigDir string
	Library   *config.Library
	Theme     *Theme

	// Evaluation (Task 14). Result and Err belong to EvalFlavor/EvalPattern
	// and to ResultText, which may lag the live inputs by one debounce.
	Matches     []engine.Match // Result's matches with byte spans into ResultText
	ResultText  string
	EvalFlavor  engine.Flavor
	EvalPattern string
	EvalTime    time.Duration
	Evaluating  bool                             // a manual evaluation is running
	Stale       bool                             // manual mode: inputs changed since the last evaluation
	All         map[engine.Flavor]runner.Outcome // EvalAll outcomes of the other Ready flavors
}

// basePanel provides defaults for the optional parts of Panel.
type basePanel struct{}

func (basePanel) Init(*State) tea.Cmd            { return nil }
func (basePanel) Update(tea.Msg, *State) tea.Cmd { return nil }
func (basePanel) Keys(*State) []KeyHelp          { return nil }
func (basePanel) Counter(*State) string          { return "" }
func (basePanel) Height(*State, int) int         { return 0 }
func (basePanel) IsEditor() bool                 { return false }

// editorBase is basePanel for the editor panels: enter starts editing when
// the panel has focus but is not being edited.
type editorBase struct{ basePanel }

func (editorBase) IsEditor() bool { return true }

func (editorBase) Update(msg tea.Msg, s *State) tea.Cmd {
	if k, ok := msg.(tea.KeyPressMsg); ok && !s.Editing && k.String() == "enter" {
		s.Editing = true
	}
	return nil
}

func (editorBase) Keys(s *State) []KeyHelp {
	if s.Editing {
		return nil
	}
	return []KeyHelp{{"enter", "Edit"}}
}

// counterOf formats a list position as "i of n" ("" for an empty list).
func counterOf(i, n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%d of %d", i+1, n)
}

// listRow renders a list row padded to w; the selected row gets the
// selection background across the full width.
func listRow(t *Theme, text string, w int, selected bool) string {
	text = fitWidth(sanitize(text), w)
	if selected {
		return t.selectedRow().Render(text)
	}
	return t.fg("text").Render(text)
}
