package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestVersionFlag(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{"--version"}, strings.NewReader(""), &out, io.Discard)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.HasPrefix(out.String(), "lazyregex dev") {
		t.Fatalf("got %q", out.String())
	}
}
