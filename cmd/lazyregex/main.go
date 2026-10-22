// Command lazyregex is a terminal regex tester.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"

	"github.com/dor1202/lazyregex/internal/config"
	"github.com/dor1202/lazyregex/internal/engine"
	"github.com/dor1202/lazyregex/internal/engine/all"
	"github.com/dor1202/lazyregex/internal/explain"
	"github.com/dor1202/lazyregex/internal/runner"
	"github.com/dor1202/lazyregex/internal/termsafe"
	"github.com/dor1202/lazyregex/internal/ui"
	"github.com/dor1202/lazyregex/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// maxStdin caps how much piped input is read; the rest is dropped with a
// note on stderr (the TUI owns the screen, so a startup notice there would
// be lost).
const maxStdin = 64 << 20

// flagLetters are the regex flag letters each flavor accepts.
var flagLetters = map[engine.Flavor]string{
	engine.Python: "gimsxa",
	engine.JS:     "gdimsuvy",
	engine.PCRE:   "gimsxnUu",
	engine.Go:     "gimsU",
}

// Test hooks: whether stdout is a terminal, where the config lives, how the
// TUI is started (with keyboard input from in), and where keyboard input
// comes from when stdin is piped.
var (
	configDir   = config.Dir
	interactive = func(stdout io.Writer) bool { return isTTY(stdout) }
	runTUI      = func(d ui.Deps, in io.Reader) error {
		opts := []tea.ProgramOption{}
		if in != nil {
			opts = append(opts, tea.WithInput(in))
		}
		_, err := tea.NewProgram(ui.New(d), opts...).Run() // the view requests the alt screen
		return err
	}
	openTTY = func() (io.Reader, func(), error) {
		in, out, err := tea.OpenTTY()
		if err != nil {
			return nil, nil, err
		}
		// On Unix in and out are the same *os.File (one /dev/tty handle):
		// close it once, and only after the program has finished with it.
		return in, func() {
			in.Close()
			if out != in {
				out.Close()
			}
		}, nil
	}
)

func isTTY(v any) bool {
	f, ok := v.(*os.File)
	return ok && term.IsTerminal(f.Fd())
}

const usage = `usage: lazyregex [flags] [pattern]

A terminal regex tester. Text piped on stdin becomes the test string.

flags:
  -f, --flavor NAME   python (default), js, pcre or go
  -p, --pattern RE    initial pattern (or give it as the first argument)
      --flags LETTERS regex flags: python gimsxa, js gdimsuvy, pcre gimsxnUu, go gimsU
      --explain       print a plain-English explanation of the pattern and exit
      --licenses      print the licences of lazyregex and its bundled engines
      --version       print the version and the pinned engine versions
  -h, --help          show this help

exit codes: 0 ok, 1 runtime error, 2 usage error
`

