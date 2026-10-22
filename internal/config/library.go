package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/dor1202/lazyregex/internal/engine"
)

const libraryFile = "library.yaml"

// Entry is one saved pattern.
type Entry struct {
	Name        string        `yaml:"name"`
	Flavor      engine.Flavor `yaml:"flavor"`
	Flags       string        `yaml:"flags"`
	Pattern     string        `yaml:"pattern"`
	Replacement string        `yaml:"replacement"`
	Text        string        `yaml:"text"`
}

// Library is the set of saved patterns.
type Library struct {
	Entries []Entry `yaml:"entries"`
}

// LoadLibrary reads library.yaml from dir. A missing file yields an empty
// library; a malformed one yields an empty library and an error naming the file.
func LoadLibrary(dir string) (Library, error) {
	path := filepath.Join(dir, libraryFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Library{}, nil
	}
	if err != nil {
		return Library{}, err
	}
	var l Library
	if err := yaml.Unmarshal(data, &l); err != nil {
		return Library{}, fmt.Errorf("%s: %w", path, err)
	}
	return l, nil
}

// Put replaces the entry with e.Name or appends e.
func (l *Library) Put(e Entry) {
	for i := range l.Entries {
		if l.Entries[i].Name == e.Name {
			l.Entries[i] = e
			return
		}
	}
	l.Entries = append(l.Entries, e)
}

// Delete removes the entry called name, if present.
func (l *Library) Delete(name string) {
	for i := range l.Entries {
		if l.Entries[i].Name == name {
			l.Entries = append(l.Entries[:i], l.Entries[i+1:]...)
			return
		}
	}
}

// SaveLibrary writes l to library.yaml in dir atomically.
func SaveLibrary(dir string, l Library) error {
	data, err := yaml.Marshal(l)
	if err != nil {
		return err
	}
	return writeAtomic(dir, libraryFile, data)
}

// MarshalYAML forces double-quoted style for text that yaml's literal block
// style would not preserve exactly (leading spaces/tabs on the first line,
// carriage returns, trailing whitespace), keeping the round trip lossless.
func (e Entry) MarshalYAML() (any, error) {
	type plain Entry
	var n yaml.Node
	if err := n.Encode(plain(e)); err != nil {
		return nil, err
	}
	if needsQuoting(e.Text) {
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == "text" {
				n.Content[i+1].Style = yaml.DoubleQuotedStyle
			}
		}
	}
	return &n, nil
}

func needsQuoting(s string) bool {
	if s == "" {
		return false
	}
	if s[0] == ' ' || s[0] == '\t' || strings.Contains(s, "\r") {
		return true
	}
	last := s[len(s)-1]
	return last == ' ' || last == '\t'
}
