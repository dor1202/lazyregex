// Package explain parses a regex pattern into a flavor-aware syntax tree and
// turns it into plain-English rows. The parser never fails: whatever a
// flavor does not support becomes an Unknown node with its span, because the
// real engine, not the explainer, decides whether a pattern is valid.
package explain

// Kind is the type of a syntax tree node.
type Kind int

// Node kinds. The group kinds (Capture through BranchReset) all describe a
// parenthesised construct; IsGroup reports them.
const (
	Unknown       Kind = iota // a piece the flavor does not support, or malformed syntax
	Alternation               // a|b; Children are the branches, each a Sequence
	Sequence                  // items matched one after another
	Literal                   // one character; Index is its code point (-1 if unknown)
	Dot                       // .
	CharClass                 // [...]; see Node for the fields used
	Shorthand                 // \d \w \s \h \v \R \N \X \C; Name is the lower-case letter for d/w/s/h/v
	Anchor                    // ^ $ \b \B \A \z \Z \G
	Capture                   // (...); Index is the group number
	Named                     // named capture group; Name, Index
	NonCapture                // (?:...), or (...) when auto-capture is off
	Atomic                    // (?>...)
	LookAhead                 // (?=...)
	NegLookAhead              // (?!...)
	LookBehind                // (?<=...)
	NegLookBehind             // (?<!...)
	InlineFlags               // (?i) or (?i:...); Flags holds the flag text
	BranchReset               // (?|...)
	Quantifier                // Children[0] is the repeated item; Min, Max, Greedy
	Backref                   // \1, \g{-1}; Index is the (absolute) group number
	NamedBackref              // \k<n>, (?P=n); Name, and Index when the name is known
	UnicodeProp               // \p{L}; Name is the property text, Negated for \P or ^
	Recursion                 // (?R) (?1) (?&n) \g<n>; Index (0 = whole pattern) or Name
	KeepOut                   // \K
	Comment                   // (?#...), or #... in extended mode
	Conditional               // (?(cond)yes|no); Children: Condition, then each branch as a Sequence
	Condition                 // the (cond) of a Conditional; Index, Name, or one lookaround child
	Verb                      // PCRE (*VERB), (*VERB:arg), start options and callouts; Name is the text inside
)

var kindNames = [...]string{
	Unknown: "Unknown", Alternation: "Alternation", Sequence: "Sequence", Literal: "Literal",
	Dot: "Dot", CharClass: "CharClass", Shorthand: "Shorthand", Anchor: "Anchor",
	Capture: "Capture", Named: "Named", NonCapture: "NonCapture", Atomic: "Atomic",
	LookAhead: "LookAhead", NegLookAhead: "NegLookAhead", LookBehind: "LookBehind",
	NegLookBehind: "NegLookBehind", InlineFlags: "InlineFlags", BranchReset: "BranchReset",
	Quantifier: "Quantifier", Backref: "Backref", NamedBackref: "NamedBackref",
	UnicodeProp: "UnicodeProp", Recursion: "Recursion", KeepOut: "KeepOut", Comment: "Comment",
	Conditional: "Conditional", Condition: "Condition", Verb: "Verb",
}

func (k Kind) String() string {
	if k >= 0 && int(k) < len(kindNames) {
		return kindNames[k]
	}
	return "Kind(?)"
}

// IsGroup reports whether k is one of the parenthesised group kinds.
func (k Kind) IsGroup() bool { return k >= Capture && k <= BranchReset }

// Node is one element of the syntax tree. Raw is always
// pattern[Span[0]:Span[1]], and children lie inside their parent's span in
// pattern order.
//
// Field use by kind:
//   - Literal: Index is the code point; -1 for Python \N{NAME}, whose Name is the character name.
//   - Shorthand: Name is the letter in lower case for d w s h v (Negated for the upper-case
//     form) and as written for R N X C.
//   - Capture, Named: Index is the group number; Named also has Name.
//   - InlineFlags: Flags is the text between "(?" and ")" or ":", e.g. "i-m" or "^x".
//     Without children it is a bare (?i) that applies to the rest of the enclosing group.
//   - Group kinds: Children are the items of the body, or a single Alternation.
//   - CharClass: Negated, ClassItems for the plain members. Children hold Unknown nodes for
//     members the flavor rejects and, in JS v mode, nested classes. When Flags is "--" or
//     "&&", the class is a v-mode set operation and Children are its operands in order.
//   - Quantifier: Min, Max (-1 = unbounded), Greedy. Children[0] is the repeated item; any
//     further children are Comment nodes that sat between it and the quantifier.
//   - Unknown: may have children when the unsupported piece is a group with a parseable body.
type Node struct {
	Kind     Kind
	Span     [2]int // byte offsets in the pattern
	Raw      string
	Children []*Node

	Name       string      // group/backref name, property name, verb text
	Index      int         // capture index / backref number / literal code point
	Min, Max   int         // quantifier; Max -1 = unbounded
	Greedy     string      // "greedy" | "lazy" | "possessive"
	Negated    bool        // classes, \D, \P
	Flags      string      // inline flags on/off, e.g. "i-m"; CharClass set operator
	ClassItems []ClassItem // ranges/literals/shorthands inside [...]
}

// ClassItem is one member of a character class. Exactly one form is used:
//   - a character or range: Lo..Hi (Lo == Hi for a single character)
//   - a shorthand: Shorthand is "d", "w", "s", "h" or "v" (Negated for \D etc.), or a POSIX
//     class written "[:alpha:]" (Negated for [:^alpha:])
//   - a Unicode property: Prop is the property text, Negated for \P or ^
//   - a JS v-mode string set \q{abc|d}: Strings holds the alternatives
type ClassItem struct {
	Lo, Hi    rune
	Shorthand string
	Prop      string
	Negated   bool
	Strings   []string
}
