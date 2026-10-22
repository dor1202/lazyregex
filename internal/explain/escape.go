package explain

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dor1202/lazyregex/internal/engine"
)

// parseEscape parses a backslash sequence outside a class. It returns nil
// for \Q and \E, which only switch quoting.
func (p *parser) parseEscape() *Node {
	s := p.pos
	if s+1 >= len(p.src) {
		p.pos = len(p.src)
		return p.unknown(s, p.pos)
	}
	c := p.src[s+1]
	if c >= utf8.RuneSelf || !isAlnum(c) {
		r, w := utf8.DecodeRuneInString(p.src[s+1:])
		p.pos = s + 1 + w
		return p.literal(s, p.pos, r)
	}
	if c >= '0' && c <= '9' {
		return p.digitEscape(false)
	}
	switch c {
	case 'd', 'w', 's', 'D', 'W', 'S':
		return p.shorthand(s)
	case 'b', 'B':
		p.pos = s + 2
		return p.mk(Anchor, s, p.pos)
	}
	if r, e, name, ok := p.charEscape(s, false); ok {
		p.pos = e
		n := p.literal(s, e, r)
		n.Name = name
		return n
	}
	f := p.d.Flavor
	switch {
	case c == 'p' || c == 'P':
		if p.d.UnicodeProps {
			return p.unicodeProp()
		}
	case c == 'k' && f == engine.JS:
		if p.d.Unicode || p.hasNamed {
			return p.namedBackref(s)
		}
	case c == 'k' || c == 'g':
		if f == engine.PCRE {
			if c == 'g' {
				return p.gEscape(s)
			}
			return p.namedBackref(s)
		}
	case c == 'K' && p.d.KeepOut:
		p.pos = s + 2
		return p.mk(KeepOut, s, p.pos)
	case c == 'Q' && p.d.Quoting:
		p.pos = s + 2
		p.inQuote = true
		return nil
	case c == 'E' && f == engine.PCRE: // a stray \E is ignored
		p.pos = s + 2
		return nil
	case c == 'c' && f == engine.JS && !p.d.Unicode: // \c without a letter is a literal backslash
		p.pos = s + 1
		return p.literal(s, p.pos, '\\')
	case strings.IndexByte(anchorLetters[f], c) >= 0:
		p.pos = s + 2
		return p.mk(Anchor, s, p.pos)
	case f == engine.PCRE && strings.IndexByte("hHvVRNXC", c) >= 0:
		return p.shorthand(s)
	}
	p.pos = s + 2
	if f == engine.JS && !p.d.Unicode { // Annex B identity escape
		return p.literal(s, p.pos, rune(c))
	}
	return p.unknown(s, p.pos)
}

// anchorLetters lists the letter anchors each flavor has beyond \b \B.
var anchorLetters = map[engine.Flavor]string{
	engine.Python: "AZz",
	engine.PCRE:   "AZzG",
	engine.Go:     "Az",
}

func (p *parser) shorthand(s int) *Node {
	p.pos = s + 2
	n := p.mk(Shorthand, s, p.pos)
	c := p.src[s+1]
	switch c {
	case 'D', 'W', 'S', 'H', 'V':
		n.Name, n.Negated = strings.ToLower(string(c)), true
	default:
		n.Name = string(c)
	}
	return n
}

