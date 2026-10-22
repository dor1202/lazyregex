package explain

import (
	"strings"
	"unicode/utf8"
)

// flagState is the part of the inline-flag state that changes how the
// pattern is read. It is scoped to the enclosing group.
type flagState struct {
	extended      bool // x
	extendedClass bool // xx: also ignore space and tab inside classes
	noAuto        bool // n
	ungreedy      bool // U
}

type parser struct {
	src      string
	pos      int
	d        Dialect
	st       flagState
	groups   int  // capture groups opened so far
	total    int  // capture groups in the whole pattern; -1 on the counting pass
	hasNamed bool // the pattern has a named group (JS: \k becomes a backreference)
	inQuote  bool // inside \Q...\E
	names    map[string]int
}

// Parse parses pattern with dialect d. It never fails: anything d does not
// support, and any malformed piece, becomes an Unknown node with its span.
// The root is an Alternation when the top level has "|", else a Sequence.
func Parse(pattern string, d Dialect) *Node {
	// A counting pass learns the number of capture groups and whether any
	// are named; JS and PCRE need both to tell \12 and \k apart.
	count := newParser(pattern, d, -1, false)
	count.parseRoot()
	p := newParser(pattern, d, count.groups, len(count.names) > 0)
	root := p.parseRoot()
	p.resolveNames(root)
	return root
}

func newParser(src string, d Dialect, total int, hasNamed bool) *parser {
	return &parser{
		src: src, d: d, total: total, hasNamed: hasNamed,
		st:    flagState{extended: d.Extended, noAuto: d.NoAutoCapture, ungreedy: d.Ungreedy},
		names: map[string]int{},
	}
}

func (p *parser) mk(k Kind, s, e int) *Node {
	return &Node{Kind: k, Span: [2]int{s, e}, Raw: p.src[s:e]}
}

// finish sets n's end to the current position.
func (p *parser) finish(n *Node) *Node {
	n.Span[1] = p.pos
	n.Raw = p.src[n.Span[0]:p.pos]
	return n
}

func (p *parser) unknown(s, e int) *Node { return p.mk(Unknown, s, e) }

func (p *parser) literal(s, e int, r rune) *Node {
	n := p.mk(Literal, s, e)
	n.Index = int(r)
	return n
}

// literalRune reads one UTF-8 character as a Literal.
func (p *parser) literalRune() *Node {
	s := p.pos
	r, w := utf8.DecodeRuneInString(p.src[s:])
	p.pos += w
	return p.literal(s, p.pos, r)
}

func (p *parser) parseRoot() *Node {
	return p.altNode(p.parseAlt(true, false))
}

func (p *parser) altNode(branches []*Node) *Node {
	if len(branches) == 1 {
		return branches[0]
	}
	n := p.mk(Alternation, branches[0].Span[0], branches[len(branches)-1].Span[1])
	n.Children = branches
	return n
}

// parseAlt reads "|"-separated branches up to ")" or the end. With reset
// (a PCRE branch-reset group) every branch numbers its groups from the same
// start.
func (p *parser) parseAlt(top, reset bool) []*Node {
	start, most := p.groups, p.groups
	var branches []*Node
	for {
		if reset {
			p.groups = start
		}
		branches = append(branches, p.parseSeq(top))
		most = max(most, p.groups)
		if p.pos < len(p.src) && p.src[p.pos] == '|' && !p.inQuote {
			p.pos++
			continue
		}
		break
	}
	if reset {
		p.groups = most
	}
	return branches
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\v' || c == '\f'
}

func (p *parser) parseSeq(top bool) *Node {
	seq := p.mk(Sequence, p.pos, p.pos)
	var items []*Node
	for p.pos < len(p.src) {
		if p.inQuote {
			if strings.HasPrefix(p.src[p.pos:], `\E`) {
				p.inQuote = false
				p.pos += 2
				continue
			}
			items = append(items, p.literalRune())
			continue
		}
		c := p.src[p.pos]
		if c == '|' {
			break
		}
		if c == ')' {
			if !top {
				break
			}
			items = append(items, p.unknown(p.pos, p.pos+1))
			p.pos++
			continue
		}
		if p.st.extended {
			if isSpace(c) {
				p.pos++
				continue
			}
			if c == '#' {
				items = append(items, p.lineComment())
				continue
			}
		}
		if q := p.quantifier(); q != nil {
			items = p.applyQuantifier(items, q)
			continue
		}
		if n := p.parseAtom(); n != nil {
			items = append(items, n)
		}
	}
	seq.Children = items
	return p.finish(seq)
}

