package explain

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/dor1202/lazyregex/internal/engine"
)

const (
	py   = engine.Python
	js   = engine.JS
	pcre = engine.PCRE
	gor  = engine.Go
)

// dump renders a tree as Kind@start-end{payload}[children].
func dump(n *Node) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%v@%d-%d", n.Kind, n.Span[0], n.Span[1])
	var p []string
	switch n.Kind {
	case Anchor, Unknown, Comment:
		p = append(p, strconv.Quote(n.Raw))
	}
	switch {
	case n.Kind == Literal && n.Index < 0:
		p = append(p, "?")
	case n.Kind == Literal:
		p = append(p, strconv.QuoteRune(rune(n.Index)))
	case n.Index != 0 || n.Kind == Capture || n.Kind == Named || n.Kind == Backref || n.Kind == Recursion:
		p = append(p, fmt.Sprintf("#%d", n.Index))
	}
	if n.Name != "" {
		p = append(p, "name="+n.Name)
	}
	if n.Kind == Quantifier {
		max := strconv.Itoa(n.Max)
		if n.Max == -1 {
			max = "inf"
		}
		p = append(p, fmt.Sprintf("%d..%s %s", n.Min, max, n.Greedy))
	}
	if n.Negated {
		p = append(p, "neg")
	}
	if n.Flags != "" {
		p = append(p, "flags="+n.Flags)
	}
	for _, it := range n.ClassItems {
		p = append(p, itemString(it))
	}
	if len(p) > 0 {
		b.WriteString("{" + strings.Join(p, " ") + "}")
	}
	if len(n.Children) > 0 {
		var cs []string
		for _, c := range n.Children {
			cs = append(cs, dump(c))
		}
		b.WriteString("[" + strings.Join(cs, " ") + "]")
	}
	return b.String()
}

