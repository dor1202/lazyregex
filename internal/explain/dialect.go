package explain

import (
	"strings"

	"github.com/dor1202/lazyregex/internal/engine"
)

// Dialect lists what a flavor's syntax supports. Escape details that differ
// letter by letter are decided in escape.go by Flavor.
type Dialect struct {
	Flavor engine.Flavor

	// NamedGroup lists the accepted named-group openers: "(?P<", "(?<", "(?'".
	NamedGroup []string
	// NamedBackref lists the accepted named-backreference openers:
	// "(?P=", `\k<`, `\k'`, `\k{`, `\g{`.
	NamedBackref []string

	Lookahead, Lookbehind bool
	Atomic, Possessive    bool
	Recursion             bool // (?R) (?1) (?&n) (?P>n) \g<n>
	KeepOut               bool // \K
	BranchReset           bool // (?|...)
	UnicodeProps          bool // \p{...} \P{...}
	Backrefs              bool // \1
	Conditionals          bool // (?(1)a|b)
	InlineComments        bool // (?#...)
	Verbs                 bool // (*VERB), (*pla:...), (?C...)
	Quoting               bool // \Q...\E
	POSIXClasses          bool // [[:alpha:]]
	BraceMinOmitted       bool // {,n} is a quantifier
	BraceNoBounds         bool // {,} is a quantifier ({0,})
	BraceSpaces           bool // spaces allowed inside {n,m}

	FlagLetters string // letters allowed in (?flags) and (?flags:...)
	BareFlags   bool   // (?i) without ":" is allowed

	// State from the flags string.
	Extended      bool // x: whitespace and #comments are insignificant
	NoAutoCapture bool // n: plain (...) does not capture
	Ungreedy      bool // U: quantifiers are lazy unless followed by ?
	Unicode       bool // JS u or v
	UnicodeSets   bool // JS v
}

// DialectFor returns the dialect for flavor f with the given flag letters.
// The x flag enables extended mode for Python and PCRE; JS and Go have none.
func DialectFor(f engine.Flavor, flags string) Dialect {
	has := func(c string) bool { return strings.Contains(flags, c) }
	d := Dialect{Flavor: f}
	switch f {
	case engine.Python:
		d.NamedGroup = []string{"(?P<"}
		d.NamedBackref = []string{"(?P="}
		d.Lookahead, d.Lookbehind = true, true
		d.Atomic, d.Possessive = true, true
		d.Backrefs, d.Conditionals, d.InlineComments = true, true, true
		d.BraceMinOmitted, d.BraceNoBounds = true, true
		d.FlagLetters, d.BareFlags = "aiLmsux", true
		d.Extended = has("x")
	case engine.JS:
		d.NamedGroup = []string{"(?<"}
		d.NamedBackref = []string{`\k<`}
		d.Lookahead, d.Lookbehind = true, true
		d.Backrefs = true
		d.FlagLetters = "ims"
		d.Unicode = has("u") || has("v")
		d.UnicodeSets = has("v")
		d.UnicodeProps = d.Unicode
	case engine.PCRE:
		d.NamedGroup = []string{"(?<", "(?'", "(?P<"}
		d.NamedBackref = []string{"(?P=", `\k<`, `\k'`, `\k{`, `\g{`}
		d.Lookahead, d.Lookbehind = true, true
		d.Atomic, d.Possessive, d.Recursion, d.KeepOut, d.BranchReset = true, true, true, true, true
		d.UnicodeProps, d.Backrefs, d.Conditionals, d.InlineComments = true, true, true, true
		d.Verbs, d.Quoting, d.POSIXClasses = true, true, true
		d.BraceMinOmitted, d.BraceSpaces = true, true
		d.FlagLetters, d.BareFlags = "imnsxJUaDSWPTr^", true
		d.Extended = has("x")
		d.NoAutoCapture = has("n")
		d.Ungreedy = has("U")
	case engine.Go:
		d.NamedGroup = []string{"(?P<", "(?<"}
		d.UnicodeProps, d.Quoting, d.POSIXClasses = true, true, true
		d.FlagLetters, d.BareFlags = "imsU", true
		d.Ungreedy = has("U")
	}
	return d
}

func (d Dialect) acceptsNamedGroup(opener string) bool {
	for _, o := range d.NamedGroup {
		if o == opener {
			return true
		}
	}
	return false
}

func (d Dialect) acceptsNamedBackref(opener string) bool {
	for _, o := range d.NamedBackref {
		if o == opener {
			return true
		}
	}
	return false
}