func (p *parser) lineComment() *Node {
	s := p.pos
	e := strings.IndexByte(p.src[s:], '\n')
	if e < 0 {
		p.pos = len(p.src)
	} else {
		p.pos = s + e
	}
	return p.mk(Comment, s, p.pos)
}

// applyQuantifier attaches q to the last item. Comments between the item and
// q become extra children. With nothing to repeat, or right after another
// quantifier, q becomes Unknown: every flavor rejects "a**" and "{2}" alone.
func (p *parser) applyQuantifier(items []*Node, q *Node) []*Node {
	j := len(items) - 1
	for j >= 0 && items[j].Kind == Comment {
		j--
	}
	if j < 0 || isRepeat(items[j]) {
		q.Kind = Unknown
		return append(items, q)
	}
	q.Children = append([]*Node{items[j]}, items[j+1:]...)
	q.Span[0] = items[j].Span[0]
	q.Raw = p.src[q.Span[0]:q.Span[1]]
	return append(items[:j], q)
}

func isRepeat(n *Node) bool {
	return n.Kind == Quantifier ||
		(n.Kind == Unknown && len(n.Children) == 0 && n.Raw != "" && strings.IndexByte("*+?{", n.Raw[0]) >= 0)
}

// quantifier reads a quantifier token at the current position, or returns
// nil (leaving the position alone) if there is none.
func (p *parser) quantifier() *Node {
	s := p.pos
	lo, hi, end := 0, 0, s+1
	switch p.src[s] {
	case '*':
		lo, hi = 0, -1
	case '+':
		lo, hi = 1, -1
	case '?':
		lo, hi = 0, 1
	case '{':
		var ok bool
		if lo, hi, end, ok = p.braceQuantifier(s); !ok {
			return nil
		}
	default:
		return nil
	}
	greedy := "greedy"
	switch {
	case end < len(p.src) && p.src[end] == '?':
		end++
		if !p.st.ungreedy {
			greedy = "lazy"
		}
	case end < len(p.src) && p.src[end] == '+' && p.d.Possessive:
		end++
		greedy = "possessive"
	case p.st.ungreedy:
		greedy = "lazy"
	}
	p.pos = end
	q := p.mk(Quantifier, s, end)
	q.Min, q.Max, q.Greedy = lo, hi, greedy
	return q
}

// braceQuantifier parses {n}, {n,}, {n,m} and, where the flavor allows,
// {,m}, {,} and spaces inside the braces. Anything else is not a quantifier.
func (p *parser) braceQuantifier(s int) (lo, hi, end int, ok bool) {
	i := s + 1
	spaces := func() {
		for p.d.BraceSpaces && i < len(p.src) && (p.src[i] == ' ' || p.src[i] == '\t') {
			i++
		}
	}
	number := func() (int, bool) {
		j, v := i, 0
		for i < len(p.src) && p.src[i] >= '0' && p.src[i] <= '9' {
			v = min(v*10+int(p.src[i]-'0'), 1<<30)
			i++
		}
		return v, i > j
	}
	spaces()
	lo, hasLo := number()
	spaces()
	hi = lo
	hasHi := hasLo
	comma := i < len(p.src) && p.src[i] == ','
	if comma {
		i++
		spaces()
		hi, hasHi = number()
		spaces()
		if !hasHi {
			hi = -1
		}
	}
	if i >= len(p.src) || p.src[i] != '}' {
		return 0, 0, 0, false
	}
	if !hasLo {
		if !comma || !p.d.BraceMinOmitted || (!hasHi && !p.d.BraceNoBounds) {
			return 0, 0, 0, false
		}
		lo = 0
	}
	return lo, hi, i + 1, true
}

func (p *parser) parseAtom() *Node {
	s := p.pos
	switch p.src[s] {
	case '(':
		return p.parseGroup()
	case '[':
		return p.parseClass()
	case '.':
		p.pos++
		return p.mk(Dot, s, p.pos)
	case '^', '$':
		p.pos++
		return p.mk(Anchor, s, p.pos)
	case '\\':
		return p.parseEscape()
	}
	return p.literalRune()
}

// resolveNames fills Index for references by name once all groups are known.
func (p *parser) resolveNames(n *Node) {
	switch n.Kind {
	case NamedBackref, Recursion, Condition:
		if n.Name != "" && n.Index == 0 {
			n.Index = p.names[n.Name]
		}
	}
	for _, c := range n.Children {
		p.resolveNames(c)
	}
}