func itemString(it ClassItem) string {
	switch {
	case it.Strings != nil:
		return `\q{` + strings.Join(it.Strings, "|") + `}`
	case it.Prop != "":
		if it.Negated {
			return `\P{` + it.Prop + `}`
		}
		return `\p{` + it.Prop + `}`
	case strings.HasPrefix(it.Shorthand, "[:"):
		if it.Negated {
			return "^" + it.Shorthand
		}
		return it.Shorthand
	case it.Shorthand != "":
		if it.Negated {
			return `\` + strings.ToUpper(it.Shorthand)
		}
		return `\` + it.Shorthand
	case it.Lo == it.Hi:
		return strconv.QuoteRune(it.Lo)
	}
	return strconv.QuoteRune(it.Lo) + "-" + strconv.QuoteRune(it.Hi)
}

type row struct {
	flavors []engine.Flavor
	flags   string
	pattern string
	want    string
}

var all = []engine.Flavor{py, js, pcre, gor}

func fl(f ...engine.Flavor) []engine.Flavor { return f }

var rows = []row{
	// Brief rows.
	{fl(py), "", `^(?P<date>\d{4})$`, `Sequence@0-17[Anchor@0-1{"^"} Named@1-16{#1 name=date}[Quantifier@10-15{4..4 greedy}[Shorthand@10-12{name=d}]] Anchor@16-17{"$"}]`},
	{fl(js), "", `(?<=\$)\d+`, `Sequence@0-10[LookBehind@0-7[Literal@4-6{'$'}] Quantifier@7-10{1..inf greedy}[Shorthand@7-9{name=d}]]`},
	{fl(gor), "", `(?<=\$)\d+`, `Sequence@0-10[Unknown@0-7{"(?<=\\$)"}[Literal@4-6{'$'}] Quantifier@7-10{1..inf greedy}[Shorthand@7-9{name=d}]]`},
	{fl(pcre, py), "", `a++`, `Sequence@0-3[Quantifier@0-3{1..inf possessive}[Literal@0-1{'a'}]]`},
	{fl(js, gor), "", `a++`, `Sequence@0-3[Quantifier@0-2{1..inf greedy}[Literal@0-1{'a'}] Unknown@2-3{"+"}]`},
	{fl(pcre), "", `(?R)`, `Sequence@0-4[Recursion@0-4{#0}]`},
	{fl(py, js, gor), "", `(?R)`, `Sequence@0-4[Unknown@0-4{"(?R)"}]`},
	{all, "", `[^a-z\d]`, `Sequence@0-8[CharClass@0-8{neg 'a'-'z' \d}]`},
	{fl(js), "u", `\p{L}`, `Sequence@0-5[UnicodeProp@0-5{name=L}]`},
	{fl(js), "v", `\p{L}`, `Sequence@0-5[UnicodeProp@0-5{name=L}]`},
	{fl(js), "", `\p{L}`, `Sequence@0-5[Literal@0-2{'p'} Literal@2-3{'{'} Literal@3-4{'L'} Literal@4-5{'}'}]`},
	{all, "", `(?i:abc)`, `Sequence@0-8[InlineFlags@0-8{flags=i}[Literal@4-5{'a'} Literal@5-6{'b'} Literal@6-7{'c'}]]`},
	{all, "", `a|b|c`, `Alternation@0-5[Sequence@0-1[Literal@0-1{'a'}] Sequence@2-3[Literal@2-3{'b'}] Sequence@4-5[Literal@4-5{'c'}]]`},

	// Structure.
	{all, "", `a|`, `Alternation@0-2[Sequence@0-1[Literal@0-1{'a'}] Sequence@2-2]`},
	{all, "", ``, `Sequence@0-0`},
	{all, "", `(a|b)`, `Sequence@0-5[Capture@0-5{#1}[Alternation@1-4[Sequence@1-2[Literal@1-2{'a'}] Sequence@3-4[Literal@3-4{'b'}]]]]`},
	{all, "", `.`, `Sequence@0-1[Dot@0-1]`},
	{all, "", `é+`, `Sequence@0-3[Quantifier@0-3{1..inf greedy}[Literal@0-2{'é'}]]`},
	{all, "", `(a`, `Sequence@0-2[Capture@0-2{#1}[Literal@1-2{'a'}]]`},
	{all, "", `a)`, `Sequence@0-2[Literal@0-1{'a'} Unknown@1-2{")"}]`},
	{all, "", `a\`, `Sequence@0-2[Literal@0-1{'a'} Unknown@1-2{"\\"}]`},
	{all, "", `[a`, `Sequence@0-2[Unknown@0-2{"[a"}]`},
	{all, "", `(?:a)`, `Sequence@0-5[NonCapture@0-5[Literal@3-4{'a'}]]`},

	// Named groups: python (?P< only; JS (?< only; Go both; PCRE three forms.
	{fl(py), "", `(?<n>a)`, `Sequence@0-7[Unknown@0-7{"(?<n>a)"}[Literal@5-6{'a'}]]`},
	{fl(js, gor, pcre), "", `(?<n>a)`, `Sequence@0-7[Named@0-7{#1 name=n}[Literal@5-6{'a'}]]`},
	{fl(py, gor, pcre), "", `(?P<n>a)`, `Sequence@0-8[Named@0-8{#1 name=n}[Literal@6-7{'a'}]]`},
	{fl(js), "", `(?P<n>a)`, `Sequence@0-8[Unknown@0-8{"(?P<n>a)"}[Literal@6-7{'a'}]]`},
	{fl(pcre), "", `(?'n'a)`, `Sequence@0-7[Named@0-7{#1 name=n}[Literal@5-6{'a'}]]`},
	{fl(py), "", `(a)(?P<x>b)(c)`, `Sequence@0-14[Capture@0-3{#1}[Literal@1-2{'a'}] Named@3-11{#2 name=x}[Literal@9-10{'b'}] Capture@11-14{#3}[Literal@12-13{'c'}]]`},

	// Lookaround.
	{fl(py, js, pcre), "", `(?=a)(?!b)`, `Sequence@0-10[LookAhead@0-5[Literal@3-4{'a'}] NegLookAhead@5-10[Literal@8-9{'b'}]]`},
	{fl(gor), "", `(?=a)`, `Sequence@0-5[Unknown@0-5{"(?=a)"}[Literal@3-4{'a'}]]`},
	{fl(py, js, pcre), "", `(?<!a)`, `Sequence@0-6[NegLookBehind@0-6[Literal@4-5{'a'}]]`},

	// Atomic, \K, recursion, branch reset.
	{fl(py, pcre), "", `(?>a)`, `Sequence@0-5[Atomic@0-5[Literal@3-4{'a'}]]`},
	{fl(js, gor), "", `(?>a)`, `Sequence@0-5[Unknown@0-5{"(?>a)"}[Literal@3-4{'a'}]]`},
	{fl(pcre), "", `a\Kb`, `Sequence@0-4[Literal@0-1{'a'} KeepOut@1-3 Literal@3-4{'b'}]`},
	{fl(py, gor), "", `a\Kb`, `Sequence@0-4[Literal@0-1{'a'} Unknown@1-3{"\\K"} Literal@3-4{'b'}]`},
	{fl(js), "", `a\Kb`, `Sequence@0-4[Literal@0-1{'a'} Literal@1-3{'K'} Literal@3-4{'b'}]`},
	{fl(pcre), "", `(a)(?1)(?-1)(?+1)(b)`, `Sequence@0-20[Capture@0-3{#1}[Literal@1-2{'a'}] Recursion@3-7{#1} Recursion@7-12{#1} Recursion@12-17{#2} Capture@17-20{#2}[Literal@18-19{'b'}]]`},
	{fl(pcre), "", `(?<n>a)(?&n)\g<n>(?P>n)`, `Sequence@0-23[Named@0-7{#1 name=n}[Literal@5-6{'a'}] Recursion@7-12{#1 name=n} Recursion@12-17{#1 name=n} Recursion@17-23{#1 name=n}]`},
	{fl(pcre), "", `(?|(a)|(b))(c)`, `Sequence@0-14[BranchReset@0-11[Alternation@3-10[Sequence@3-6[Capture@3-6{#1}[Literal@4-5{'a'}]] Sequence@7-10[Capture@7-10{#1}[Literal@8-9{'b'}]]]] Capture@11-14{#2}[Literal@12-13{'c'}]]`},
	{fl(pcre), "n", `(a)(?<x>b)`, `Sequence@0-10[NonCapture@0-3[Literal@1-2{'a'}] Named@3-10{#1 name=x}[Literal@8-9{'b'}]]`},
	{fl(pcre), "", `(?n)(a)`, `Sequence@0-7[InlineFlags@0-4{flags=n} NonCapture@4-7[Literal@5-6{'a'}]]`},

	// Backreferences.
	{fl(py, js, pcre), "", `(a)\1`, `Sequence@0-5[Capture@0-3{#1}[Literal@1-2{'a'}] Backref@3-5{#1}]`},
	{fl(gor), "", `(a)\1`, `Sequence@0-5[Capture@0-3{#1}[Literal@1-2{'a'}] Unknown@3-5{"\\1"}]`},
	{fl(js), "", `\1`, `Sequence@0-2[Literal@0-2{'\x01'}]`},
	{fl(js), "u", `\1`, `Sequence@0-2[Backref@0-2{#1}]`},
	{fl(pcre), "", `(a)\g{-1}\g1`, `Sequence@0-12[Capture@0-3{#1}[Literal@1-2{'a'}] Backref@3-9{#1} Backref@9-12{#1}]`},
	{fl(pcre), "", `\10`, `Sequence@0-3[Literal@0-3{'\b'}]`},
	{fl(py), "", `\101\0`, `Sequence@0-6[Literal@0-4{'A'} Literal@4-6{'\x00'}]`},
	{fl(gor), "", `\012\8`, `Sequence@0-6[Literal@0-4{'\n'} Unknown@4-6{"\\8"}]`},
	{fl(py), "", `(?P<n>a)(?P=n)`, `Sequence@0-14[Named@0-8{#1 name=n}[Literal@6-7{'a'}] NamedBackref@8-14{#1 name=n}]`},
	{fl(js), "", `(?<n>a)\k<n>`, `Sequence@0-12[Named@0-7{#1 name=n}[Literal@5-6{'a'}] NamedBackref@7-12{#1 name=n}]`},
	{fl(js), "", `\k<n>`, `Sequence@0-5[Literal@0-2{'k'} Literal@2-3{'<'} Literal@3-4{'n'} Literal@4-5{'>'}]`},
	{fl(js), "u", `\k<n>`, `Sequence@0-5[NamedBackref@0-5{name=n}]`},
	{fl(pcre), "", `(?<n>a)\k{n}\g{n}\k'n'`, `Sequence@0-22[Named@0-7{#1 name=n}[Literal@5-6{'a'}] NamedBackref@7-12{#1 name=n} NamedBackref@12-17{#1 name=n} NamedBackref@17-22{#1 name=n}]`},

	// Quantifiers.
	{fl(py, pcre), "", `a{,3}`, `Sequence@0-5[Quantifier@0-5{0..3 greedy}[Literal@0-1{'a'}]]`},
	{fl(js, gor), "", `a{,3}`, `Sequence@0-5[Literal@0-1{'a'} Literal@1-2{'{'} Literal@2-3{','} Literal@3-4{'3'} Literal@4-5{'}'}]`},
	{fl(py), "", `a{,}`, `Sequence@0-4[Quantifier@0-4{0..inf greedy}[Literal@0-1{'a'}]]`},
	{fl(pcre), "", `a{,}`, `Sequence@0-4[Literal@0-1{'a'} Literal@1-2{'{'} Literal@2-3{','} Literal@3-4{'}'}]`},
	{fl(pcre), "", `a{ 2 }`, `Sequence@0-6[Quantifier@0-6{2..2 greedy}[Literal@0-1{'a'}]]`},
	{fl(py), "", `a{ 2 }`, `Sequence@0-6[Literal@0-1{'a'} Literal@1-2{'{'} Literal@2-3{' '} Literal@3-4{'2'} Literal@4-5{' '} Literal@5-6{'}'}]`},
	{all, "", `a{2,}b{2,3}?`, `Sequence@0-12[Quantifier@0-5{2..inf greedy}[Literal@0-1{'a'}] Quantifier@5-12{2..3 lazy}[Literal@5-6{'b'}]]`},
	{all, "", `a**`, `Sequence@0-3[Quantifier@0-2{0..inf greedy}[Literal@0-1{'a'}] Unknown@2-3{"*"}]`},
	{all, "", `a{2}{3}`, `Sequence@0-7[Quantifier@0-4{2..2 greedy}[Literal@0-1{'a'}] Unknown@4-7{"{3}"}]`},
	{all, "", `*a`, `Sequence@0-2[Unknown@0-1{"*"} Literal@1-2{'a'}]`},
	{fl(gor), "U", `a*a*?`, `Sequence@0-5[Quantifier@0-2{0..inf lazy}[Literal@0-1{'a'}] Quantifier@2-5{0..inf greedy}[Literal@2-3{'a'}]]`},
	{fl(pcre), "", `(?U)a+`, `Sequence@0-6[InlineFlags@0-4{flags=U} Quantifier@4-6{1..inf lazy}[Literal@4-5{'a'}]]`},

	// Inline flags.
	{fl(js), "", `(?i)a`, `Sequence@0-5[Unknown@0-4{"(?i)"} Literal@4-5{'a'}]`},
	{fl(py, pcre, gor), "", `(?i)a`, `Sequence@0-5[InlineFlags@0-4{flags=i} Literal@4-5{'a'}]`},
	{fl(js, pcre, gor, py), "", `(?s-i:a)`, `Sequence@0-8[InlineFlags@0-8{flags=s-i}[Literal@6-7{'a'}]]`},
	{fl(gor), "", `(?x)a`, `Sequence@0-5[Unknown@0-4{"(?x)"} Literal@4-5{'a'}]`},

	// Extended mode: whitespace skipped, #... becomes a Comment node.
	{fl(py, pcre), "x", "a b # c\nd", `Sequence@0-9[Literal@0-1{'a'} Literal@2-3{'b'} Comment@4-7{"# c"} Literal@8-9{'d'}]`},
	{fl(js, gor), "x", "a #", `Sequence@0-3[Literal@0-1{'a'} Literal@1-2{' '} Literal@2-3{'#'}]`},
	{fl(py, pcre), "x", `a +`, `Sequence@0-3[Quantifier@0-3{1..inf greedy}[Literal@0-1{'a'}]]`},
	{fl(py, pcre), "x", `[ a]\ `, `Sequence@0-6[CharClass@0-4{' ' 'a'} Literal@4-6{' '}]`},
	{fl(pcre), "", `(?x)a b`, `Sequence@0-7[InlineFlags@0-4{flags=x} Literal@4-5{'a'} Literal@6-7{'b'}]`},
	{fl(pcre), "", `(?x:a b)c d`, `Sequence@0-11[InlineFlags@0-8{flags=x}[Literal@4-5{'a'} Literal@6-7{'b'}] Literal@8-9{'c'} Literal@9-10{' '} Literal@10-11{'d'}]`},
	{fl(pcre), "", `(?xx)[ a]`, `Sequence@0-9[InlineFlags@0-5{flags=xx} CharClass@5-9{'a'}]`},
	{fl(py, pcre), "", `a(?#c)*`, `Sequence@0-7[Quantifier@0-7{0..inf greedy}[Literal@0-1{'a'} Comment@1-6{"(?#c)"}]]`},
	{fl(js, gor), "", `(?#c)`, `Sequence@0-5[Unknown@0-5{"(?#c)"}]`},

	// Conditionals.
	{fl(py, pcre), "", `(a)?(?(1)b|c)`, `Sequence@0-13[Quantifier@0-4{0..1 greedy}[Capture@0-3{#1}[Literal@1-2{'a'}]] Conditional@4-13[Condition@6-9{#1} Sequence@9-10[Literal@9-10{'b'}] Sequence@11-12[Literal@11-12{'c'}]]]`},
	{fl(pcre), "", `(?(?=a)a|b)`, `Sequence@0-11[Conditional@0-11[Condition@2-7[LookAhead@2-7[Literal@5-6{'a'}]] Sequence@7-8[Literal@7-8{'a'}] Sequence@9-10[Literal@9-10{'b'}]]]`},
	{fl(pcre), "", `(?(<n>)a)(?<n>x)`, `Sequence@0-16[Conditional@0-9[Condition@2-7{#1 name=n} Sequence@7-8[Literal@7-8{'a'}]] Named@9-16{#1 name=n}[Literal@14-15{'x'}]]`},
	{fl(pcre), "", `(?(DEFINE)a)`, `Sequence@0-12[Conditional@0-12[Condition@2-10{name=DEFINE} Sequence@10-11[Literal@10-11{'a'}]]]`},
	{fl(js, gor), "", `(?(1)a)`, `Sequence@0-7[Unknown@0-7{"(?(1)a)"}]`},

	// PCRE verbs, callouts, quoting.
	{fl(pcre), "", `(*FAIL)a`, `Sequence@0-8[Verb@0-7{name=FAIL} Literal@7-8{'a'}]`},
	{fl(pcre), "", `(*MARK:x)(?C1)`, `Sequence@0-14[Verb@0-9{name=MARK:x} Verb@9-14{name=C1}]`},
	{fl(pcre), "", `(*pla:a)`, `Sequence@0-8[LookAhead@0-8[Literal@6-7{'a'}]]`},
	{fl(pcre), "", `\Q.*\E+`, `Sequence@0-7[Literal@2-3{'.'} Quantifier@3-7{1..inf greedy}[Literal@3-4{'*'}]]`},
	{fl(gor), "", `\Q.\E`, `Sequence@0-5[Literal@2-3{'.'}]`},

	// Classes.
	{fl(gor, pcre), "", `[[:alpha:]]`, `Sequence@0-11[CharClass@0-11{[:alpha:]}]`},
	{fl(gor, pcre), "", `[[:^alpha:]x]`, `Sequence@0-13[CharClass@0-13{^[:alpha:] 'x'}]`},
	{fl(py, js), "", `[[:alpha:]]`, `Sequence@0-11[CharClass@0-10{'[' ':' 'a' 'l' 'p' 'h' 'a' ':'} Literal@10-11{']'}]`},
	{fl(js), "v", `[\p{L}--[a-z]]`, `Sequence@0-14[CharClass@0-14{flags=--}[CharClass@1-6{\p{L}} CharClass@8-13{'a'-'z'}]]`},
	{fl(js), "v", `[a[bc]]`, `Sequence@0-7[CharClass@0-7{'a'}[CharClass@2-6{'b' 'c'}]]`},
	{fl(js), "v", `[\q{ab|c}]`, `Sequence@0-10[CharClass@0-10{\q{ab|c}}]`},
	{fl(js), "", `[]`, `Sequence@0-2[CharClass@0-2]`},
	{fl(js), "", `[^]`, `Sequence@0-3[CharClass@0-3{neg}]`},
	{fl(py, pcre, gor), "", `[]a]`, `Sequence@0-4[CharClass@0-4{']' 'a'}]`},
	{fl(js, gor), "", `[\d-z]`, `Sequence@0-6[CharClass@0-6{\d '-' 'z'}]`},
	{all, "", `[a-]`, `Sequence@0-4[CharClass@0-4{'a' '-'}]`},
	{fl(py, js, pcre), "", `[\b]`, `Sequence@0-4[CharClass@0-4{'\b'}]`},
	{fl(gor), "", `[\b]`, `Sequence@0-4[CharClass@0-4[Unknown@1-3{"\\b"}]]`},
	{fl(pcre, gor), "", `[\pL\P{N}]`, `Sequence@0-10[CharClass@0-10{\p{L} \P{N}}]`},

	// Anchors and escapes.
	{fl(py), "", `\A\z\Z`, `Sequence@0-6[Anchor@0-2{"\\A"} Anchor@2-4{"\\z"} Anchor@4-6{"\\Z"}]`},
	{fl(gor), "", `\A\z\Z`, `Sequence@0-6[Anchor@0-2{"\\A"} Anchor@2-4{"\\z"} Unknown@4-6{"\\Z"}]`},
	{fl(js), "", `\A\z\b`, `Sequence@0-6[Literal@0-2{'A'} Literal@2-4{'z'} Anchor@4-6{"\\b"}]`},
	{fl(pcre), "", `\G\Z`, `Sequence@0-4[Anchor@0-2{"\\G"} Anchor@2-4{"\\Z"}]`},
	{fl(py), "", `\p{L}`, `Sequence@0-5[Unknown@0-2{"\\p"} Literal@2-3{'{'} Literal@3-4{'L'} Literal@4-5{'}'}]`},
	{fl(gor), "", `\pL\P{^Greek}`, `Sequence@0-13[UnicodeProp@0-3{name=L} UnicodeProp@3-13{name=Greek}]`},
	{fl(pcre), "", `\p{^L}`, `Sequence@0-6[UnicodeProp@0-6{name=L neg}]`},
	{fl(pcre), "", `\x{41}\x41\101\cA\e`, `Sequence@0-19[Literal@0-6{'A'} Literal@6-10{'A'} Literal@10-14{'A'} Literal@14-17{'\x01'} Literal@17-19{'\x1b'}]`},
	{fl(py), "", `\N{DIGIT ONE}`, `Sequence@0-13[Literal@0-13{? name=DIGIT ONE}]`},
	{fl(js), "", `\u{41}`, `Sequence@0-6[Quantifier@0-6{41..41 greedy}[Literal@0-2{'u'}]]`},
	{fl(js), "u", `\u{41}\u0042`, `Sequence@0-12[Literal@0-6{'A'} Literal@6-12{'B'}]`},
	{fl(pcre), "", `\h\R\N\X\V`, `Sequence@0-10[Shorthand@0-2{name=h} Shorthand@2-4{name=R} Shorthand@4-6{name=N} Shorthand@6-8{name=X} Shorthand@8-10{name=v neg}]`},
	{fl(py, js, gor), "", `\v\D`, `Sequence@0-4[Literal@0-2{'\v'} Shorthand@2-4{name=d neg}]`},
	// \e is PCRE's escape character; Python and Go reject it.
	{fl(pcre), "", `\e\_`, `Sequence@0-4[Literal@0-2{'\x1b'} Literal@2-4{'_'}]`},
	{fl(py, gor), "", `\e\_`, `Sequence@0-4[Unknown@0-2{"\\e"} Literal@2-4{'_'}]`},
}

func TestParse(t *testing.T) {
	for _, r := range rows {
		for _, f := range r.flavors {
			t.Run(fmt.Sprintf("%s/%s/%s", f, r.flags, r.pattern), func(t *testing.T) {
				got := dump(Parse(r.pattern, DialectFor(f, r.flags)))
				if got != r.want {
					t.Errorf("Parse(%q)\n got %s\nwant %s", r.pattern, got, r.want)
				}
			})
		}
	}
}

func TestDialectFacts(t *testing.T) {
	type facts struct{ lookbehind, atomic, possessive, recursion, keepOut, backrefs bool }
	want := map[engine.Flavor]facts{
		py:   {true, true, true, false, false, true},
		js:   {true, false, false, false, false, true},
		pcre: {true, true, true, true, true, true},
		gor:  {false, false, false, false, false, false},
	}
	for f, w := range want {
		d := DialectFor(f, "")
		got := facts{d.Lookbehind, d.Atomic, d.Possessive, d.Recursion, d.KeepOut, d.Backrefs}
		if got != w {
			t.Errorf("%s: got %+v, want %+v", f, got, w)
		}
	}
	if !DialectFor(js, "u").UnicodeProps || DialectFor(js, "").UnicodeProps {
		t.Error("JS \\p must need u/v")
	}
	if DialectFor(js, "x").Extended || DialectFor(gor, "x").Extended || !DialectFor(py, "x").Extended {
		t.Error("only python/pcre have x")
	}
}

// checkInvariants verifies Raw == pattern[span] and that children nest inside
// their parent in order.
func checkInvariants(t *testing.T, pattern string, n *Node) {
	t.Helper()
	if n.Span[0] < 0 || n.Span[1] > len(pattern) || n.Span[0] > n.Span[1] {
		t.Fatalf("%q: bad span %v on %v", pattern, n.Span, n.Kind)
	}
	if n.Raw != pattern[n.Span[0]:n.Span[1]] {
		t.Fatalf("%q: Raw %q != pattern%v on %v", pattern, n.Raw, n.Span, n.Kind)
	}
	prev := n.Span[0]
	for _, c := range n.Children {
		if c == nil {
			t.Fatalf("%q: nil child in %v", pattern, n.Kind)
		}
		if c.Span[0] < prev || c.Span[1] > n.Span[1] {
			t.Fatalf("%q: child %v%v outside/out of order in %v%v", pattern, c.Kind, c.Span, n.Kind, n.Span)
		}
		prev = c.Span[1]
		checkInvariants(t, pattern, c)
	}
}

func TestParseInvariants(t *testing.T) {
	for _, r := range rows {
		for _, f := range all {
			for _, flags := range []string{"", r.flags, "xnUuv"} {
				checkInvariants(t, r.pattern, Parse(r.pattern, DialectFor(f, flags)))
			}
		}
	}
}

func FuzzParse(f *testing.F) {
	for _, r := range rows {
		f.Add(r.pattern, "")
	}
	f.Add(`(?|(?<a>x)|(?'b'y))\k<a>(?(R&a)z)(*sr:q)[[:<:]]`, "xn")
	f.Add("(?x)[a-\\Q]\\E#\n]{1,", "v")
	f.Add(`[\q{a|[--&&]]]\cZ\c`, "v")
	f.Fuzz(func(t *testing.T, pattern, flags string) {
		for _, fl := range all {
			checkInvariants(t, pattern, Parse(pattern, DialectFor(fl, flags)))
		}
	})
}
