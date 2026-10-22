package main

import (
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/dor1202/lazyregex"
	"github.com/dor1202/lazyregex/internal/version"
)

// licenseTitles names each bundled file, in print order.
var licenseTitles = []struct{ file, title string }{
	{"CPython-PSF.txt", "CPython " + version.CPython + " (Python flavor) - PSF License"},
	{"PCRE2-BSD.txt", "PCRE2 " + version.PCRE2 + " (PCRE flavor) - BSD"},
	{"QuickJS-ng-MIT.txt", "QuickJS-ng " + version.QuickJSNG + " (JS flavor) - MIT"},
	{"wasi-libc.txt", "wasi-libc (statically linked into the WASM engines) - Apache-2.0 WITH LLVM-exception / Apache-2.0 / MIT, plus musl, cloudlibc and musl-fts"},
}

// printLicenses writes lazyregex's licence followed by every bundled one,
// each under a header.
func printLicenses(w io.Writer) error {
	fmt.Fprintf(w, "%s\nlazyregex - MIT\n%s\n\n%s\n", rule, rule, strings.TrimRight(lazyregex.Own, "\n"))
	for _, l := range licenseTitles {
		b, err := fs.ReadFile(lazyregex.Third, "LICENSES/"+l.file)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "\n%s\n%s\n%s\n\n%s\n", rule, l.title, rule, strings.TrimRight(string(b), "\n"))
	}
	return nil
}

var rule = strings.Repeat("=", 72)