// charEscape decodes a letter escape that stands for one character, such as
// \n, \x41, A, \cA or Python's \N{NAME} (r = -1, name set). It returns
// ok = false if the escape is not a character escape in this flavor.
func (p *parser) charEscape(s int, inClass bool) (r rune, end int, name string, ok bool) {
	f := p.d.Flavor
	c := p.src[s+1]
	rest := p.src[s+2:]
	switch c {
	case 't':
		return '\t', s + 2, "", true
	case 'n':
		return '\n', s + 2, "", true
	case 'r':
		return '\r', s + 2, "", true
	case 'f':
		return '\f', s + 2, "", true
	case 'v':
		if f != engine.PCRE {
			return '\v', s + 2, "", true
		}
	case 'a':
		if f != engine.JS {
			return 7, s + 2, "", true
		}
	case 'e':
		if f == engine.PCRE {
			return 0x1b, s + 2, "", true
		}
	case 'b':
		if inClass && f != engine.Go {
			return '\b', s + 2, "", true
		}
	case 'x':
		if (f == engine.PCRE || f == engine.Go) && strings.HasPrefix(rest, "{") {
			return braced(rest, s+2, 16)
		}
		n := hexLen(rest, 2)
		if n == 2 || (f == engine.PCRE && n >= 0) {
			v, _ := strconv.ParseUint("0"+rest[:n], 16, 32)
			return rune(v), s + 2 + n, "", true
		}
	case 'u':
		if f == engine.JS && p.d.Unicode && strings.HasPrefix(rest, "{") {
			return braced(rest, s+2, 16)
		}
		if (f == engine.JS || f == engine.Python) && hexLen(rest, 4) == 4 {
			v, _ := strconv.ParseUint(rest[:4], 16, 32)
			return rune(v), s + 6, "", true
		}
	case 'U':
		if f == engine.Python && hexLen(rest, 8) == 8 {
			v, _ := strconv.ParseUint(rest[:8], 16, 32)
			return rune(v), s + 10, "", true
		}
	case 'N':
		if !strings.HasPrefix(rest, "{") {
			break
		}
		e := strings.IndexByte(rest, '}')
		if e < 0 {
			break
		}
		if f == engine.Python {
			return -1, s + 2 + e + 1, rest[1:e], true
		}
		if f == engine.PCRE && strings.HasPrefix(rest, "{U+") {
			v, err := strconv.ParseUint(rest[3:e], 16, 32)
			if err == nil {
				return rune(v), s + 2 + e + 1, "", true
			}
		}
	case 'c':
		if len(rest) == 0 {
			break
		}
		l := rest[0]
		if f == engine.JS && (l|0x20 >= 'a' && l|0x20 <= 'z') {
			return rune(l % 32), s + 3, "", true
		}
		if f == engine.PCRE && l >= 0x20 && l < 0x7f {
			if l >= 'a' && l <= 'z' {
				l -= 0x20
			}
			return rune(l ^ 0x40), s + 3, "", true
		}
	case 'o':
		if f == engine.PCRE && strings.HasPrefix(rest, "{") {
			return braced(rest, s+2, 8)
		}
	}
	return 0, 0, "", false
}

// braced decodes "{digits}" in base at the start of rest; at is the offset
// of rest in the pattern.
func braced(rest string, at, base int) (rune, int, string, bool) {
	e := strings.IndexByte(rest, '}')
	if e < 0 {
		return 0, 0, "", false
	}
	v, err := strconv.ParseUint(rest[1:e], base, 32)
	if err != nil {
		return 0, 0, "", false
	}
	return rune(v), at + e + 1, "", true
}

// hexLen counts the leading hex digits of s, up to max.
func hexLen(s string, max int) int {
	n := 0
	for n < max && n < len(s) && strings.IndexByte("0123456789abcdefABCDEF", s[n]) >= 0 {
		n++
	}
	return n
}

// octal reads up to max octal digits at i and returns the value and end.
func (p *parser) octal(i, max int) (rune, int) {
	v := 0
	for n := 0; n < max && i < len(p.src) && p.src[i] >= '0' && p.src[i] <= '7'; n++ {
		v = v*8 + int(p.src[i]-'0')
		i++
	}
	return rune(v), i
}

// digitEscape parses \0, \12 etc.: a backreference or an octal escape,
// following each flavor's rules. In a class there are no backreferences.
func (p *parser) digitEscape(inClass bool) *Node {
	s := p.pos
	i := s + 1
	first := p.src[i]
	octalLit := func(max int) *Node {
		r, e := p.octal(i, max)
		p.pos = e
		return p.literal(s, e, r)
	}
	backref := func(e int) *Node {
		v, _ := strconv.Atoi(p.src[i:e])
		p.pos = e
		n := p.mk(Backref, s, e)
		n.Index = v
		return n
	}
	decimalEnd := func(max int) int {
		e := i
		for e < len(p.src) && e-i < max && p.src[e] >= '0' && p.src[e] <= '9' {
			e++
		}
		return e
	}
	isOct := func(j int) bool { return j < len(p.src) && p.src[j] >= '0' && p.src[j] <= '7' }

	switch p.d.Flavor {
	case engine.Python:
		if first == '0' {
			return octalLit(3)
		}
		if inClass || (isOct(i) && isOct(i+1) && isOct(i+2)) {
			if first > '7' {
				break
			}
			return octalLit(3)
		}
		return backref(decimalEnd(2))
	case engine.JS:
		if first == '0' && !(i+1 < len(p.src) && p.src[i+1] >= '0' && p.src[i+1] <= '9') {
			p.pos = i + 1
			return p.literal(s, p.pos, 0)
		}
		if p.d.Unicode {
			if first == '0' || inClass {
				break
			}
			return backref(decimalEnd(len(p.src)))
		}
		e := decimalEnd(len(p.src))
		if v, _ := strconv.Atoi(p.src[i:e]); !inClass && first != '0' && (p.total < 0 || v <= p.total) {
			return backref(e)
		}
		if first >= '8' { // identity escape
			p.pos = i + 1
			return p.literal(s, p.pos, rune(first))
		}
		if first <= '3' {
			return octalLit(3)
		}
		return octalLit(2)
	case engine.PCRE:
		if first == '0' {
			return octalLit(3)
		}
		e := decimalEnd(len(p.src))
		v, _ := strconv.Atoi(p.src[i:e])
		if !inClass && (v < 10 || first >= '8' || v <= p.total) {
			return backref(e)
		}
		if first >= '8' {
			break
		}
		return octalLit(3)
	case engine.Go:
		if first == '0' || (first <= '7' && isOct(i+1)) {
			return octalLit(3)
		}
	}
	p.pos = s + 2
	return p.unknown(s, p.pos)
}

