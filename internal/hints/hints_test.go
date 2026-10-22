package hints

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/dor1202/lazyregex/internal/engine"
)

func TestGoLookbehindRewrite(t *testing.T) {
	const pat = `(?<=\$)\d+(\.\d{2})?`
	h, ok := For(engine.Go, pat, "g", &engine.CompileError{Message: "x"})
	if !ok {
		t.Fatal("no hint")
	}
	if h.Rewrite != `\$(\d+(\.\d{2})?)` || h.RewriteNote != "read group 1" {
		t.Fatalf("got %+v", h)
	}
	if !strings.Contains(h.Why, "lookbehind") || !strings.Contains(h.Why, "linear-time") {
		t.Errorf("why = %q", h.Why)
	}
	// The rewrite compiles and group 1 gives the same amounts the lookbehind would.
	re := regexp.MustCompile(h.Rewrite)
	var got []string
	for _, m := range re.FindAllStringSubmatch("Total: $42.50, tax $3.10", -1) {
		got = append(got, m[1])
	}
	if want := []string{"42.50", "3.10"}; !reflect.DeepEqual(got, want) {
		t.Errorf("group 1 = %v, want %v", got, want)
	}
}

func TestGoTrailingLookaheadRewrite(t *testing.T) {
	h, ok := For(engine.Go, `\d+(?=%)`, "", nil)
	// The lookahead's text is consumed now: `\d(?=\d)` on "123" found 1 and 2;
	// `(\d)\d` finds only "12".
	if !ok || h.Rewrite != `(\d+)%` || h.RewriteNote != "read group 1; the lookahead's text is now consumed, so overlapping matches differ" {
		t.Fatalf("got %+v ok=%v", h, ok)
	}
}

func TestGoNoRewriteCases(t *testing.T) {
	for _, tc := range []struct{ pat, want string }{
		{`(?<!a)b`, "negative lookbehind"},
		{`(?!a)b`, "negative lookahead"},
		{`(a)\1`, "backreference"},
		{`(?>a)`, "atomic"},
		{`a++`, "possessive"},
		{`(?<=a)b|c`, "lookbehind"}, // top-level alternation: rewrite would change meaning
		{`(?<=(a))b`, "lookbehind"}, // would renumber captures
		{`a(?=b)c`, "lookahead"},    // not trailing
	} {
		h, ok := For(engine.Go, tc.pat, "", nil)
		if !ok || !strings.Contains(h.Why, tc.want) {
			t.Errorf("%s: got %+v ok=%v", tc.pat, h, ok)
		}
		if h.Rewrite != "" {
			t.Errorf("%s: unexpected rewrite %q", tc.pat, h.Rewrite)
		}
	}
}

func TestNoHintForValid(t *testing.T) {
	for _, f := range engine.Flavors {
		if h, ok := For(f, `(\w+)@\d+`, "g", nil); ok {
			t.Errorf("%s: unexpected %+v", f, h)
		}
	}
	if _, ok := For(engine.Python, `(?<=a)b`, "", &engine.CompileError{Message: "x"}); ok {
		t.Error("python should get no hint")
	}
}

func TestJSPythonNamedGroupHint(t *testing.T) {
	err := &engine.CompileError{Message: "Invalid regular expression: /(?P<y>\\d+)-(?P=y)/: Invalid capture group name"}
	h, ok := For(engine.JS, `(?P<y>\d+)-(?P=y)`, "", err)
	if !ok {
		t.Fatal("no hint")
	}
	if h.Why != "JavaScript uses (?<name>…) for named groups" {
		t.Errorf("why = %q", h.Why)
	}
	if h.Rewrite != `(?<y>\d+)-\k<y>` {
		t.Errorf("rewrite = %q", h.Rewrite)
	}
	if _, ok := For(engine.JS, `(?P<y>\d+)`, "", nil); ok {
		t.Error("no error, no hint")
	}
}

func TestGoRewriteKeepsMeaning(t *testing.T) {
	for _, tc := range []struct{ pat, rewrite, text, want string }{
		{`(?<=a|b)c`, `(?:a|b)(c)`, "ac bc cc", "c c"},
		{`x(?=b|c)`, `(x)(?:b|c)`, "xb xc xd", "x x"},
	} {
		h, ok := For(engine.Go, tc.pat, "", nil)
		if !ok || h.Rewrite != tc.rewrite {
			t.Fatalf("%s: got %+v ok=%v", tc.pat, h, ok)
		}
		var got []string
		for _, m := range regexp.MustCompile(h.Rewrite).FindAllStringSubmatch(tc.text, -1) {
			got = append(got, m[1])
		}
		if strings.Join(got, " ") != tc.want {
			t.Errorf("%s: group 1 = %v, want %q", tc.pat, got, tc.want)
		}
	}
}

func TestGoNoRewriteWithBareFlags(t *testing.T) {
	for _, pat := range []string{`(?i)x(?=b)`, `(?<=(?i)a)b`, `(?<=a)(?i)b`} {
		h, ok := For(engine.Go, pat, "", nil)
		if !ok || h.Rewrite != "" {
			t.Errorf("%s: got %+v ok=%v", pat, h, ok)
		}
	}
}
