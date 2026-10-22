package runner

import "strings"

// ManualThreshold is the largest text (in bytes) that is evaluated live;
// anything longer requires a manual evaluation.
const ManualThreshold = 1 << 20

// Prepared is piped input text made safe for the engines.
type Prepared struct {
	Text        string
	InvalidUTF8 bool // invalid sequences were replaced with U+FFFD
	Manual      bool // len(b) > ManualThreshold
}

// PrepareText converts raw input to valid UTF-8, replacing each run of
// invalid bytes with U+FFFD, and flags oversized text.
func PrepareText(b []byte) Prepared {
	s := string(b)
	p := Prepared{Text: s, Manual: len(b) > ManualThreshold}
	if v := strings.ToValidUTF8(s, "�"); v != s {
		p.Text, p.InvalidUTF8 = v, true
	}
	return p
}
