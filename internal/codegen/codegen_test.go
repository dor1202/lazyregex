package codegen

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/dor1202/lazyregex/internal/engine"
)

var update = flag.Bool("update", false, "rewrite golden files")

const sample = "ann@x.com, Bob@y.org\nzed@z.net"

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Tolerate CRLF checkouts (Windows autocrlf).
	want = bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n"))
	if string(want) != got {
		t.Errorf("%s mismatch\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestSnippetGolden(t *testing.T) {
	repl := map[engine.Flavor]string{engine.Python: `\2@\1`, engine.JS: "$2@$1", engine.PCRE: "$2@$1", engine.Go: "${2}@${1}"}
	for _, f := range engine.Flavors {
		cases := []struct {
			name string
			req  engine.Request
		}{
			{"match", engine.Request{Pattern: `(\w+)@(\w+)\.\w+`, Flags: "gim", Text: sample, Mode: engine.ModeMatch}},
			{"replace", engine.Request{Pattern: `(\w+)@(\w+)\.\w+`, Flags: "", Text: sample, Mode: engine.ModeReplace, Replacement: repl[f]}},
			{"split", engine.Request{Pattern: `,\s*|/`, Flags: "g", Text: "a, b/c", Mode: engine.ModeSplit}},
		}
		for _, c := range cases {
			t.Run(string(f)+"_"+c.name, func(t *testing.T) {
				golden(t, string(f)+"_"+c.name, Snippet(f, c.req))
			})
		}
	}
}

func TestPythonTrailingBackslash(t *testing.T) {
	got := Snippet(engine.Python, engine.Request{Pattern: `a\\\`, Text: `a\`, Mode: engine.ModeMatch})
	golden(t, "python_trailing_backslash", got)
}

func TestPythonQuoteFallback(t *testing.T) {
	for in, want := range map[string]string{
		`a\d`:   `r"a\d"`,
		`a\\`:   `r"a\\"`,
		`a\`:    `"a\\"`,
		`say"x`: `"say\"x"`,
		"a\nb":  `"a\nb"`,
	} {
		if got := pyString(in, true); got != want {
			t.Errorf("pyString(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestSlashEscaping(t *testing.T) {
	for in, want := range map[string]string{`a/b`: `a\/b`, `a\/b`: `a\/b`, `a\\/b`: `a\\\/b`, `a\`: `a\`} {
		if got := escapeSlashes(in); got != want {
			t.Errorf("escapeSlashes(%q) = %q, want %q", in, got, want)
		}
	}
}
