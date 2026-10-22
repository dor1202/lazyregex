package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// palette maps a colour token to its {dark, light} hex values. The dark
// values are the approved mockup palette; the light ones keep the same hues
// with enough contrast on a white background.
var palette = map[string][2]string{
	// chrome
	"border":   {"#5b6472", "#9aa3af"},
	"focus":    {"#87d787", "#2e7d32"}, // focused border, active tab
	"title":    {"#c9d1d9", "#24292f"},
	"key":      {"#6cb6ff", "#0969da"}, // key bar
	"dim":      {"#8b96a5", "#6e7781"},
	"text":     {"#d0d7de", "#1f2328"},
	"selected": {"#1f3a5f", "#cfe2ff"}, // selected row background
	"match":    {"#1d3557", "#dbe9ff"}, // whole-match background in [6]
	"error":    {"#ff6b6b", "#cf222e"},
	"ok":       {"#87d787", "#1a7f37"},

	// pattern syntax ([5])
	"anchor": {"#ffd75f", "#9a6700"},
	"paren":  {"#ffaf5f", "#bc4c00"},
	"escape": {"#5fd7ff", "#0a7ea4"},
	"quant":  {"#ff87d7", "#bf3989"},
	"delim":  {"#8b96a5", "#6e7781"},

	// capture groups: accent (pattern group names) and the [6] highlight
	// background/foreground pair. Group i uses token group((i-1)%6+1).
	"group1": {"#ffb870", "#b35900"}, "group1.bg": {"#6b4513", "#ffe3c2"}, "group1.fg": {"#ffe3c2", "#6b4513"},
	"group2": {"#87d787", "#2e7d32"}, "group2.bg": {"#1e5631", "#d6f5d6"}, "group2.fg": {"#d6f5d6", "#1e5631"},
	"group3": {"#d7a6ff", "#8250df"}, "group3.bg": {"#4b2a63", "#efd9ff"}, "group3.fg": {"#efd9ff", "#4b2a63"},
	"group4": {"#87afff", "#0550ae"}, "group4.bg": {"#1f3d6b", "#dbe8ff"}, "group4.fg": {"#dbe8ff", "#1f3d6b"},
	"group5": {"#ff87af", "#bf3989"}, "group5.bg": {"#5c1f3a", "#ffd9e8"}, "group5.fg": {"#ffd9e8", "#5c1f3a"},
	"group6": {"#d7d75f", "#7d6c00"}, "group6.bg": {"#4a4a14", "#f5f5c8"}, "group6.fg": {"#f5f5c8", "#4a4a14"},
}

// Theme resolves colour tokens for a light or dark terminal.
type Theme struct {
	dark   bool
	colors map[string]color.Color
}

// newTheme builds the theme for the terminal background. overrides (the
// config's theme map, token -> "#rrggbb" or ANSI number) apply to both
// variants; unknown tokens and unparsable values are ignored.
func newTheme(dark bool, overrides map[string]string) *Theme {
	pick := lipgloss.LightDark(dark)
	t := &Theme{dark: dark, colors: map[string]color.Color{}}
	for tok, v := range palette {
		t.colors[tok] = pick(lipgloss.Color(v[1]), lipgloss.Color(v[0]))
	}
	for tok, v := range overrides {
		if _, known := palette[tok]; !known {
			continue
		}
		if c := lipgloss.Color(v); c != (lipgloss.NoColor{}) {
			t.colors[tok] = c
		}
	}
	return t
}

// color returns the token's colour (no colour for an unknown token).
func (t *Theme) color(tok string) color.Color {
	if c, ok := t.colors[tok]; ok {
		return c
	}
	return lipgloss.NoColor{}
}

// fg is a style with the token as foreground.
func (t *Theme) fg(tok string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.color(tok))
}

// selectedRow is the style of a selected list row.
func (t *Theme) selectedRow() lipgloss.Style {
	return lipgloss.NewStyle().Background(t.color("selected")).Foreground(t.color("text")).Bold(true)
}
