package explain

import (
	"strconv"
	"strings"
)

// parseGroup parses a construct starting with "(".
func (p *parser) parseGroup() *Node {
	s := p.pos
	rest := p.src[s:]
	if p.d.Verbs && strings.HasPrefix(rest, "(*") {
		return p.parseVerb()
	}
	if !strings.HasPrefix(rest, "(?") {
		p.pos++
		if p.st.noAuto {
			return p.groupBody(p.mk(NonCapture, s, s), false)
		}
		p.groups++
		n := p.mk(Capture, s, s)
		n.Index = p.groups
		return p.groupBody(n, false)
	}
	has := func(prefix string) bool { return strings.HasPrefix(rest, prefix) }
	switch {
	case has("(?#"):
		e := strings.IndexByte(rest, ')')
		p.pos = len(p.src)
		if e >= 0 {
			p.pos = s + e + 1
		}
		if p.d.InlineComments {
			return p.mk(Comment, s, p.pos)
		}
		return p.unknown(s, p.pos)
	case has("(?:"):
		return p.open(NonCapture, 3, true)
	case has("(?="):
		return p.open(LookAhead, 3, p.d.Lookahead)
	case has("(?!"):
		return p.open(NegLookAhead, 3, p.d.Lookahead)
	case has("(?<="):
		return p.open(LookBehind, 4, p.d.Lookbehind)
	case has("(?<!"):
		return p.open(NegLookBehind, 4, p.d.Lookbehind)
	case has("(?*") && p.d.Verbs:
		n := p.open(LookAhead, 3, true)
		n.Name = "non-atomic"
		return n
	case has("(?<*") && p.d.Verbs:
		n := p.open(LookBehind, 4, true)
		n.Name = "non-atomic"
		return n
	case has("(?>"):
		return p.open(Atomic, 3, p.d.Atomic)
	case has("(?|"):
		if !p.d.BranchReset {
			return p.open(Unknown, 3, false)
		}
		p.pos += 3
		return p.groupBody(p.mk(BranchReset, s, s), true)
	case has("(?P<"):
		return p.namedGroup("(?P<", '>')
	case has("(?<"):
		return p.namedGroup("(?<", '>')
	case has("(?'"):
		return p.namedGroup("(?'", '\'')
	case has("(?P="):
		return p.nameRef(NamedBackref, 4, p.d.acceptsNamedBackref("(?P="))
	case has("(?P>"):
		return p.nameRef(Recursion, 4, p.d.Recursion)
	case has("(?&"):
		return p.nameRef(Recursion, 3, p.d.Recursion)
	case has("(?("):
		return p.parseConditional()
	case has("(?C") && p.d.Verbs:
		return p.toParen(Verb, 2)
	}
	if n := p.numberedRecursion(); n != nil {
		return n
	}
	return p.flagGroup()
}

// open starts a group whose opener is n bytes long; an unsupported group
// still has its body parsed, under an Unknown node.
func (p *parser) open(k Kind, n int, ok bool) *Node {
	s := p.pos
	p.pos += n
	if !ok {
		k = Unknown
	}
	return p.groupBody(p.mk(k, s, s), false)
}

// groupBody parses the body of group n up to and including ")". Inline
// flags set inside the body end with it.
func (p *parser) groupBody(n *Node, reset bool) *Node {
	saved := p.st
	branches := p.parseAlt(false, reset)
	p.st = saved
	if len(branches) == 1 {
		n.Children = branches[0].Children
	} else {
		n.Children = []*Node{p.altNode(branches)}
	}
	if p.pos < len(p.src) && p.src[p.pos] == ')' {
		p.pos++
	}
	return p.finish(n)
}

// skipGroup returns the end of the parenthesised construct starting at s,
// honouring escapes and classes; it is used for constructs that become
// Unknown without a parsed body.
func (p *parser) skipGroup(s int) int {
	depth, inClass := 0, false
	for i := s; i < len(p.src); i++ {
		switch p.src[i] {
		case '\\':
			i++
		case '[':
			inClass = true
		case ']':
			inClass = false
		case '(':
			if !inClass {
				depth++
			}
		case ')':
			if !inClass {
				if depth--; depth == 0 {
					return i + 1
				}
			}
		}
	}
	return len(p.src)
}

