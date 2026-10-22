package explain

import (
	"strings"
	"unicode/utf8"

	"github.com/dor1202/lazyregex/internal/engine"
)

// member is one element of a class body: a ClassItem or a node (a nested
// v-mode class, or an Unknown escape).
type member struct {
	item   *ClassItem
	node   *Node
	s, e   int
	single bool // a plain character, usable as a range endpoint
	op     string
}

// parseClass parses [...]. An unterminated class is Unknown to the end.
func (p *parser) parseClass() *Node {
	s := p.pos
	n := p.classBody(s)
	if n == nil {
		p.pos = len(p.src)
		return p.unknown(s, p.pos)
	}
	return n
}

func (p *parser) classBody(s int) *Node {
	n := p.mk(CharClass, s, s)
	p.pos = s + 1
	if p.pos < len(p.src) && p.src[p.pos] == '^' {
		n.Negated = true
		p.pos++
	}
	sets := p.d.UnicodeSets
	var ms []member
	first := true
	for {
		if p.pos >= len(p.src) {
			return nil
		}
		c := p.src[p.pos]
		if p.inQuote {
			if strings.HasPrefix(p.src[p.pos:], `\E`) {
				p.inQuote = false
				p.pos += 2
			} else {
				ms = append(ms, p.charMember())
			}
			first = false
			continue
		}
		if c == ']' && (!first || p.d.Flavor == engine.JS) {
			p.pos++
			break
		}
		first = false
		if p.st.extendedClass && (c == ' ' || c == '\t') {
			p.pos++
			continue
		}
		if sets {
			if c == '[' {
				sub := p.classBody(p.pos)
				if sub == nil {
					return nil
				}
				ms = append(ms, member{node: sub, s: sub.Span[0], e: sub.Span[1]})
				continue
			}
			if op := p.src[p.pos:min(p.pos+2, len(p.src))]; op == "--" || op == "&&" {
				ms = append(ms, member{op: op})
				p.pos += 2
				continue
			}
		}
		if m, ok := p.posixMember(); ok {
			ms = append(ms, m)
			continue
		}
		m, ok := p.classAtom()
		if !ok {
			continue
		}
		if m.single && p.isRangeDash() {
			save := p.pos
			p.pos++
			m2, ok2 := p.classAtom()
			switch {
			case ok2 && m2.single:
				m.item.Hi, m.e = m2.item.Lo, m2.e
			case ok2:
				ms = append(ms, m, member{item: &ClassItem{Lo: '-', Hi: '-'}, s: save, e: save + 1})
				m = m2
			default:
				p.pos = save
			}
		}
		ms = append(ms, m)
	}
	p.finish(n)
	p.fillClass(n, ms)
	return n
}

// isRangeDash reports whether a "-" at the current position makes a range.
func (p *parser) isRangeDash() bool {
	i := p.pos
	if i+1 >= len(p.src) || p.src[i] != '-' || p.src[i+1] == ']' {
		return false
	}
	return !p.d.UnicodeSets || p.src[i+1] != '-'
}

// fillClass stores members in n: plain members as ClassItems and nodes as
// Children, or, for a v-mode set operation, one node per operand.
func (p *parser) fillClass(n *Node, ms []member) {
	var operands [][]member
	cur := []member{}
	for _, m := range ms {
		if m.op != "" {
			n.Flags = m.op
			operands = append(operands, cur)
			cur = []member{}
			continue
		}
		cur = append(cur, m)
	}
	if n.Flags == "" {
		addMembers(n, cur)
		return
	}
	for _, o := range append(operands, cur) {
		switch {
		case len(o) == 0:
		case len(o) == 1 && o[0].node != nil:
			n.Children = append(n.Children, o[0].node)
		default:
			c := p.mk(CharClass, o[0].s, o[len(o)-1].e)
			addMembers(c, o)
			n.Children = append(n.Children, c)
		}
	}
}

func addMembers(n *Node, ms []member) {
	for _, m := range ms {
		if m.node != nil {
			n.Children = append(n.Children, m.node)
		} else {
			n.ClassItems = append(n.ClassItems, *m.item)
		}
	}
}

