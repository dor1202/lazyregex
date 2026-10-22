package explain

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/dor1202/lazyregex/internal/engine"
)

// Row is one line of the explanation tree.
type Row struct {
	Depth   int    // tree depth for ├─ └─ drawing
	Token   string // display form, groups collapsed as "(?P<date>…)"
	Text    string // plain English
	Span    [2]int // pattern byte span (for underline / click-to-select)
	Group   int    // capture index this row belongs to (0 = none), for colouring
	Unknown bool   // "can't explain this part"
}

const (
	cantExplain = "can't explain this part"
	maxToken    = 40
)

// Explain parses pattern for flavor f with the given flag letters and returns
// its explanation rows in pattern order.
func Explain(pattern string, f engine.Flavor, flags string) []Row {
	d := DialectFor(f, flags)
	e := &explainer{d: d, fl: initialFlags(d, flags)}
	e.items(rootItems(Parse(pattern, d)), 0)
	return e.rows
}

func rootItems(root *Node) []*Node {
	if root.Kind == Sequence {
		return root.Children
	}
	return []*Node{root}
}

// activeFlags is the part of the active flags that changes the wording.
type activeFlags struct {
	i, m, s       bool // case-insensitive, multiline, dot matches newline
	unicode       bool // \w \b and friends are Unicode aware
	dollarEndOnly bool // PCRE D: $ never matches before a final newline
}

func initialFlags(d Dialect, flags string) activeFlags {
	has := func(c string) bool { return strings.Contains(flags, c) }
	fl := activeFlags{i: has("i"), m: has("m"), s: has("s")}
	switch d.Flavor {
	case engine.Python:
		fl.unicode = !has("a")
	case engine.PCRE:
		fl.unicode = has("u")
		fl.dollarEndOnly = has("D")
	}
	return fl
}

func (fl *activeFlags) apply(flags string) {
	on, off, _ := strings.Cut(flags, "-")
	if strings.HasPrefix(on, "^") {
		on = on[1:]
		fl.i, fl.m, fl.s = false, false, false
	}
	set := func(letters string, v bool) {
		for _, c := range letters {
			switch c {
			case 'i':
				fl.i = v
			case 'm':
				fl.m = v
			case 's':
				fl.s = v
			case 'a':
				fl.unicode = !v
			case 'u':
				if v {
					fl.unicode = true
				}
			case 'D':
				fl.dollarEndOnly = v
			}
		}
	}
	set(on, true)
	set(off, false)
}

type explainer struct {
	d     Dialect
	fl    activeFlags
	group int
	rows  []Row
}

func (e *explainer) add(depth int, token, text string, span [2]int, unknown bool) {
	e.rows = append(e.rows, Row{Depth: depth, Token: truncate(token), Text: text, Span: span, Group: e.group, Unknown: unknown})
}

func truncate(s string) string {
	if utf8.RuneCountInString(s) <= maxToken {
		return s
	}
	r := []rune(s)
	return string(r[:maxToken-1]) + "…"
}

// items emits rows for a list of sibling nodes, merging runs of plain
// literal characters into one row.
func (e *explainer) items(list []*Node, depth int) {
	for i := 0; i < len(list); i++ {
		n := list[i]
		if mergeable(n) {
			j := i + 1
			for j < len(list) && mergeable(list[j]) {
				j++
			}
			e.literalRun(list[i:j], depth)
			i = j - 1
			continue
		}
		e.item(n, depth)
	}
}

func mergeable(n *Node) bool {
	return n.Kind == Literal && n.Index >= 0 && unicode.IsPrint(rune(n.Index))
}

func (e *explainer) literalRun(run []*Node, depth int) {
	var b strings.Builder
	for _, n := range run {
		b.WriteRune(rune(n.Index))
	}
	span := [2]int{run[0].Span[0], run[len(run)-1].Span[1]}
	token := ""
	for _, n := range run {
		token += n.Raw
	}
	e.add(depth, token, e.literalText(b.String()), span, false)
}

func (e *explainer) literalText(s string) string {
	t := `"` + s + `" literally`
	if e.fl.i && hasCased(s) {
		t += " (case-insensitive)"
	}
	return t
}

func hasCased(s string) bool {
	for _, r := range s {
		if unicode.ToUpper(r) != unicode.ToLower(r) {
			return true
		}
	}
	return false
}

