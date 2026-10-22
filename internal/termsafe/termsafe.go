// Package termsafe maps user and engine text to what is safe to write to a
// terminal.
package termsafe

import (
	"strings"
	"unicode/utf8"
)

// Rune is how a rune of user or engine text is drawn, so that nothing
// reaches the terminal raw that could act as a control sequence or reorder
// the display, and widths stay exact: C0 controls (tab included) become
// their one-cell Unicode control pictures, DEL becomes ␡, and C1 controls
// (U+0080–U+009F, e.g. U+009B CSI) and bidirectional controls become U+FFFD.
func Rune(r rune) rune {
	switch {
	case r < 0x20:
		return 0x2400 + r
	case r == 0x7f:
		return '␡'
	case r >= 0x80 && r <= 0x9f, isBidiControl(r):
		return utf8.RuneError
	}
	return r
}

// isBidiControl reports the Unicode bidirectional formatting characters.
func isBidiControl(r rune) bool {
	switch {
	case r == 0x061c, r == 0x200e, r == 0x200f:
		return true
	case r >= 0x202a && r <= 0x202e, r >= 0x2066 && r <= 0x2069:
		return true
	}
	return false
}

// String applies Rune to a whole string.
func String(s string) string { return strings.Map(Rune, s) }

// Lines applies Rune to every line of s, keeping its newlines.
func Lines(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = String(l)
	}
	return strings.Join(lines, "\n")
}
