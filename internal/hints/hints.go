// Package hints explains why a pattern fails in a flavor and, when a small
// mechanical change fixes it, offers a rewrite to try.
package hints

import (
	"regexp"
	"strings"

	"github.com/dor1202/lazyregex/internal/engine"
	"github.com/dor1202/lazyregex/internal/explain"
)

// Hint is a "Why / Try" suggestion shown in the Compare tab.
type Hint struct {
	Why         string
	Rewrite     string // "" if there is no mechanical rewrite
	RewriteNote string
}

const re2 = "Go uses RE2, which guarantees linear-time matching and so leaves it out."

// For returns a hint for a pattern that fails in flavor f. err is the engine's
// compile error, if any; it is only consulted for JavaScript.
func For(f engine.Flavor, pattern string, flags string, err *engine.CompileError) (Hint, bool) {
	switch f {
	case engine.Go:
		return goHint(pattern, flags)
	case engine.JS:
		return jsHint(pattern, err)
	}
	return Hint{}, false
}

func jsHint(pattern string, err *engine.CompileError) (Hint, bool) {
	if err == nil || !strings.Contains(pattern, "(?P<") {
		return Hint{}, false
	}
	rw := strings.ReplaceAll(pattern, "(?P<", "(?<")
	// (?P=name) back-references become \k<name>.
	rw = pyBackref.ReplaceAllString(rw, `\k<$1>`)
	return Hint{
		Why:         "JavaScript uses (?<name>…) for named groups",
		Rewrite:     rw,
		RewriteNote: "(?P<name>…) became (?<name>…)",
	}, true
}

var pyBackref = regexp.MustCompile(`\(\?P=(\w+)\)`)

func goHint(pattern, flags string) (Hint, bool) {
	root := explain.Parse(pattern, explain.DialectFor(engine.Go, flags))
	name := findUnsupported(root)
	if name == "" {
		return Hint{}, false
	}
	h := Hint{Why: "Go does not support " + name + ". " + re2}
	if rw, ok := goRewrite(root, pattern, flags); ok {
		h.Rewrite, h.RewriteNote = rw, "read group 1"
	}
	return h, true
}

// findUnsupported names the first construct RE2 deliberately lacks, or "".
func findUnsupported(n *explain.Node) string {
	for i, c := range n.Children {
		if c.Kind == explain.Unknown {
			if name := construct(c, n, i); name != "" {
				return name
			}
		}
		if name := findUnsupported(c); name != "" {
			return name
		}
	}
	return ""
}

// construct names the unsupported construct, or "" for other Unknown nodes.
func construct(n, parent *explain.Node, idx int) string {
	r := n.Raw
	switch {
	case strings.HasPrefix(r, "(?<="):
		return "lookbehind (?<=…)"
	case strings.HasPrefix(r, "(?<!"):
		return "negative lookbehind (?<!…)"
	case strings.HasPrefix(r, "(?="):
		return "lookahead (?=…)"
	case strings.HasPrefix(r, "(?!"):
		return "negative lookahead (?!…)"
	case strings.HasPrefix(r, "(?>"):
		return "atomic groups (?>…)"
	case len(r) == 2 && r[0] == '\\' && r[1] >= '1' && r[1] <= '9':
		return "backreferences (" + r + ")"
	case r == "+" && idx > 0 && parent.Children[idx-1].Kind == explain.Quantifier &&
		parent.Children[idx-1].Span[1] == n.Span[0]:
		return "possessive quantifiers (a*+, a++, a?+)"
	}
	return ""
}

// goRewrite handles a leading (?<=A)B -> A(B) and a trailing A(?=B) -> (A)B.
// It only fires on a top-level sequence where the moved part has no capture
// groups, and only when the result compiles in Go.
func goRewrite(root *explain.Node, pattern, flags string) (string, bool) {
	if root.Kind != explain.Sequence || len(root.Children) < 2 {
		return "", false
	}
	if hasBareFlags(root) {
		// Regrouping would change how far an inline (?i) reaches.
		return "", false
	}
	var rw string
	first, last := root.Children[0], root.Children[len(root.Children)-1]
	switch {
	case first.Kind == explain.Unknown && strings.HasPrefix(first.Raw, "(?<=") &&
		strings.HasSuffix(first.Raw, ")") && first.Span[0] == 0:
		if hasCapture(first) {
			return "", false
		}
		rw = body(first.Raw[len("(?<="):len(first.Raw)-1]) + "(" + pattern[first.Span[1]:] + ")"
	case last.Kind == explain.Unknown && strings.HasPrefix(last.Raw, "(?=") &&
		strings.HasSuffix(last.Raw, ")") && last.Span[1] == len(pattern):
		if hasCapture(last) {
			return "", false
		}
		rw = "(" + pattern[:last.Span[0]] + ")" + body(last.Raw[len("(?="):len(last.Raw)-1])
	default:
		return "", false
	}
	if _, err := regexp.Compile(rw); err != nil {
		return "", false
	}
	return rw, true
}

func hasCapture(n *explain.Node) bool {
	for _, c := range n.Children {
		if c.Kind == explain.Capture || c.Kind == explain.Named || hasCapture(c) {
			return true
		}
	}
	return false
}

// body wraps a lookaround body that has a top-level "|" so that moving it
// next to other text keeps the alternation inside its own group.
func body(b string) string {
	if explain.Parse(b, explain.DialectFor(engine.Go, "")).Kind == explain.Alternation {
		return "(?:" + b + ")"
	}
	return b
}

// hasBareFlags reports a (?i)-style group without a body anywhere in n.
func hasBareFlags(n *explain.Node) bool {
	if n.Kind == explain.InlineFlags && len(n.Children) == 0 {
		return true
	}
	for _, c := range n.Children {
		if hasBareFlags(c) {
			return true
		}
	}
	return false
}