func (e *explainer) item(n *Node, depth int) {
	switch n.Kind {
	case Sequence:
		e.items(n.Children, depth)
	case Alternation:
		e.alternation(n, depth)
	case Quantifier:
		if len(n.Children) == 0 {
			e.add(depth, n.Raw, cantExplain, n.Span, true)
			return
		}
		e.node(n.Children[0], n, depth)
	default:
		e.node(n, nil, depth)
	}
}

// node emits the row for core, optionally repeated by quantifier q, and the
// rows of its body.
func (e *explainer) node(core, q *Node, depth int) {
	text, unknown := e.describe(core)
	token, span := e.tokenFor(core), core.Span
	if q != nil {
		text += ", " + quantText(q)
		token += quantSuffix(core, q)
		span = q.Span
	}
	savedGroup, savedFl := e.group, e.fl
	if core.Kind == Capture || core.Kind == Named {
		e.group = core.Index
	}
	e.add(depth, token, text, span, unknown)
	// The row itself is coloured by the group it opens; flags set by a
	// scoped (?i:...) apply to its body only.
	if core.Kind == InlineFlags && len(core.Children) > 0 {
		e.fl.apply(core.Flags)
	}
	switch {
	case core.Kind == Conditional:
		e.conditional(core, depth+1)
	case core.Kind == CharClass && len(core.Children) > 0:
		for _, c := range core.Children {
			e.item(c, depth+1)
		}
	case len(core.Children) > 0 && hasBody(core):
		e.items(core.Children, depth+1)
	}
	e.group = savedGroup
	if core.Kind == InlineFlags && len(core.Children) == 0 {
		e.fl.apply(core.Flags) // a bare (?i) lasts to the end of the enclosing group
	} else {
		e.fl = savedFl
	}
}

// hasBody reports whether n's Children are the items of a parenthesised body.
func hasBody(n *Node) bool {
	if n.Kind.IsGroup() {
		return true
	}
	return (n.Kind == Verb || n.Kind == Unknown) && strings.HasPrefix(n.Raw, "(")
}

func (e *explainer) alternation(n *Node, depth int) {
	e.add(depth, n.Raw, fmt.Sprintf("one of %d alternatives", len(n.Children)), n.Span, false)
	for i, b := range n.Children {
		e.branch(b, depth+1, false, fmt.Sprintf("alternative %d", i+1))
	}
}

// branch emits one alternative. A branch that is a single row stands for
// itself (with force, its text is prefixed by the label); a longer one gets a
// labelled row above its items.
func (e *explainer) branch(b *Node, depth int, force bool, label string) {
	first := len(e.rows)
	items := b.Children
	if b.Kind != Sequence {
		items = []*Node{b}
	}
	if len(items) == 0 {
		e.add(depth, "", "empty alternative", b.Span, false)
		return
	}
	if countRows(items) == 1 {
		e.items(items, depth)
		if force && first < len(e.rows) { // keep the branch name on the single row
			e.rows[first].Text = label + ": " + e.rows[first].Text
		}
		return
	}
	e.add(depth, b.Raw, label, b.Span, false)
	e.items(items, depth+1)
}

// countRows is how many top-level rows items would produce.
func countRows(items []*Node) int {
	n := 0
	for i := 0; i < len(items); i++ {
		n++
		if mergeable(items[i]) {
			for i+1 < len(items) && mergeable(items[i+1]) {
				i++
			}
		}
	}
	return n
}

func (e *explainer) conditional(n *Node, depth int) {
	if len(n.Children) == 0 {
		return
	}
	cond := n.Children[0]
	text := e.conditionText(cond)
	e.add(depth, cond.Raw, text, cond.Span, false)
	if len(cond.Children) > 0 {
		e.items(cond.Children, depth+1)
	}
	labels := []string{"then", "otherwise"}
	for i, b := range n.Children[1:] {
		label := "extra branch"
		if i < len(labels) {
			label = labels[i]
		}
		e.branch(b, depth, true, label)
	}
}

