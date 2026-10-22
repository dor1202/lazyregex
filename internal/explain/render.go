package explain

import (
	"strings"
	"unicode/utf8"
)

// Render draws rows as a plain-text tree: the pattern token (with tree
// guides) in a left column and its explanation in the right one.
func Render(rows []Row) string {
	left := make([]string, len(rows))
	width := 0
	for i, r := range rows {
		left[i] = guides(rows, i) + visible(r.Token)
		width = max(width, utf8.RuneCountInString(left[i]))
	}
	var b strings.Builder
	for i, r := range rows {
		line := left[i]
		if r.Text != "" {
			line += strings.Repeat(" ", width-utf8.RuneCountInString(line)+2) + r.Text
		}
		b.WriteString(strings.TrimRight(line, " "))
		b.WriteByte('\n')
	}
	return b.String()
}

// visible makes whitespace in a token readable.
func visible(tok string) string {
	if strings.Trim(tok, " ") == "" {
		return strings.Repeat("␣", len(tok))
	}
	return strings.NewReplacer("\n", "⏎", "\t", "⇥", "\r", "␍").Replace(tok)
}

// guides is the ├─ / └─ prefix for row i.
func guides(rows []Row, i int) string {
	d := rows[i].Depth
	if d == 0 {
		return ""
	}
	// more reports whether a later row at depth k follows before the
	// tree climbs above depth k.
	more := func(k int) bool {
		for j := i + 1; j < len(rows); j++ {
			if rows[j].Depth < k {
				return false
			}
			if rows[j].Depth == k {
				return true
			}
		}
		return false
	}
	var b strings.Builder
	for k := 1; k < d; k++ {
		if more(k) {
			b.WriteString("│  ")
		} else {
			b.WriteString("   ")
		}
	}
	if more(d) {
		b.WriteString("├─ ")
	} else {
		b.WriteString("└─ ")
	}
	return b.String()
}
