// Command lazyregex is a terminal regex tester.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"

	"github.com/dor1202/lazyregex/internal/config"
	"github.com/dor1202/lazyregex/internal/engine/all"
	"github.com/dor1202/lazyregex/internal/runner"
	"github.com/dor1202/lazyregex/internal/ui"
	"github.com/dor1202/lazyregex/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// Test hooks: whether stdin and stdout are both terminals, where the config
// lives, and how the TUI is started.
var (
	configDir   = config.Dir
	interactive = func(stdin io.Reader, stdout io.Writer) bool { return isTTY(stdin) && isTTY(stdout) }
	runTUI      = func(d ui.Deps) error {
		_, err := tea.NewProgram(ui.New(d)).Run() // the view requests the alt screen
		return err
	}
)

func isTTY(v any) bool {
	f, ok := v.(*os.File)
	return ok && term.IsTerminal(f.Fd())
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lazyregex", flag.ContinueOnError)
	fs.SetOutput(stderr)
	showVersion := fs.Bool("version", false, "print version and exit")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *showVersion {
		fmt.Fprintf(stdout, "lazyregex %s\n", version.Version)
		return 0
	}
	if !interactive(stdin, stdout) {
		// Piped input and output arrive with Task 18.
		fmt.Fprintln(stdout, "lazyregex needs a terminal")
		return 0
	}

	// Load always returns usable values; reporting its errors is Task 18's.
	cfg := config.Defaults()
	dir, _ := configDir()
	var lib config.Library
	if dir != "" {
		cfg, _ = config.Load(dir)
		lib, _ = config.LoadLibrary(dir)
	}

	r := runner.New(all.Factories(), cacheDir(), cfg.Timeout)
	defer r.Close()

	err := runTUI(ui.Deps{
		Runner:    r,
		Config:    cfg,
		ConfigDir: dir,
		Library:   lib,
		Initial:   ui.InitialInput{Flavor: cfg.DefaultFlavor},
	})
	if err != nil {
		fmt.Fprintln(stderr, "lazyregex:", err)
		return 1
	}
	return 0
}

// cacheDir is where wazero keeps compiled WASM modules; it falls back to a
// temp dir when the user cache dir is unavailable.
func cacheDir() string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "lazyregex", "wazero")
}