func (p *parser) charMember() member {
	s := p.pos
	r, w := utf8.DecodeRuneInString(p.src[s:])
	p.pos += w
	return member{item: &ClassItem{Lo: r, Hi: r}, s: s, e: p.pos, single: true}
}

// posixMember parses [:alpha:] and [:^alpha:] where the flavor has them.
func (p *parser) posixMember() (member, bool) {
	s := p.pos
	if !p.d.POSIXClasses || !strings.HasPrefix(p.src[s:], "[:") {
		return member{}, false
	}
	e := strings.Index(p.src[s+2:], ":]")
	if e < 0 {
		return member{}, false
	}
	name := p.src[s+2 : s+2+e]
	neg := strings.HasPrefix(name, "^")
	name = strings.TrimPrefix(name, "^")
	if name == "" || strings.Trim(name, "abcdefghijklmnopqrstuvwxyz") != "" {
		return member{}, false
	}
	p.pos = s + 2 + e + 2
	return member{item: &ClassItem{Shorthand: "[:" + name + ":]", Negated: neg}, s: s, e: p.pos}, true
}

// classAtom parses one character, escape or shorthand inside a class.
// ok is false when nothing was produced (\Q and \E).
func (p *parser) classAtom() (member, bool) {
	if p.pos >= len(p.src) {
		return member{}, false
	}
	if p.src[p.pos] != '\\' {
		return p.charMember(), true
	}
	return p.classEscape()
}

func (p *parser) classEscape() (member, bool) {
	s := p.pos
	f := p.d.Flavor
	if s+1 >= len(p.src) {
		p.pos = len(p.src)
		return member{}, false
	}
	c := p.src[s+1]
	single := func(r rune, e int) (member, bool) {
		p.pos = e
		return member{item: &ClassItem{Lo: r, Hi: r}, s: s, e: e, single: true}, true
	}
	unknown := func() (member, bool) {
		p.pos = s + 2
		return member{node: p.unknown(s, s+2), s: s, e: s + 2}, true
	}
	if c >= utf8.RuneSelf || !isAlnum(c) {
		r, w := utf8.DecodeRuneInString(p.src[s+1:])
		return single(r, s+1+w)
	}
	if c >= '0' && c <= '9' {
		n := p.digitEscape(true)
		if n.Kind == Literal {
			return single(rune(n.Index), n.Span[1])
		}
		return member{node: n, s: s, e: n.Span[1]}, true
	}
	switch c {
	case 'd', 'w', 's', 'D', 'W', 'S':
		return p.shorthandMember(s), true
	case 'h', 'H', 'v', 'V':
		if f == engine.PCRE {
			return p.shorthandMember(s), true
		}
	case 'p', 'P':
		if p.d.UnicodeProps {
			name, neg, e, ok := p.propAt(s)
			if !ok {
				return unknown()
			}
			p.pos = e
			return member{item: &ClassItem{Prop: name, Negated: neg}, s: s, e: e}, true
		}
	case 'q':
		if p.d.UnicodeSets && strings.HasPrefix(p.src[s+2:], "{") {
			if e := strings.IndexByte(p.src[s+2:], '}'); e >= 0 {
				p.pos = s + 2 + e + 1
				strs := strings.Split(p.src[s+3:s+2+e], "|")
				return member{item: &ClassItem{Strings: strs}, s: s, e: p.pos}, true
			}
		}
	case 'Q':
		if f == engine.PCRE {
			p.pos = s + 2
			p.inQuote = true
			return member{}, false
		}
	case 'E':
		if f == engine.PCRE {
			p.pos = s + 2
			return member{}, false
		}
	}
	if r, e, name, ok := p.charEscape(s, true); ok {
		if r < 0 { // Python \N{NAME}: kept as a Literal child, since the name is not decoded
			p.pos = e
			n := p.literal(s, e, r)
			n.Name = name
			return member{node: n, s: s, e: e}, true
		}
		return single(r, e)
	}
	if f == engine.JS && !p.d.Unicode {
		if c == 'c' { // \c without a letter: a literal backslash
			return single('\\', s+1)
		}
		return single(rune(c), s+2)
	}
	return unknown()
}

func (p *parser) shorthandMember(s int) member {
	n := p.shorthand(s)
	return member{item: &ClassItem{Shorthand: n.Name, Negated: n.Negated}, s: s, e: n.Span[1]}
}
