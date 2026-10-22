package engine

import "unicode/utf8"

// ToByteOffsets converts offsets expressed in unit into byte offsets in text.
// An offset equal to the text length in that unit maps to len(text). Offsets
// beyond the end clamp to len(text); an offset falling inside a surrogate pair
// maps to the start of that code point.
func ToByteOffsets(text string, unit Unit, offsets []int) []int {
	out := make([]int, len(offsets))
	if unit == UnitBytes {
		copy(out, offsets)
		return out
	}
	// table[u] is the byte offset for unit offset u.
	table := make([]int, 0, len(text)+1)
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		table = append(table, i)
		if unit == UnitUTF16 && r >= 0x10000 {
			table = append(table, i)
		}
		i += size
	}
	table = append(table, len(text))
	for k, o := range offsets {
		switch {
		case o < 0:
			out[k] = 0
		case o >= len(table):
			out[k] = len(text)
		default:
			out[k] = table[o]
		}
	}
	return out
}