func (e *explainer) conditionText(c *Node) string {
	switch {
	case len(c.Children) > 0:
		return "if this assertion matches"
	case c.Index > 0:
		return fmt.Sprintf("if group #%d matched", c.Index)
	case c.Name == "DEFINE":
		return "never matches: only defines groups for later calls"
	case c.Name == "R" || strings.HasPrefix(c.Name, "R&") || (strings.HasPrefix(c.Name, "R") && len(c.Name) > 1 && c.Name[1] >= '0' && c.Name[1] <= '9'):
		return "if inside a recursive call"
	case c.Name != "":
		return fmt.Sprintf("if group %q matched", c.Name)
	}
	return "if the condition holds"
}

// tokenFor is the display form of n: groups are collapsed to their opener.
func (e *explainer) tokenFor(n *Node) string {
	if len(n.Children) > 0 && hasBody(n) {
		return opener(n) + "…)"
	}
	if n.Kind == Conditional && len(n.Children) > 0 {
		return "(?" + n.Children[0].Raw + "…)"
	}
	return n.Raw
}

// opener is the text of group n up to its body, such as "(?P<date>".
func opener(n *Node) string {
	end := n.Children[0].Span[0] - n.Span[0]
	if end < 1 || end > len(n.Raw) {
		return "("
	}
	return strings.TrimRight(n.Raw[:end], " \t\r\n")
}

// quantSuffix is the quantifier text after core in the pattern.
func quantSuffix(core, q *Node) string {
	from := core.Span[1] - q.Span[0]
	if len(q.Children) > 1 {
		from = q.Children[len(q.Children)-1].Span[1] - q.Span[0]
	}
	if from < 0 || from > len(q.Raw) {
		return ""
	}
	return strings.TrimSpace(q.Raw[from:])
}

func quantText(q *Node) string {
	var c string
	switch {
	case q.Min == q.Max && q.Min == 1:
		return "exactly 1 time"
	case q.Min == q.Max:
		return fmt.Sprintf("exactly %d times", q.Min)
	case q.Min == 0 && q.Max == 1:
		c = "0 or 1"
	case q.Max < 0:
		c = fmt.Sprintf("%d or more", q.Min)
	default:
		c = fmt.Sprintf("between %d and %d times", q.Min, q.Max)
	}
	return c + " (" + q.Greedy + ")"
}

// describe returns the English text for n (ignoring any quantifier on it).
func (e *explainer) describe(n *Node) (text string, unknown bool) {
	switch n.Kind {
	case Literal:
		return e.describeLiteral(n), false
	case Dot:
		if e.fl.s {
			return "any character (s flag)", false
		}
		return "any char but newline", false
	case Shorthand:
		return e.shorthand(n), false
	case Anchor:
		return e.anchor(n), false
	case CharClass:
		return e.classText(n), false
	case Capture:
		return fmt.Sprintf("capture group #%d", n.Index), false
	case Named:
		return fmt.Sprintf("named group %q", n.Name), false
	case NonCapture:
		return "non-capturing group", false
	case Atomic:
		return "atomic group (no backtracking into it)", false
	case LookAhead:
		return lookText(n, "lookahead: what follows must match"), false
	case NegLookAhead:
		return lookText(n, "negative lookahead: what follows must not match"), false
	case LookBehind:
		return lookText(n, "lookbehind: what precedes must match"), false
	case NegLookBehind:
		return lookText(n, "negative lookbehind: what precedes must not match"), false
	case InlineFlags:
		return e.flagsText(n), false
	case BranchReset:
		return "branch reset group (each alternative numbers its groups from the same start)", false
	case Backref:
		return fmt.Sprintf("same text as group #%d", n.Index), false
	case NamedBackref:
		if n.Index > 0 {
			return fmt.Sprintf("same text as group %q (#%d)", n.Name, n.Index), false
		}
		return fmt.Sprintf("same text as group %q", n.Name), false
	case UnicodeProp:
		return propText(n.Name, n.Negated), false
	case Recursion:
		switch {
		case n.Name != "" && n.Index > 0:
			return fmt.Sprintf("recurse into group %q (#%d)", n.Name, n.Index), false
		case n.Name != "":
			return fmt.Sprintf("recurse into group %q", n.Name), false
		case n.Index == 0:
			return "recurse into the whole pattern", false
		}
		return fmt.Sprintf("recurse into group #%d", n.Index), false
	case KeepOut:
		return "reset the match start here (drop what matched so far)", false
	case Comment:
		return "comment (ignored)", false
	case Conditional:
		return "conditional: pick a branch depending on the condition", false
	case Condition:
		return e.conditionText(n), false
	case Verb:
		return verbText(n), false
	case Sequence:
		return "sequence", false
	case Alternation:
		return fmt.Sprintf("one of %d alternatives", len(n.Children)), false
	}
	return cantExplain, true
}

