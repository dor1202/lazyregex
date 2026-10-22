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

// MarshalYAML forces double-quoted style for every string field that yaml's
// block styles would not preserve exactly (any newline, carriage return, or
// leading/trailing whitespace), keeping the round trip lossless.
func (e Entry) MarshalYAML() (any, error) {
	// Nodes are built by hand: Node.Encode on a struct already mangles
	// multi-line strings (it drops a leading newline) before styling.
	n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	add := func(key, val string) {
		v := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: val}
		if needsQuoting(val) {
			v.Style = yaml.DoubleQuotedStyle
		}
		n.Content = append(n.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, v)
	}
	add("name", e.Name)
	add("flavor", string(e.Flavor))
	add("flags", e.Flags)
	add("pattern", e.Pattern)
	add("replacement", e.Replacement)
	add("text", e.Text)
	return n, nil
}

func needsQuoting(s string) bool {
	if s == "" {
		return false
	}
	if strings.ContainsAny(s, "\n\r") {
		return true
	}
	return strings.ContainsAny(s[:1], " \t") || strings.ContainsAny(s[len(s)-1:], " \t")
}
