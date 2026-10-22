package ui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/dor1202/lazyregex/internal/engine"
)

// jumpToMatchMsg asks [6] to scroll to a match.
type jumpToMatchMsg struct{ match int }

// matchesPanel is [3]: the matches, each expandable into its groups
// (Matches tab), or the selected match's groups (Groups tab). Spans are
// shown in the engine's native units.
type matchesPanel struct {
	basePanel
	expanded map[int]bool
	top      int
	result   *engine.Result // the result expanded refers to
}

func newMatchesPanel() *matchesPanel { return &matchesPanel{expanded: map[int]bool{}} }

func (*matchesPanel) Tabs() []string { return []string{"Matches", "Groups"} }

func (*matchesPanel) Keys(*State) []KeyHelp {
	return []KeyHelp{{"enter", "Expand"}, {"o", "Jump to text"}, {"y", "Copy value"}, {"Y", "Copy all as JSON"}}
}

func (p *matchesPanel) Counter(s *State) string {
	return counterOf(s.SelMatch, len(s.Matches))
}

// row is one line of the list: a match (group < 0) or one of its groups.
type row struct{ match, group int }

func (p *matchesPanel) rows(s *State) []row {
	if p.result != s.Result { // a new result: collapse everything
		p.result, p.expanded = s.Result, map[int]bool{}
	}
	var rs []row
	if s.Tab[PMatches] == 1 { // Groups: the selected match's groups
		if s.SelMatch < len(s.Matches) {
			for g := range s.Matches[s.SelMatch].Groups {
				rs = append(rs, row{s.SelMatch, g})
			}
		}
		return rs
	}
	for i, m := range s.Matches {
		rs = append(rs, row{i, -1})
		if p.expanded[i] {
			for g := range m.Groups {
				rs = append(rs, row{i, g})
			}
		}
	}
	return rs
}

// selected is the index of the selected row.
func selectedRow(rs []row, s *State) int {
	for i, r := range rs {
		if r.match == s.SelMatch && (r.group == s.SelRow || (r.group < 0 && s.SelRow < 0)) {
			return i
		}
	}
	for i, r := range rs {
		if r.match == s.SelMatch {
			return i
		}
	}
	return 0
}

func (p *matchesPanel) Update(msg tea.Msg, s *State) tea.Cmd {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok || len(s.Matches) == 0 {
		return nil
	}
	rs := p.rows(s)
	if len(rs) == 0 {
		return nil
	}
	i := selectedRow(rs, s)
	switch k.String() {
	case "up", "k":
		i = max(0, i-1)
	case "down", "j":
		i = min(len(rs)-1, i+1)
	case "enter":
		if s.Tab[PMatches] == 0 {
			m := rs[i].match
			p.expanded[m] = !p.expanded[m]
			s.SelMatch, s.SelRow = m, -1
		}
		return nil
	case "o":
		m := rs[i].match
		return func() tea.Msg { return jumpToMatchMsg{m} }
	case "y":
		return tea.SetClipboard(p.value(s, rs[i]))
	case "Y":
		return tea.SetClipboard(matchesJSON(s))
	default:
		return nil
	}
	s.SelMatch, s.SelRow = rs[i].match, rs[i].group
	return nil
}

// value is the text of a row's match or group ("" for an unmatched group).
func (p *matchesPanel) value(s *State, r row) string {
	m := s.Matches[r.match]
	if r.group < 0 {
		return slice(s.ResultText, m.Span)
	}
	if g := m.Groups[r.group]; g.Matched {
		return slice(s.ResultText, g.Span)
	}
	return ""
}

// slice returns text[sp], clamped to the text.
func slice(text string, sp engine.Span) string {
	a, b := clamp(sp.Start, 0, len(text)), clamp(sp.End, 0, len(text))
	if b < a {
		return ""
	}
	return text[a:b]
}

func (p *matchesPanel) View(s *State, w, h int) string {
	t := s.Theme
	switch {
	case s.Result == nil:
		return t.fg("dim").Render("no matches yet")
	case s.Mode != engine.ModeMatch:
		return t.fg("dim").Render("matches are listed in Match mode")
	case len(s.Matches) == 0:
		return t.fg("dim").Render("no matches")
	}
	rs := p.rows(s)
	sel := selectedRow(rs, s)
	if sel < p.top {
		p.top = sel
	} else if sel >= p.top+h {
		p.top = sel - h + 1
	}
	p.top = clamp(p.top, 0, max(0, len(rs)-h))

	native := s.Result.Matches
	nameW := 1
	for _, g := range native[min(s.SelMatch, len(native)-1)].Groups {
		nameW = max(nameW, len(groupLabel(g)))
	}
	var out []string
	for i := p.top; i < min(len(rs), p.top+h); i++ {
		r := rs[i]
		var text string
		if r.group < 0 {
			arrow := "▸"
			if p.expanded[r.match] {
				arrow = "▾"
			}
			text = fmt.Sprintf("%s #%-3d %-10s %s", arrow, r.match+1, spanText(native[r.match].Span), quoteValue(p.value(s, r)))
		} else {
			g := native[r.match].Groups[r.group]
			val := "not matched"
			sp := ""
			if g.Matched {
				sp, val = spanText(g.Span), quoteValue(p.value(s, r))
			}
			text = fmt.Sprintf("    %s %-9s %s", padRight(groupLabel(g), nameW), sp, val)
		}
		out = append(out, listRow(t, text, w, i == sel))
	}
	return strings.Join(out, "\n")
}

func groupLabel(g engine.Group) string {
	if g.Name != "" {
		return g.Name
	}
	return strconv.Itoa(g.Index)
}

func spanText(sp engine.Span) string { return fmt.Sprintf("%d–%d", sp.Start, sp.End) }

// quoteValue makes a value printable on one line.
func quoteValue(v string) string { return strings.Map(displayRune, v) }

// matchesJSON is every match with its groups, spans in native units.
func matchesJSON(s *State) string {
	type jgroup struct {
		Index   int     `json:"index"`
		Name    string  `json:"name,omitempty"`
		Matched bool    `json:"matched"`
		Span    *[2]int `json:"span"`
		Value   *string `json:"value"`
	}
	type jmatch struct {
		Index  int      `json:"index"`
		Span   [2]int   `json:"span"`
		Value  string   `json:"value"`
		Groups []jgroup `json:"groups"`
	}
	out := []jmatch{}
	if s.Result != nil {
		for i, m := range s.Result.Matches {
			jm := jmatch{Index: i, Span: [2]int{m.Span.Start, m.Span.End}, Groups: []jgroup{}}
			if i < len(s.Matches) {
				jm.Value = slice(s.ResultText, s.Matches[i].Span)
			}
			for j, g := range m.Groups {
				jg := jgroup{Index: g.Index, Name: g.Name, Matched: g.Matched}
				if g.Matched && i < len(s.Matches) {
					v := slice(s.ResultText, s.Matches[i].Groups[j].Span)
					jg.Span, jg.Value = &[2]int{g.Span.Start, g.Span.End}, &v
				}
				jm.Groups = append(jm.Groups, jg)
			}
			out = append(out, jm)
		}
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	return string(b)
}
