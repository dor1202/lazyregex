// Package codegen turns the current pattern, flavor, mode and flags into a
// ready-to-paste program for that flavor's language.
package codegen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dor1202/lazyregex/internal/engine"
)

// Snippet returns a complete program that runs req the way the engine does:
// the "g" flag selects the find-all / replace-all form, otherwise only the
// first match is used.
func Snippet(f engine.Flavor, req engine.Request) string {
	switch f {
	case engine.Python:
		return python(req)
	case engine.JS:
		return javascript(req)
	case engine.PCRE:
		return php(req)
	case engine.Go:
		return golang(req)
	}
	return ""
}

func mode(req engine.Request) engine.Mode {
	if req.Mode == "" {
		return engine.ModeMatch
	}
	return req.Mode
}

// canonical returns the letters of order that appear in flags, in order's order.
func canonical(flags, order string) string {
	var b strings.Builder
	for _, c := range order {
		if strings.ContainsRune(flags, c) {
			b.WriteRune(c)
		}
	}
	return b.String()
}

func hasGlobal(req engine.Request) bool { return strings.Contains(req.Flags, "g") }

// ---- Python ----

var pyFlags = []struct {
	letter rune
	name   string
}{{'i', "re.IGNORECASE"}, {'m', "re.MULTILINE"}, {'s', "re.DOTALL"}, {'x', "re.VERBOSE"}, {'a', "re.ASCII"}}

func python(req engine.Request) string {
	var fl []string
	for _, p := range pyFlags {
		if strings.ContainsRune(req.Flags, p.letter) {
			fl = append(fl, p.name)
		}
	}
	args := pyString(req.Pattern, true)
	if len(fl) > 0 {
		args += ", " + strings.Join(fl, " | ")
	}
	var b strings.Builder
	b.WriteString("import re\n\n")
	fmt.Fprintf(&b, "pattern = re.compile(%s)\n", args)
	fmt.Fprintf(&b, "text = %s\n\n", pyString(req.Text, false))
	switch mode(req) {
	case engine.ModeReplace:
		count := 1
		if hasGlobal(req) {
			count = 0
		}
		fmt.Fprintf(&b, "result = pattern.sub(%s, text, count=%d)\nprint(result)\n", pyString(req.Replacement, true), count)
	case engine.ModeSplit:
		b.WriteString("parts = pattern.split(text)\nprint(parts)\n")
	default:
		if hasGlobal(req) {
			b.WriteString("for m in pattern.finditer(text):\n    print(m.span(), m.group(0), m.groups())\n")
		} else {
			b.WriteString("m = pattern.search(text)\nif m:\n    print(m.span(), m.group(0), m.groups())\n")
		}
	}
	return b.String()
}

// pyString quotes s as a Python literal. With raw set it prefers r"…", which
// cannot end in an odd number of backslashes or contain a quote (or a line
// break, which a one-line literal cannot hold); otherwise it escapes.
func pyString(s string, raw bool) string {
	if raw && !strings.Contains(s, `"`) && !strings.ContainsAny(s, "\r\n") && trailingBackslashes(s)%2 == 0 {
		return `r"` + s + `"`
	}
	return escapeDouble(s, false)
}

func trailingBackslashes(s string) int {
	n := 0
	for i := len(s) - 1; i >= 0 && s[i] == '\\'; i-- {
		n++
	}
	return n
}

// escapeDouble quotes s in double quotes using escapes both Python and
// JavaScript understand. js additionally escapes U+2028/2029.
func escapeDouble(s string, js bool) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == '"':
			b.WriteString(`\"`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, r)
		case js && (r == 0x2028 || r == 0x2029):
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// ---- JavaScript ----

func javascript(req engine.Request) string {
	flags := canonical(req.Flags, "dgimsuvy")
	body := escapeSlashes(req.Pattern)
	body = strings.NewReplacer("\n", `\n`, "\r", `\r`, "\u2028", `\u2028`, "\u2029", `\u2029`).Replace(body)
	if body == "" {
		body = "(?:)" // // would start a comment
	}
	var b strings.Builder
	fmt.Fprintf(&b, "const re = /%s/%s;\n", body, flags)
	fmt.Fprintf(&b, "const text = %s;\n\n", escapeDouble(req.Text, true))
	switch mode(req) {
	case engine.ModeReplace:
		fmt.Fprintf(&b, "const result = text.replace(re, %s);\nconsole.log(result);\n", escapeDouble(req.Replacement, true))
	case engine.ModeSplit:
		b.WriteString("const parts = text.split(re);\nconsole.log(parts);\n")
	default:
		if strings.Contains(flags, "g") {
			b.WriteString("for (const m of text.matchAll(re)) {\n  console.log(m.index, m[0], m.slice(1));\n}\n")
		} else {
			b.WriteString("const m = re.exec(text);\nif (m) {\n  console.log(m.index, m[0], m.slice(1));\n}\n")
		}
	}
	return b.String()
}

