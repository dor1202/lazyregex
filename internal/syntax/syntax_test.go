package syntax

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestLexersAndStylesLoad(t *testing.T) {
	loadOnce.Do(load)
	for _, lang := range []string{Python, JavaScript, PHP, Go} {
		if lexers[lang] == nil {
			t.Errorf("no %s lexer", lang)
		}
	}
	for _, name := range []string{Dark, Light} {
		st := Style(name)
		if st == nil {
			t.Fatalf("no %s style", name)
		}
		for _, tt := range append(st.Types(), chroma.Keyword, chroma.Text) {
			if st.Get(tt).Background.IsSet() {
				t.Errorf("%s: %v has a background", name, tt)
			}
		}
	}
}

func TestLinesOnlyAddColour(t *testing.T) {
	cases := map[string][]string{
		Python:     {"import re", "", "m = re.search(r\"\\d+\", \"\"\"a", "b\"\"\")"},
		JavaScript: {"const re = /a+/g;", "for (const m of s.matchAll(re)) {", "}"},
		PHP:        {"<?php", "$t = 'x';", "preg_match('/a/', $t, $m);", "/* a", "b */"},
		Go:         {"package main", "", "func main() {", "\tx := `a", "b`", "}"},
	}
	for lang, lines := range cases {
		out := Lines(lang, lines, Dark)
		if len(out) != len(lines) {
			t.Fatalf("%s: %d lines, want %d", lang, len(out), len(lines))
		}
		coloured := false
		for i, l := range out {
			if ansi.Strip(l) != lines[i] || !strings.HasSuffix(l, Reset) {
				t.Errorf("%s line %d: %q", lang, i, l)
			}
			coloured = coloured || strings.Contains(l, "\x1b[38;2;")
		}
		if !coloured {
			t.Errorf("%s: nothing coloured", lang)
		}
	}
}

func TestPHPOpenTag(t *testing.T) {
	out := Lines(PHP, []string{"<?php", "print_r($m);"}, Dark)
	want := sgrOf(Style(Dark).Get(chroma.CommentPreproc)) + "<?php"
	if !strings.HasPrefix(out[0], want) {
		t.Errorf("the open tag is not a preprocessor comment: %q", out[0])
	}
}

func TestFallbackUncoloured(t *testing.T) {
	for _, out := range [][]string{Lines("cobol", []string{"a", "b"}, Dark), Lines(Go, []string{"a", "b"}, "nope")} {
		if out[0] != "a"+Reset || out[1] != "b"+Reset {
			t.Errorf("fallback: %q", out)
		}
	}
}