func (p *parser) unknownGroup() *Node {
	s := p.pos
	p.pos = p.skipGroup(s)
	return p.unknown(s, p.pos)
}

// toParen makes a node of kind k from the current "(" to the next ")",
// with Name set to the text after the first skip bytes.
func (p *parser) toParen(k Kind, skip int) *Node {
	s := p.pos
	e := strings.IndexByte(p.src[s:], ')')
	if e < 0 {
		p.pos = len(p.src)
		return p.unknown(s, p.pos)
	}
	p.pos = s + e + 1
	n := p.mk(k, s, p.pos)
	n.Name = p.src[s+skip : s+e]
	return n
}

func (p *parser) namedGroup(opener string, closer byte) *Node {
	s := p.pos
	e := strings.IndexByte(p.src[s+len(opener):], closer)
	if e < 0 {
		return p.unknownGroup()
	}
	name := p.src[s+len(opener) : s+len(opener)+e]
	p.pos = s + len(opener) + e + 1
	if !p.d.acceptsNamedGroup(opener) {
		return p.groupBody(p.mk(Unknown, s, s), false)
	}
	p.groups++
	n := p.mk(Named, s, s)
	n.Name, n.Index = name, p.groups
	if _, seen := p.names[name]; !seen {
		p.names[name] = p.groups
	}
	return p.groupBody(n, false)
}

// nameRef parses "(?P=name)"-style references whose opener is n bytes.
func (p *parser) nameRef(k Kind, n int, ok bool) *Node {
	if !ok {
		return p.toParen(Unknown, 0)
	}
	node := p.toParen(k, n)
	if node.Kind == Unknown {
		node.Name = ""
	}
	return node
}

// numberedRecursion parses (?R), (?1), (?+1) and (?-1).
func (p *parser) numberedRecursion() *Node {
	s := p.pos
	e := strings.IndexByte(p.src[s:], ')')
	if e < 0 {
		return nil
	}
	inner := p.src[s+2 : s+e]
	idx, ok := 0, inner == "R"
	if !ok {
		idx, ok = p.groupNumber(inner)
	}
	if !ok {
		return nil
	}
	p.pos = s + e + 1
	if !p.d.Recursion {
		return p.unknown(s, p.pos)
	}
	n := p.mk(Recursion, s, p.pos)
	n.Index = idx
	return n
}

