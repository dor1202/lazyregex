// Package syntax colours the Code tab's snippets with chroma.
//
// Only the four lexers and two styles the snippets need are compiled in
// (chroma's lexers, styles and formatters packages register every lexer,
// style and an embedded SVG font, about 4.5 MB of binary); their
// definitions are copied from chroma in chroma/ (see chroma/LICENSE).
package syntax

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/charmbracelet/x/ansi"
)

//go:embed chroma/*.xml
var defs embed.FS

// Languages with a lexer.
const (
	Python     = "python"
	JavaScript = "javascript"
	PHP        = "php"
	Go         = "go"
)

// Styles.
const (
	Dark  = "catppuccin-mocha"
	Light = "catppuccin-latte"
)

// Reset is the SGR reset every highlighted line ends with, so no colour or
// attribute runs past it.
const Reset = "\x1b[m"

// phpOpen is the tag a PHP snippet starts with; chroma's PHP lexer expects
// to start inside PHP code, so the tag is emitted as a token of its own.
const phpOpen = "<?php"

const underline = "\x1b[4m"

var (
	loadOnce sync.Once
	lexers   map[string]chroma.Lexer
	styles   map[string]*chroma.Style
)

func load() {
	lexers = map[string]chroma.Lexer{Go: goLexer}
	for lang, file := range map[string]string{Python: "python", JavaScript: "javascript", PHP: "php"} {
		if l, err := chroma.NewXMLLexer(defs, "chroma/"+file+".xml"); err == nil {
			lexers[lang] = l
		}
	}
	styles = map[string]*chroma.Style{}
	for _, name := range []string{Dark, Light} {
		if st, err := loadStyle(name); err == nil {
			styles[name] = st
		}
	}
}

// loadStyle reads a style and drops every background colour, so the panel
// background shows through every token.
func loadStyle(name string) (*chroma.Style, error) {
	b, err := defs.ReadFile("chroma/" + name + ".xml")
	if err != nil {
		return nil, err
	}
	base, err := chroma.NewXMLStyle(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	sb := chroma.NewStyleBuilder(base.Name)
	for _, tt := range append(base.Types(), chroma.Background) {
		e := base.Get(tt)
		e.Background = 0
		sb.AddEntry(tt, e)
	}
	return sb.Build()
}

// Style returns the named style (backgrounds removed), or nil.
func Style(name string) *chroma.Style {
	loadOnce.Do(load)
	return styles[name]
}

// Lines colours lines of lang source in the named style with truecolor
// SGR sequences (the renderer downsamples them to the terminal's colour
// profile), one output line per input line, each ending with Reset. The
// lines must already be safe to draw: the output adds SGR sequences and
// nothing else, which is checked, and on any failure (an unknown language
// or style, a lexer error) the lines come back uncoloured.
func Lines(lang string, lines []string, style string) []string {
	loadOnce.Do(load)
	out, ok := highlight(lexers[lang], lang == PHP, lines, styles[style])
	if !ok {
		out = make([]string, len(lines))
		for i, l := range lines {
			out[i] = l + Reset
		}
	}
	return out
}

func highlight(lexer chroma.Lexer, php bool, lines []string, style *chroma.Style) ([]string, bool) {
	if lexer == nil || style == nil {
		return nil, false
	}
	src := strings.Join(lines, "\n") + "\n"
	var lead []chroma.Token
	if php && strings.HasPrefix(src, phpOpen) {
		lead = []chroma.Token{{Type: chroma.CommentPreproc, Value: phpOpen}}
		src = src[len(phpOpen):]
	}
	it, err := chroma.Coalesce(lexer).Tokenise(nil, src)
	if err != nil {
		return nil, false
	}
	out := make([]string, 0, len(lines))
	var cur strings.Builder
	open := "" // the SGR in effect on cur
	emit := func(t chroma.Token) {
		sgr := sgrOf(style.Get(t.Type))
		for i, seg := range strings.Split(t.Value, "\n") {
			if i > 0 {
				out = append(out, cur.String()+Reset)
				cur.Reset()
				open = ""
			}
			if seg == "" {
				continue
			}
			want := sgr
			// Blanks keep the current SGR unless it underlines.
			if strings.Trim(seg, " ") == "" && !strings.Contains(open, underline) {
				want = open
			}
			if want != open {
				if open != "" {
					cur.WriteString(Reset)
				}
				cur.WriteString(want)
				open = want
			}
			cur.WriteString(seg)
		}
	}
	for _, t := range lead {
		emit(t)
	}
	for t := it(); t != chroma.EOF; t = it() {
		emit(t)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String()+Reset)
	}
	if len(out) != len(lines) {
		return nil, false
	}
	for i, l := range out {
		if ansi.Strip(l) != lines[i] {
			return nil, false // the lexer must only add colour
		}
	}
	return out, true
}

// sgrOf is the truecolor SGR sequence of a style entry, as chroma's
// terminal16m formatter writes it (without the background).
func sgrOf(e chroma.StyleEntry) string {
	var sb strings.Builder
	if e.Bold == chroma.Yes {
		sb.WriteString("\x1b[1m")
	}
	if e.Underline == chroma.Yes {
		sb.WriteString(underline)
	}
	if e.Italic == chroma.Yes {
		sb.WriteString("\x1b[3m")
	}
	if e.Colour.IsSet() {
		fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%dm", e.Colour.Red(), e.Colour.Green(), e.Colour.Blue())
	}
	return sb.String()
}