// escapeSlashes puts a backslash before every '/' that is not already
// escaped (not preceded by an odd number of backslashes).
func escapeSlashes(p string) string {
	var b strings.Builder
	backslashes := 0
	for i := 0; i < len(p); i++ {
		c := p[i]
		if c == '/' && backslashes%2 == 0 {
			b.WriteByte('\\')
		}
		b.WriteByte(c)
		if c == '\\' {
			backslashes++
		} else {
			backslashes = 0
		}
	}
	return b.String()
}

// ---- PHP (PCRE) ----

func php(req engine.Request) string {
	mods := strings.ReplaceAll(req.Flags, "g", "")
	mods = canonical(mods, "imsxnUu")
	re := phpString("/" + escapeSlashes(req.Pattern) + "/" + mods)
	var b strings.Builder
	b.WriteString("<?php\n")
	fmt.Fprintf(&b, "$text = %s;\n\n", phpString(req.Text))
	switch mode(req) {
	case engine.ModeReplace:
		limit := 1
		if hasGlobal(req) {
			limit = -1
		}
		fmt.Fprintf(&b, "$result = preg_replace(%s, %s, $text, %d);\nvar_dump($result);\n", re, phpString(req.Replacement), limit)
	case engine.ModeSplit:
		fmt.Fprintf(&b, "$parts = preg_split(%s, $text);\nprint_r($parts);\n", re)
	default:
		if hasGlobal(req) {
			fmt.Fprintf(&b, "preg_match_all(%s, $text, $m, PREG_SET_ORDER | PREG_OFFSET_CAPTURE);\nprint_r($m);\n", re)
		} else {
			fmt.Fprintf(&b, "preg_match(%s, $text, $m, PREG_OFFSET_CAPTURE);\nprint_r($m);\n", re)
		}
	}
	return b.String()
}

func phpString(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}

// ---- Go ----

func golang(req engine.Request) string {
	pat := req.Pattern
	if fl := canonical(req.Flags, "imsU"); fl != "" {
		pat = "(?" + fl + ")" + pat
	}
	var b strings.Builder
	b.WriteString("package main\n\nimport (\n\t\"fmt\"\n\t\"regexp\"\n)\n\nfunc main() {\n")
	fmt.Fprintf(&b, "\tre := regexp.MustCompile(%s)\n", goString(pat))
	fmt.Fprintf(&b, "\ttext := %s\n\n", goString(req.Text))
	switch mode(req) {
	case engine.ModeReplace:
		repl := goString(req.Replacement)
		if hasGlobal(req) {
			fmt.Fprintf(&b, "\tresult := re.ReplaceAllString(text, %s)\n", repl)
		} else {
			// Go has no replace-first call: expand the template for the first match only.
			fmt.Fprintf(&b, "\tresult := text\n\tif loc := re.FindStringSubmatchIndex(text); loc != nil {\n"+
				"\t\trepl := re.ExpandString(nil, %s, text, loc)\n"+
				"\t\tresult = text[:loc[0]] + string(repl) + text[loc[1]:]\n\t}\n", repl)
		}
		b.WriteString("\tfmt.Printf(\"%q\\n\", result)\n")
	case engine.ModeSplit:
		b.WriteString("\tparts := re.Split(text, -1)\n\tfmt.Printf(\"%q\\n\", parts)\n")
	default:
		if hasGlobal(req) {
			b.WriteString("\tfor _, m := range re.FindAllStringSubmatch(text, -1) {\n\t\tfmt.Printf(\"%q\\n\", m)\n\t}\n")
		} else {
			b.WriteString("\tif m := re.FindStringSubmatch(text); m != nil {\n\t\tfmt.Printf(\"%q\\n\", m)\n\t}\n")
		}
	}
	b.WriteString("}\n")
	return b.String()
}

// goString prefers a raw string literal; it falls back to a quoted one when
// s holds a backtick or a carriage return (which raw strings drop).
func goString(s string) string {
	if strings.ContainsAny(s, "`\r") {
		return strconv.Quote(s)
	}
	return "`" + s + "`"
}
