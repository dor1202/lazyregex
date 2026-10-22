package termsafe

import "testing"

func TestString(t *testing.T) {
	for in, want := range map[string]string{
		"plain é😀":       "plain é😀",
		"a\x1b[31mb\x07": "a␛[31mb␇",
		"tab\there\n":    "tab␉here␊",
		"del\x7f":        "del␡",
		"csi\u009b":      "csi�",
		"bidi‮⁦":         "bidi��",
	} {
		if got := String(in); got != want {
			t.Errorf("String(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLinesKeepsNewlines(t *testing.T) {
	if got, want := Lines("a\x1b\nb\x07\n"), "a␛\nb␇\n"; got != want {
		t.Errorf("Lines = %q, want %q", got, want)
	}
}