func lookText(n *Node, base string) string {
	if n.Name == "non-atomic" {
		return "non-atomic " + base
	}
	return base
}

func (e *explainer) describeLiteral(n *Node) string {
	if n.Index < 0 {
		return fmt.Sprintf("the character named %q", n.Name)
	}
	r := rune(n.Index)
	if mergeable(n) {
		return e.literalText(string(r))
	}
	return runeName(r) + " literally"
}

// runeName names a character for display: itself when printable.
func runeName(r rune) string {
	switch r {
	case '\n':
		return "newline"
	case '\t':
		return "tab"
	case '\r':
		return "carriage return"
	case '\f':
		return "form feed"
	case '\v':
		return "vertical tab"
	case 7:
		return "bell"
	case 0x1b:
		return "escape"
	case 0:
		return "null character"
	case ' ':
		return "space"
	}
	if unicode.IsPrint(r) {
		return string(r)
	}
	return fmt.Sprintf("U+%04X", r)
}

func (e *explainer) wordKind() string {
	if e.d.Flavor == engine.Python || e.d.Flavor == engine.PCRE {
		if e.fl.unicode {
			return " (Unicode)"
		}
	}
	return " (ASCII)"
}

func (e *explainer) shorthand(n *Node) string {
	neg := n.Negated
	switch n.Name {
	case "d":
		return pick(neg, "a digit", "any char but a digit")
	case "w":
		return pick(neg, "a word character", "any char but a word character") + e.wordKind()
	case "s":
		return pick(neg, "whitespace", "any char but whitespace")
	case "h":
		return pick(neg, "horizontal whitespace", "any char but horizontal whitespace")
	case "v":
		return pick(neg, "vertical whitespace", "any char but vertical whitespace")
	case "R":
		return "a line break (any newline sequence)"
	case "N":
		return "any char but newline"
	case "X":
		return "an extended grapheme cluster"
	case "C":
		return "a single code unit"
	}
	return cantExplain
}

func pick(neg bool, pos, negText string) string {
	if neg {
		return negText
	}
	return pos
}

func (e *explainer) anchor(n *Node) string {
	f := e.d.Flavor
	switch n.Raw {
	case "^":
		if e.fl.m {
			return "start of a line (m flag)"
		}
		return "start of the string"
	case "$":
		switch {
		case e.fl.m:
			return "end of a line (m flag)"
		case f == engine.Python || (f == engine.PCRE && !e.fl.dollarEndOnly):
			return "end of the string (or before a final newline)"
		}
		return "end of the string"
	case `\A`:
		return "start of the string"
	case `\z`:
		return "end of the string"
	case `\Z`:
		if f == engine.PCRE {
			return "end of the string (or before a final newline)"
		}
		return "end of the string"
	case `\G`:
		return "where the previous match ended"
	case `\b`:
		return "a word boundary" + e.wordKind()
	case `\B`:
		return "not a word boundary" + e.wordKind()
	}
	return cantExplain
}

func (e *explainer) classText(n *Node) string {
	switch n.Flags {
	case "--":
		return "characters in the first set but not in the others"
	case "&&":
		return "characters in every one of these sets"
	}
	var parts []string
	for _, it := range n.ClassItems {
		parts = append(parts, itemText(it))
	}
	if len(n.Children) > 0 {
		parts = append(parts, "the sets below")
	}
	lead := "any of: "
	if n.Negated {
		lead = "any char except: "
	}
	if len(parts) == 0 {
		if n.Negated {
			return "any character"
		}
		return "nothing (empty class never matches)"
	}
	return lead + strings.Join(parts, ", ")
}

