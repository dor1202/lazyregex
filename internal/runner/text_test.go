package runner

import (
	"bytes"
	"testing"
)

func TestPrepareTextInvalidUTF8(t *testing.T) {
	p := PrepareText([]byte{0x66, 0xff, 0x6f})
	if p.Text != "f�o" || !p.InvalidUTF8 {
		t.Fatalf("got %+v", p)
	}
	if q := PrepareText([]byte("héllo")); q.InvalidUTF8 || q.Text != "héllo" {
		t.Fatalf("valid text altered: %+v", q)
	}
}

func TestPrepareTextLarge(t *testing.T) {
	if PrepareText(bytes.Repeat([]byte("a"), 1<<20+1)).Manual != true {
		t.Fatal("1 MiB+1 should be manual")
	}
	if PrepareText(bytes.Repeat([]byte("a"), 1<<20)).Manual {
		t.Fatal("1 MiB should not be manual")
	}
}
