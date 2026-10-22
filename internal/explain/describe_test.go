package explain

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dor1202/lazyregex/internal/engine"
)

var update = flag.Bool("update", false, "rewrite golden files")

func TestExplainGolden(t *testing.T) {
	cases := []struct {
		name    string
		flavor  engine.Flavor
		flags   string
		pattern string
	}{
		{"python_logline", py, "gm", `^(?P<date>\d{4}-\d{2}-\d{2}) (?P<level>INFO|WARN|ERROR)\s+(?P<msg>.*)$`},
		{"pcre_keep_recursion_possessive", pcre, "", `foo\Kbar++(\((?:[^()]|(?1))*\))(?R)?`},
		{"js_unicode_sets", js, "v", `[\p{L}--[a-z]]+|[\q{abc|d}0-9]{2,}?\p{Script=Greek}`},
		{"go_lookbehind_unknown", gor, "", `(?<=a)b(?i:c)d*`},
		{"python_extended_conditional", py, "x", "(a)? # optional a\n (?(1)b|c) \\1 (?:x|yz)* [^\\n\\t]"},
		{"pcre_verbs_flags", pcre, "i", `(*UTF)(?m)^a(?-i)b$(?#note)(*SKIP)(?>x)(?<n>y)\k<n>`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Render(Explain(c.pattern, c.flavor, c.flags))
			path := filepath.Join("testdata", c.name+".golden")
			if *update {
				if err := os.MkdirAll("testdata", 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if got != string(want) {
				t.Errorf("mismatch for %s\n--- got ---\n%s--- want ---\n%s", c.name, got, want)
			}
		})
	}
}

// find returns the text of the first row whose token is tok.
func find(t *testing.T, f engine.Flavor, flags, pattern, tok string) Row {
	t.Helper()
	for _, r := range Explain(pattern, f, flags) {
		if r.Token == tok {
			return r
		}
	}
	t.Fatalf("no row with token %q in %q", tok, pattern)
	return Row{}
}

func TestFlavorWording(t *testing.T) {
	cases := []struct {
		f       engine.Flavor
		flags   string
		pattern string
		tok     string
		want    string
	}{
		{py, "", `a$`, "$", "end of the string (or before a final newline)"},
		{js, "", `a$`, "$", "end of the string"},
		{py, "m", `$`, "$", "end of a line (m flag)"},
		{py, "", `\w`, `\w`, "a word character (Unicode)"},
		{py, "a", `\w`, `\w`, "a word character (ASCII)"},
		{py, "", `(?a)\w`, `\w`, "a word character (ASCII)"},
		{js, "", `\w`, `\w`, "a word character (ASCII)"},
		{gor, "", `\w`, `\w`, "a word character (ASCII)"},
		{pcre, "", `\w`, `\w`, "a word character (ASCII)"},
		{pcre, "u", `\w`, `\w`, "a word character (Unicode)"},
		{py, "s", `.`, ".", "any character (s flag)"},
		{py, "", `.`, ".", "any char but newline"},
		{py, "", `\Z`, `\Z`, "end of the string"},
		{pcre, "", `\Z`, `\Z`, "end of the string (or before a final newline)"},
		{pcre, "D", `$`, `$`, "end of the string"},
		{py, "i", `ab`, `ab`, `"ab" literally (case-insensitive)`},
		{py, "", `a{2,5}?`, `a{2,5}?`, `"a" literally, between 2 and 5 times (lazy)`},
		{py, "", `a?`, `a?`, `"a" literally, 0 or 1 (greedy)`},
		{pcre, "", `a{3}`, `a{3}`, `"a" literally, exactly 3 times`},
		{py, "", `\t`, `\t`, "tab literally"},
		{pcre, "", `\p{Lu}`, `\p{Lu}`, `a uppercase letter (Unicode Lu)`},
		{py, "", `(a)(?P=x)`, `(?P=x)`, `same text as group "x"`},
	}
	for _, c := range cases {
		if got := find(t, c.f, c.flags, c.pattern, c.tok).Text; got != c.want {
			t.Errorf("%s %q %q: got %q, want %q", c.f, c.flags, c.pattern, got, c.want)
		}
	}
}

func TestGroupIndexAndSpans(t *testing.T) {
	rows := Explain(`a(b(c)d)e`, py, "")
	var got []int
	for _, r := range rows {
		got = append(got, r.Group)
		if r.Token == `(…)` && r.Span[1] <= r.Span[0] {
			t.Errorf("empty span on %+v", r)
		}
	}
	want := []int{0, 1, 1, 2, 2, 1, 0}
	if len(got) != len(want) {
		t.Fatalf("rows %v, want %v\n%s", got, want, Render(rows))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("groups %v, want %v\n%s", got, want, Render(rows))
		}
	}
	if r := rows[1]; r.Span != [2]int{1, 8} || r.Token != "(…)" {
		t.Errorf("group row = %+v", r)
	}
}

func TestUnknownRows(t *testing.T) {
	rows := Explain(`(?<=a)b`, gor, "")
	if !rows[0].Unknown || rows[0].Text != "can't explain this part" {
		t.Errorf("row 0 = %+v", rows[0])
	}
}

// Every pattern must give non-empty text and a valid span, whatever the kind.
func TestEveryRowHasText(t *testing.T) {
	pats := []string{
		`a|b`, `(?:x)`, `(?>x)`, `(?=x)(?!x)(?<=x)(?<!x)`, `(?|(a)|(b))`, `\1(?P<n>a)(?P=n)`, `(?R)(?1)(?&n)`, `\K`,
		`(?#c)`, `(?(1)a|b)`, `(?(?=a)b|c)`, `(?(DEFINE)a)`, `(*FAIL)(*ACCEPT)(?C1)(*sr:a)`, `[[:alpha:]\d\P{L}a-z]`,
		`\X\R\N\h\H\C`, `a**`, `(`, `[a`, `\q`, `()`, ``, `|`, `a|`, `(?i)`, `(?^i:a)`, `\G\A\z\b\B`, `\N{LATIN SMALL LETTER A}`,
	}
	for _, f := range engine.Flavors {
		for _, p := range pats {
			for _, r := range Explain(p, f, "") {
				if strings.TrimSpace(r.Text) == "" {
					t.Errorf("%s %q: empty text on %+v", f, p, r)
				}
				if r.Span[0] < 0 || r.Span[1] < r.Span[0] || r.Span[1] > len(p) {
					t.Errorf("%s %q: bad span on %+v", f, p, r)
				}
			}
		}
	}
}

func TestRenderGuides(t *testing.T) {
	got := Render(Explain(`(a(b)c)d`, py, ""))
	if !strings.Contains(got, "│  └─ b") || !strings.Contains(got, "└─ c") {
		t.Errorf("tree guides wrong:\n%s", got)
	}
}
