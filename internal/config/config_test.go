package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dor1202/lazyregex/internal/engine"
)

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDefaults(t *testing.T) {
	c, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if c.DefaultFlavor != engine.Python || !c.Mouse || c.Timeout != 2*time.Second {
		t.Fatalf("bad defaults: %+v", c)
	}
}

func TestLoadPartial(t *testing.T) {
	d := t.TempDir()
	write(t, d, "config.yaml", "mouse: false\nunknown_key: 1\n")
	c, err := Load(d)
	if err != nil {
		t.Fatal(err)
	}
	if c.Mouse || c.DefaultFlavor != engine.Python || c.Timeout != 2*time.Second {
		t.Fatalf("bad: %+v", c)
	}
}

func TestLoadDurations(t *testing.T) {
	for in, want := range map[string]time.Duration{"2s": 2 * time.Second, "500ms": 500 * time.Millisecond, "1m": time.Minute} {
		d := t.TempDir()
		write(t, d, "config.yaml", "timeout: "+in+"\ndefault_flavor: go\ntheme:\n  accent: \"#ff0000\"\n")
		c, err := Load(d)
		if err != nil {
			t.Fatal(err)
		}
		if c.Timeout != want || c.DefaultFlavor != engine.Go || c.Theme["accent"] != "#ff0000" {
			t.Fatalf("%s: %+v", in, c)
		}
	}
}

func TestLoadInvalidValuesFallBack(t *testing.T) {
	for _, body := range []string{"timeout: 0s\n", "timeout: -5s\n", "timeout: banana\n", "timeout: [1]\n", "default_flavor: ruby\n"} {
		d := t.TempDir()
		write(t, d, "config.yaml", body)
		c, err := Load(d)
		var iv *InvalidValuesError
		if !errors.As(err, &iv) || len(iv.Problems) != 1 {
			t.Fatalf("%q: want InvalidValuesError, got %v", body, err)
		}
		if c.Timeout != 2*time.Second || c.DefaultFlavor != engine.Python || !c.Mouse {
			t.Fatalf("%q: not usable: %+v", body, c)
		}
	}
}

func TestLoadMalformed(t *testing.T) {
	d := t.TempDir()
	write(t, d, "config.yaml", "mouse: true\nfoo: [unclosed\n")
	c, err := Load(d)
	if err == nil || !strings.Contains(err.Error(), "config.yaml") || !strings.Contains(err.Error(), "line") {
		t.Fatalf("err = %v", err)
	}
	if c.Timeout != 2*time.Second || !c.Mouse {
		t.Fatalf("not defaults: %+v", c)
	}
}

func TestConfigSaveRoundTrip(t *testing.T) {
	d := filepath.Join(t.TempDir(), "sub")
	in := Config{DefaultFlavor: engine.PCRE, Mouse: false, Timeout: 1500 * time.Millisecond, Theme: map[string]string{"a": "b"}}
	if err := Save(d, in); err != nil {
		t.Fatal(err)
	}
	out, err := Load(d)
	if err != nil || !reflect.DeepEqual(in, out) {
		t.Fatalf("%v %+v", err, out)
	}
	assertPerms(t, d, 0o700)
	assertPerms(t, filepath.Join(d, "config.yaml"), 0o600)
}

func assertPerms(t *testing.T, p string, want os.FileMode) {
	t.Helper()
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != want {
		t.Fatalf("%s perm %o want %o", p, st.Mode().Perm(), want)
	}
}

func TestLibraryRoundTrip(t *testing.T) {
	d := t.TempDir()
	var l Library
	l.Put(Entry{Name: "a", Flavor: engine.JS, Flags: "gi", Pattern: `\d+`, Replacement: "<$1>", Text: "\n  indented\n\ttab\ntrailing   \n"})
	l.Put(Entry{Name: "b", Flavor: engine.Go, Text: "  lead\n\n\n"})
	l.Put(Entry{Name: "c", Flavor: engine.Python, Text: "line1\r\nline2 \t"})
	if err := SaveLibrary(d, l); err != nil {
		t.Fatal(err)
	}
	got, err := LoadLibrary(d)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(l, got) {
		t.Fatalf("round trip mismatch:\n%#v\n%#v", l, got)
	}
	assertPerms(t, filepath.Join(d, "library.yaml"), 0o600)
}

func TestLibraryMissingAndMalformed(t *testing.T) {
	d := t.TempDir()
	l, err := LoadLibrary(d)
	if err != nil || len(l.Entries) != 0 {
		t.Fatalf("%v %+v", err, l)
	}
	write(t, d, "library.yaml", "entries: [oops\n")
	l, err = LoadLibrary(d)
	if err == nil || !strings.Contains(err.Error(), "library.yaml") || len(l.Entries) != 0 {
		t.Fatalf("%v %+v", err, l)
	}
}

func TestLibraryPutReplaces(t *testing.T) {
	var l Library
	l.Put(Entry{Name: "a", Pattern: "1"})
	l.Put(Entry{Name: "b", Pattern: "2"})
	l.Put(Entry{Name: "a", Pattern: "3"})
	if len(l.Entries) != 2 || l.Entries[0].Pattern != "3" || l.Entries[1].Name != "b" {
		t.Fatalf("%+v", l.Entries)
	}
	l.Delete("a")
	l.Delete("zzz")
	if len(l.Entries) != 1 || l.Entries[0].Name != "b" {
		t.Fatalf("%+v", l.Entries)
	}
}

func TestSaveAtomic(t *testing.T) {
	d := t.TempDir()
	for i := 0; i < 3; i++ {
		if err := Save(d, Defaults()); err != nil {
			t.Fatal(err)
		}
		if err := SaveLibrary(d, Library{}); err != nil {
			t.Fatal(err)
		}
	}
	ents, _ := os.ReadDir(d)
	var names []string
	for _, e := range ents {
		names = append(names, e.Name())
	}
	if !reflect.DeepEqual(names, []string{"config.yaml", "library.yaml"}) {
		t.Fatalf("leftover files: %v", names)
	}
}

func TestLibraryRoundTripAllFields(t *testing.T) {
	vals := []string{"\n", "\n\n", "\nlead", "\n a", "\n\nx", " x", "x ", "\tx", "a\nb", "a\n", "x\r\ny", "plain", ""}
	set := map[string]func(*Entry, string){
		"Name":        func(e *Entry, v string) { e.Name = v },
		"Flags":       func(e *Entry, v string) { e.Flags = v },
		"Pattern":     func(e *Entry, v string) { e.Pattern = v },
		"Replacement": func(e *Entry, v string) { e.Replacement = v },
		"Text":        func(e *Entry, v string) { e.Text = v },
	}
	for field, f := range set {
		for _, v := range vals {
			e := Entry{Name: "n", Flavor: engine.Go}
			f(&e, v)
			d := t.TempDir()
			l := Library{Entries: []Entry{e, {Name: "other", Pattern: "p"}}}
			if err := SaveLibrary(d, l); err != nil {
				t.Fatal(err)
			}
			got, err := LoadLibrary(d)
			if err != nil || !reflect.DeepEqual(l, got) {
				t.Errorf("%s=%q: err=%v got=%#v", field, v, err, got)
			}
		}
	}
}
