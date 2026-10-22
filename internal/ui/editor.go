package ui

import (
	"strings"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

// editor is the text buffer behind the pattern, replacement and test-string
// editors: a string, a cursor (a byte offset on a rune boundary) and an
// undo/redo history of small edits. It stores the text exactly as given —
// tabs, carriage returns, control characters and U+FFFD survive — and has no
// size or line limit; rendering is done by the panels.
type editor struct {
	text      string
	cur       int
	multiline bool
	goal      int // rune column kept by up/down; -1 = the current column

	undo, redo []edit
	typing     bool // the last edit was typed text (consecutive typing is one undo step)
	loaded     bool // Sync has seen the first value
}

// edit replaces old with new at pos; before and after are the cursor
// positions around it.
type edit struct {
	pos           int
	old, new      string
	before, after int
}

// maxUndo bounds the history.
const maxUndo = 200

func newEditor(multiline bool) *editor { return &editor{multiline: multiline, goal: -1} }

// SetValue replaces the text, puts the cursor at the end and clears the
// history (it is used for text that changed outside the editor).
func (e *editor) SetValue(s string) {
	e.text, e.cur, e.goal = s, len(s), -1
	e.undo, e.redo, e.typing = nil, nil, false
}

// Sync takes a value that changed outside the editor (a library load, an
// applied rewrite). The first value is loaded as is; every later change is
// recorded as one undoable edit, so ctrl+z brings the previous text back.
func (e *editor) Sync(s string) {
	switch {
	case s == e.text:
	case !e.loaded:
		e.SetValue(s)
	default:
		e.goal, e.typing = -1, false
		e.replace(0, len(e.text), s)
	}
	e.loaded = true
}

func (e *editor) Value() string { return e.text }
func (e *editor) Cursor() int   { return e.cur }

// Update applies a key press or paste and reports whether the text changed.
func (e *editor) Update(msg tea.Msg) bool {
	switch msg := msg.(type) {
	case tea.PasteMsg:
		e.typing = false
		return e.insert(e.clean(msg.Content), false)
	case tea.KeyPressMsg:
		return e.key(msg)
	}
	return false
}

// clean normalises pasted line endings; a single-line editor drops them.
func (e *editor) clean(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	if !e.multiline {
		s = strings.ReplaceAll(s, "\n", "")
	}
	return s
}

func (e *editor) key(k tea.KeyPressMsg) bool {
	ks := k.String()
	if ks != "up" && ks != "down" {
		e.goal = -1
	}
	if ks != "ctrl+z" && ks != "ctrl+y" && !isTyped(k) {
		e.typing = false
	}
	switch ks {
	case "ctrl+z":
		return e.undoLast()
	case "ctrl+y":
		return e.redoLast()
	case "left", "ctrl+b":
		e.cur = prevRune(e.text, e.cur)
	case "right", "ctrl+f":
		e.cur = nextRune(e.text, e.cur)
	case "alt+left", "ctrl+left", "alt+b":
		e.cur = wordLeft(e.text, e.cur)
	case "alt+right", "ctrl+right", "alt+f":
		e.cur = wordRight(e.text, e.cur)
	case "home", "ctrl+a":
		e.cur = lineStart(e.text, e.cur)
	case "end", "ctrl+e":
		e.cur = lineEnd(e.text, e.cur)
	case "ctrl+home", "alt+<":
		e.cur = 0
	case "ctrl+end", "alt+>":
		e.cur = len(e.text)
	case "up":
		e.vertical(-1)
	case "down":
		e.vertical(+1)
	case "backspace", "ctrl+h":
		return e.replace(prevRune(e.text, e.cur), e.cur, "")
	case "delete", "ctrl+d":
		return e.replace(e.cur, nextRune(e.text, e.cur), "")
	case "alt+backspace", "ctrl+w":
		return e.replace(wordLeft(e.text, e.cur), e.cur, "")
	case "alt+delete", "alt+d":
		return e.replace(e.cur, wordRight(e.text, e.cur), "")
	case "ctrl+u":
		return e.replace(lineStart(e.text, e.cur), e.cur, "")
	case "ctrl+k":
		end := lineEnd(e.text, e.cur)
		if end == e.cur && e.cur < len(e.text) { // at the end of a line: join the next
			end++
		}
		return e.replace(e.cur, end, "")
	case "enter":
		if e.multiline {
			return e.insert("\n", false)
		}
	default:
		if isTyped(k) {
			return e.insert(k.Text, true)
		}
	}
	return false
}

// isTyped reports whether the key inserts its text.
func isTyped(k tea.KeyPressMsg) bool {
	return k.Text != "" && k.Mod&(tea.ModCtrl|tea.ModAlt|tea.ModMeta|tea.ModSuper) == 0
}

func (e *editor) insert(s string, typed bool) bool {
	if s == "" {
		return false
	}
	if typed && e.typing && len(e.undo) > 0 {
		// Extend the previous typed insert so a typed word undoes at once.
		last := &e.undo[len(e.undo)-1]
		if last.old == "" && last.pos+len(last.new) == e.cur {
			e.text = e.text[:e.cur] + s + e.text[e.cur:]
			e.cur += len(s)
			last.new += s
			last.after = e.cur
			e.redo = nil
			return true
		}
	}
	changed := e.replace(e.cur, e.cur, s)
	e.typing = typed
	return changed
}

// replace swaps text[from:to] for s, records the edit and leaves the cursor
// after s.
func (e *editor) replace(from, to int, s string) bool {
	if from == to && s == "" {
		return false
	}
	ed := edit{pos: from, old: e.text[from:to], new: s, before: e.cur, after: from + len(s)}
	e.apply(ed.pos, ed.old, ed.new)
	e.cur = ed.after
	e.undo = append(e.undo, ed)
	if len(e.undo) > maxUndo {
		e.undo = e.undo[1:]
	}
	e.redo = nil
	return true
}

func (e *editor) apply(pos int, old, new string) {
	e.text = e.text[:pos] + new + e.text[pos+len(old):]
}

func (e *editor) undoLast() bool {
	if len(e.undo) == 0 {
		return false
	}
	ed := e.undo[len(e.undo)-1]
	e.undo = e.undo[:len(e.undo)-1]
	e.apply(ed.pos, ed.new, ed.old)
	e.cur = ed.before
	e.redo = append(e.redo, ed)
	e.typing = false
	return true
}

func (e *editor) redoLast() bool {
	if len(e.redo) == 0 {
		return false
	}
	ed := e.redo[len(e.redo)-1]
	e.redo = e.redo[:len(e.redo)-1]
	e.apply(ed.pos, ed.old, ed.new)
	e.cur = ed.after
	e.undo = append(e.undo, ed)
	e.typing = false
	return true
}

// vertical moves the cursor dir lines, keeping the rune column.
func (e *editor) vertical(dir int) {
	start := lineStart(e.text, e.cur)
	if e.goal < 0 {
		e.goal = utf8.RuneCountInString(e.text[start:e.cur])
	}
	var target int
	if dir < 0 {
		if start == 0 {
			return
		}
		target = lineStart(e.text, start-1)
	} else {
		end := lineEnd(e.text, e.cur)
		if end == len(e.text) {
			return
		}
		target = end + 1
	}
	end := lineEnd(e.text, target)
	pos := target
	for n := 0; n < e.goal && pos < end; n++ {
		pos = nextRune(e.text, pos)
	}
	e.cur = pos
}

// RowCol returns the cursor's line (0-based) and the byte offset of that
// line's start.
func (e *editor) RowCol() (row, lineStartByte int) {
	return strings.Count(e.text[:e.cur], "\n"), lineStart(e.text, e.cur)
}

func prevRune(s string, i int) int {
	if i <= 0 {
		return 0
	}
	_, n := utf8.DecodeLastRuneInString(s[:i])
	return i - n
}

func nextRune(s string, i int) int {
	if i >= len(s) {
		return len(s)
	}
	_, n := utf8.DecodeRuneInString(s[i:])
	return i + n
}

func lineStart(s string, i int) int { return strings.LastIndexByte(s[:i], '\n') + 1 }

func lineEnd(s string, i int) int {
	if j := strings.IndexByte(s[i:], '\n'); j >= 0 {
		return i + j
	}
	return len(s)
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }

// wordLeft skips non-word runes, then word runes, to the left.
func wordLeft(s string, i int) int {
	for i > 0 {
		r, n := utf8.DecodeLastRuneInString(s[:i])
		if isWordRune(r) {
			break
		}
		i -= n
	}
	for i > 0 {
		r, n := utf8.DecodeLastRuneInString(s[:i])
		if !isWordRune(r) {
			break
		}
		i -= n
	}
	return i
}

// wordRight skips non-word runes, then word runes, to the right.
func wordRight(s string, i int) int {
	for i < len(s) {
		r, n := utf8.DecodeRuneInString(s[i:])
		if isWordRune(r) {
			break
		}
		i += n
	}
	for i < len(s) {
		r, n := utf8.DecodeRuneInString(s[i:])
		if !isWordRune(r) {
			break
		}
		i += n
	}
	return i
}