// groupNumber reads "3", "+1" or "-1" and resolves relative numbers against
// the groups opened so far: -1 is the most recently opened, +1 the next.
func (p *parser) groupNumber(s string) (int, bool) {
	sign := byte(0)
	if s != "" && (s[0] == '+' || s[0] == '-') {
		sign, s = s[0], s[1:]
	}
	if s == "" || strings.Trim(s, "0123456789") != "" {
		return 0, false
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	switch sign {
	case '+':
		return p.groups + v, true
	case '-':
		return p.groups - v + 1, true
	}
	return v, true
}

// flagGroup parses (?flags), (?flags:...) and their "-" and "^" forms.
func (p *parser) flagGroup() *Node {
	s := p.pos
	i := s + 2
	for i < len(p.src) && (p.src[i] == '-' || strings.IndexByte(p.d.FlagLetters, p.src[i]) >= 0) {
		i++
	}
	if i >= len(p.src) || (p.src[i] != ')' && p.src[i] != ':') {
		return p.unknownGroup()
	}
	flags := p.src[s+2 : i]
	if p.src[i] == ')' {
		p.pos = i + 1
		if !p.d.BareFlags {
			return p.unknown(s, p.pos)
		}
		p.applyFlags(flags)
		n := p.mk(InlineFlags, s, p.pos)
		n.Flags = flags
		return n
	}
	p.pos = i + 1
	outer := p.st
	p.applyFlags(flags)
	n := p.mk(InlineFlags, s, s)
	n.Flags = flags
	p.groupBody(n, false)
	p.st = outer
	return n
}

func (p *parser) applyFlags(flags string) {
	on, off, _ := strings.Cut(flags, "-")
	if strings.HasPrefix(on, "^") {
		on = on[1:]
		p.st.extended, p.st.extendedClass, p.st.noAuto = false, false, false
	}
	if strings.Contains(on, "x") {
		p.st.extended = true
		p.st.extendedClass = strings.Count(on, "x") >= 2
	}
	if strings.Contains(on, "n") {
		p.st.noAuto = true
	}
	if strings.Contains(on, "U") {
		p.st.ungreedy = true
	}
	if strings.Contains(off, "x") {
		p.st.extended, p.st.extendedClass = false, false
	}
	if strings.Contains(off, "n") {
		p.st.noAuto = false
	}
	if strings.Contains(off, "U") {
		p.st.ungreedy = false
	}
}

// parseConditional parses (?(cond)yes|no).
func (p *parser) parseConditional() *Node {
	s := p.pos
	if !p.d.Conditionals {
		return p.unknownGroup()
	}
	cs := s + 2 // the "(" of the condition
	var cond *Node
	rest := p.src[cs:]
	if strings.HasPrefix(rest, "(?=") || strings.HasPrefix(rest, "(?!") ||
		strings.HasPrefix(rest, "(?<=") || strings.HasPrefix(rest, "(?<!") {
		p.pos = cs
		a := p.parseGroup()
		cond = p.mk(Condition, a.Span[0], a.Span[1])
		cond.Children = []*Node{a}
	} else {
		e := strings.IndexByte(rest, ')')
		if e < 0 {
			return p.unknownGroup()
		}
		cond = p.mk(Condition, cs, cs+e+1)
		p.setCondition(cond, rest[1:e])
		p.pos = cs + e + 1
	}
	n := p.mk(Conditional, s, s)
	saved := p.st
	branches := p.parseAlt(false, false)
	p.st = saved
	n.Children = append([]*Node{cond}, branches...)
	if p.pos < len(p.src) && p.src[p.pos] == ')' {
		p.pos++
	}
	return p.finish(n)
}

func (p *parser) setCondition(n *Node, text string) {
	if idx, ok := p.groupNumber(text); ok {
		n.Index = idx
		return
	}
	if len(text) >= 2 && ((text[0] == '<' && text[len(text)-1] == '>') ||
		(text[0] == '\'' && text[len(text)-1] == '\'')) {
		text = text[1 : len(text)-1]
	}
	n.Name = text
}

// parseVerb parses PCRE's (*...) constructs: backtracking verbs, start-of-
// pattern options, and alphabetic assertions such as (*pla:...).
func (p *parser) parseVerb() *Node {
	s := p.pos
	i := s + 2
	for i < len(p.src) && (isAlnum(p.src[i]) || p.src[i] == '_') {
		i++
	}
	word := p.src[s+2 : i]
	if i < len(p.src) && p.src[i] == ':' && word != "" && word[0] >= 'a' && word[0] <= 'z' {
		k, name := alphaAssertion(word)
		p.pos = i + 1
		n := p.mk(k, s, s)
		n.Name = name
		return p.groupBody(n, false)
	}
	return p.toParen(Verb, 2)
}

func alphaAssertion(word string) (Kind, string) {
	switch word {
	case "pla", "positive_lookahead":
		return LookAhead, ""
	case "nla", "negative_lookahead":
		return NegLookAhead, ""
	case "plb", "positive_lookbehind":
		return LookBehind, ""
	case "nlb", "negative_lookbehind":
		return NegLookBehind, ""
	case "napla", "non_atomic_positive_lookahead":
		return LookAhead, "non-atomic"
	case "naplb", "non_atomic_positive_lookbehind":
		return LookBehind, "non-atomic"
	case "atomic":
		return Atomic, ""
	}
	return Verb, word // script runs (*sr:...), (*asr:...) and anything newer
}

func isAlnum(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