// unicodeProp parses \pL, \p{Name}, \P{Name} and \p{^Name}.
func (p *parser) unicodeProp() *Node {
	s := p.pos
	name, neg, e, ok := p.propAt(s)
	if !ok {
		p.pos = s + 2
		return p.unknown(s, p.pos)
	}
	p.pos = e
	n := p.mk(UnicodeProp, s, e)
	n.Name, n.Negated = name, neg
	return n
}

func (p *parser) propAt(s int) (name string, neg bool, end int, ok bool) {
	neg = p.src[s+1] == 'P'
	i := s + 2
	if i >= len(p.src) {
		return "", false, 0, false
	}
	if p.src[i] == '{' {
		e := strings.IndexByte(p.src[i:], '}')
		if e < 0 {
			return "", false, 0, false
		}
		name = p.src[i+1 : i+e]
		if strings.HasPrefix(name, "^") && p.d.Flavor != engine.JS {
			name, neg = name[1:], !neg
		}
		return name, neg, i + e + 1, true
	}
	if p.d.Flavor == engine.JS {
		return "", false, 0, false
	}
	r, w := utf8.DecodeRuneInString(p.src[i:])
	return string(r), neg, i + w, true
}

// namedBackref parses \k<name>, \k'name' and \k{name}.
func (p *parser) namedBackref(s int) *Node {
	if s+2 < len(p.src) {
		closer := map[byte]byte{'<': '>', '\'': '\'', '{': '}'}[p.src[s+2]]
		if closer != 0 && p.d.acceptsNamedBackref(p.src[s:s+3]) {
			if e := strings.IndexByte(p.src[s+3:], closer); e >= 0 {
				p.pos = s + 3 + e + 1
				n := p.mk(NamedBackref, s, p.pos)
				n.Name = p.src[s+3 : s+3+e]
				return n
			}
		}
	}
	p.pos = s + 2
	return p.unknown(s, p.pos)
}

// gEscape parses PCRE's \g forms: \g1 \g-1 \g{1} \g{-1} are backreferences,
// \g{name} a named backreference, and \g<...> \g'...' subroutine calls.
func (p *parser) gEscape(s int) *Node {
	rest := p.src[s+2:]
	if rest != "" && (rest[0] == '{' || rest[0] == '<' || rest[0] == '\'') {
		closer := map[byte]byte{'{': '}', '<': '>', '\'': '\''}[rest[0]]
		if e := strings.IndexByte(rest[1:], closer); e >= 0 {
			inner := rest[1 : 1+e]
			p.pos = s + 2 + e + 2
			kind := Recursion
			if rest[0] == '{' {
				kind = Backref
			}
			n := p.mk(kind, s, p.pos)
			if idx, ok := p.groupNumber(inner); ok {
				n.Index = idx
			} else {
				if kind == Backref {
					n.Kind = NamedBackref
				}
				n.Name = inner
			}
			return n
		}
	} else {
		e := 0
		if e < len(rest) && (rest[0] == '-' || rest[0] == '+') {
			e++
		}
		for e < len(rest) && rest[e] >= '0' && rest[e] <= '9' {
			e++
		}
		if idx, ok := p.groupNumber(rest[:e]); ok {
			p.pos = s + 2 + e
			n := p.mk(Backref, s, p.pos)
			n.Index = idx
			return n
		}
	}
	p.pos = s + 2
	return p.unknown(s, p.pos)
}