func itemText(it ClassItem) string {
	switch {
	case it.Strings != nil:
		q := make([]string, len(it.Strings))
		for i, s := range it.Strings {
			q[i] = `"` + s + `"`
		}
		return "string " + strings.Join(q, " or ")
	case it.Prop != "":
		return strings.TrimPrefix(propText(it.Prop, it.Negated), "any char but ")
	case strings.HasPrefix(it.Shorthand, "[:"):
		name := strings.Trim(it.Shorthand, "[:]")
		name = strings.TrimPrefix(name, "^")
		return pick(it.Negated, name, "not "+name)
	case it.Shorthand != "":
		words := map[string]string{"d": "digit", "w": "word character", "s": "whitespace", "h": "horizontal whitespace", "v": "vertical whitespace"}
		w := words[it.Shorthand]
		return pick(it.Negated, w, "non-"+w)
	case it.Lo == it.Hi:
		return runeName(it.Lo)
	}
	return runeName(it.Lo) + "-" + runeName(it.Hi)
}

var propNames = map[string]string{
	"L": "letter", "Lu": "uppercase letter", "Ll": "lowercase letter", "Lt": "titlecase letter",
	"Lm": "modifier letter", "Lo": "other letter", "M": "mark", "Mn": "nonspacing mark",
	"Mc": "spacing mark", "Me": "enclosing mark", "N": "number", "Nd": "decimal digit",
	"Nl": "letter number", "No": "other number", "P": "punctuation", "S": "symbol",
	"Z": "separator", "Zs": "space separator", "C": "other character", "Cc": "control character",
}

func propText(name string, neg bool) string {
	short := name
	for _, p := range []string{"General_Category=", "gc=", "Category="} {
		short = strings.TrimPrefix(short, p)
	}
	if w, ok := propNames[short]; ok {
		return pick(neg, "a "+w+" (Unicode "+short+")", "any char but a "+w+" (Unicode "+short+")")
	}
	return pick(neg, fmt.Sprintf("a character with Unicode property %q", name),
		fmt.Sprintf("any char but one with Unicode property %q", name))
}

var flagWords = map[rune]string{
	'i': "case-insensitive", 'm': "multiline", 's': "dot matches newline", 'x': "extended (whitespace and #comments ignored)",
	'a': "ASCII-only matching", 'u': "Unicode matching", 'L': "locale-dependent matching",
	'n': "no auto-capture", 'U': "ungreedy quantifiers", 'J': "duplicate group names allowed",
	'D': "$ only at the very end", 'S': "study", 'W': "ASCII-only classes", 'P': "POSIX classes ASCII-only", 'T': "ASCII-only digits", 'r': "restricted caseless matching",
}

func (e *explainer) flagsText(n *Node) string {
	on, off, _ := strings.Cut(n.Flags, "-")
	reset := strings.HasPrefix(on, "^")
	on = strings.TrimPrefix(on, "^")
	words := func(s string) string {
		var w []string
		for _, c := range s {
			if t, ok := flagWords[c]; ok {
				w = append(w, t)
			}
		}
		return strings.Join(w, ", ")
	}
	var parts []string
	if reset {
		parts = append(parts, "resets flags")
	}
	if on != "" {
		parts = append(parts, "turns on "+words(on))
	}
	if off != "" {
		parts = append(parts, "turns off "+words(off))
	}
	t := strings.Join(parts, "; ")
	if t == "" {
		t = "sets flags"
	}
	if len(n.Children) > 0 {
		return "non-capturing group that " + t
	}
	return "flags: " + t + " from here to the end of the group"
}

func verbText(n *Node) string {
	name := n.Name
	if strings.HasPrefix(n.Raw, "(?C") {
		return "callout (calls back into the host program)"
	}
	base, _, _ := strings.Cut(name, ":")
	switch base {
	case "ACCEPT":
		return "end the match successfully here"
	case "FAIL", "F":
		return "force a failure here"
	case "COMMIT":
		return "if the rest fails, give up the whole match (no further start positions)"
	case "PRUNE":
		return "if the rest fails, give up at this start position"
	case "SKIP":
		return "if the rest fails, skip ahead to this position for the next try"
	case "THEN":
		return "if the rest fails, jump to the next alternative"
	case "MARK", "":
		return "name this position for backtracking verbs"
	}
	if len(n.Children) > 0 {
		return fmt.Sprintf("PCRE group (*%s:…)", base)
	}
	return fmt.Sprintf("PCRE option or verb (*%s)", base)
}