func usageErr(stderr io.Writer, format string, a ...any) int {
	fmt.Fprintf(stderr, "lazyregex: "+format+"\n", a...)
	return 2
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lazyregex", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		flavorArg, pattern, flags string
		showVersion, doExplain    bool
		doLicenses, help          bool
	)
	fs.StringVar(&flavorArg, "flavor", "", "")
	fs.StringVar(&flavorArg, "f", "", "")
	fs.StringVar(&pattern, "pattern", "", "")
	fs.StringVar(&pattern, "p", "", "")
	fs.StringVar(&flags, "flags", "", "")
	fs.BoolVar(&doExplain, "explain", false, "")
	fs.BoolVar(&doLicenses, "licenses", false, "")
	fs.BoolVar(&showVersion, "version", false, "")
	fs.BoolVar(&help, "help", false, "")
	fs.BoolVar(&help, "h", false, "")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "lazyregex: %v\n\n%s", err, usage)
		return 2
	}
	if help {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if showVersion {
		printVersion(stdout)
		return 0
	}
	if doLicenses {
		if err := printLicenses(stdout); err != nil {
			fmt.Fprintln(stderr, "lazyregex:", err)
			return 1
		}
		return 0
	}

	patternSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "p" || f.Name == "pattern" {
			patternSet = true
		}
	})
	switch rest := fs.Args(); {
	case len(rest) > 1:
		return usageErr(stderr, "too many arguments (want one pattern)")
	case len(rest) == 1 && patternSet:
		return usageErr(stderr, "pattern given both with -p and as an argument")
	case len(rest) == 1:
		pattern, patternSet = rest[0], true
	}

	// Load always returns usable values; its errors are reported, not fatal.
	cfg := config.Defaults()
	var lib config.Library
	var notes []string // shown in the UI as well as on stderr
	dir := ""
	if !doExplain {
		dir, _ = configDir()
		if dir != "" {
			var err error
			if cfg, err = config.Load(dir); err != nil {
				fmt.Fprintln(stderr, "lazyregex:", err)
				notes = append(notes, err.Error())
			}
			if lib, err = config.LoadLibrary(dir); err != nil {
				fmt.Fprintln(stderr, "lazyregex:", err)
				notes = append(notes, err.Error())
			}
		}
	}

	flavor := cfg.DefaultFlavor
	if flavorArg != "" {
		flavor = engine.Flavor(flavorArg)
		if _, ok := flagLetters[flavor]; !ok {
			return usageErr(stderr, "unknown flavor %q (want python, js, pcre, go)", flavorArg)
		}
	}
	for _, c := range flags {
		if !strings.ContainsRune(flagLetters[flavor], c) {
			return usageErr(stderr, "unknown flag %q for %s (want %s)", string(c), flavor, flagLetters[flavor])
		}
	}

	if doExplain {
		if !patternSet {
			return usageErr(stderr, "--explain needs a pattern (-p or an argument)")
		}
		out := explain.Render(explain.Explain(pattern, flavor, flags))
		if interactive(stdout) {
			out = termsafe.Lines(out)
		}
		fmt.Fprint(stdout, out)
		if !strings.HasSuffix(out, "\n") {
			fmt.Fprintln(stdout)
		}
		return 0
	}

	if !interactive(stdout) {
		fmt.Fprintln(stdout, "lazyregex needs a terminal")
		return 0
	}

	initial := ui.InitialInput{Flavor: flavor, Pattern: pattern, Flags: flags}
	var in io.Reader // nil: Bubble Tea reads stdin itself
	if !isTTY(stdin) {
		tty, closeTTY, err := openTTY()
		if err != nil {
			fmt.Fprintln(stderr, "lazyregex: no terminal for interactive input")
			return 1
		}
		defer closeTTY()
		in = tty
		data, err := io.ReadAll(io.LimitReader(stdin, maxStdin+1))
		if err != nil {
			fmt.Fprintln(stderr, "lazyregex: reading stdin:", err)
			return 1
		}
		if len(data) > maxStdin {
			data = data[:maxStdin]
			msg := fmt.Sprintf("stdin truncated to %d MiB", maxStdin>>20)
			fmt.Fprintln(stderr, "lazyregex:", msg)
			notes = append(notes, msg)
		}
		initial.TextPrepared = runner.PrepareText(data)
	}

	initial.Notice = strings.ReplaceAll(strings.Join(notes, "; "), "\n", " ")

	r := runner.New(all.Factories(), cacheDir(), cfg.Timeout)
	defer r.Close()

	err := runTUI(ui.Deps{
		Runner:    r,
		Config:    cfg,
		ConfigDir: dir,
		Library:   lib,
		Initial:   initial,
	}, in)
	if err != nil {
		fmt.Fprintln(stderr, "lazyregex:", err)
		return 1
	}
	return 0
}

// printVersion writes lazyregex's version and the pinned engine versions
// (without starting the engines).
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "lazyregex %s\n", version.String())
	for _, e := range [][2]string{
		{"python", "CPython " + version.CPython + " (re)"},
		{"js", "QuickJS-ng " + version.QuickJSNG},
		{"pcre", "PCRE2 " + version.PCRE2},
		{"go", "Go regexp (RE2) " + runtime.Version()},
	} {
		fmt.Fprintf(w, "%-7s %s\n", e[0], e[1])
	}
}

// cacheDir is where wazero keeps compiled WASM modules (native code). When
// the user cache dir is unavailable it returns "" (in-memory compilation
// only): a shared temp dir could be pre-created by another user and poisoned.
func cacheDir() string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		return ""
	}
	return filepath.Join(base, "lazyregex", "wazero")
}
