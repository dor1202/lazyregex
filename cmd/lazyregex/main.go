// Command lazyregex is a terminal regex tester.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/dor1202/lazyregex/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	_ = stdin
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
	fmt.Fprintln(stdout, "TUI not implemented")
	return 0
}
