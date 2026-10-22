// Package config loads and saves the user config and the pattern library.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/dor1202/lazyregex/internal/engine"
)

const (
	configFile  = "config.yaml"
	defaultTime = 2 * time.Second
)

// Config is the user configuration.
type Config struct {
	DefaultFlavor engine.Flavor
	Mouse         bool
	Timeout       time.Duration
	Theme         map[string]string // optional colour overrides by token name
}

// fileConfig is the on-disk form; Timeout is kept as a node so human
// durations like "500ms" can be parsed and bad values reported.
type fileConfig struct {
	DefaultFlavor string            `yaml:"default_flavor,omitempty"`
	Mouse         *bool             `yaml:"mouse,omitempty"`
	Timeout       yaml.Node         `yaml:"timeout,omitempty"`
	Theme         map[string]string `yaml:"theme,omitempty"`
}

// InvalidValuesError reports config values that were replaced by defaults.
// It is non-fatal: Load returns a usable Config alongside it.
type InvalidValuesError struct {
	File     string
	Problems []string
}

func (e *InvalidValuesError) Error() string {
	return fmt.Sprintf("%s: invalid values replaced with defaults: %s", e.File, strings.Join(e.Problems, "; "))
}

// Defaults returns the default configuration.
func Defaults() Config {
	return Config{DefaultFlavor: engine.Python, Mouse: true, Timeout: defaultTime}
}

// Dir returns the lazyregex directory inside the OS config dir.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "lazyregex"), nil
}

// Load reads config.yaml from dir. A missing file yields defaults. Load always
// returns a usable Config: on a malformed file it returns defaults plus an
// error naming the file and line; on invalid values it substitutes defaults
// and returns an *InvalidValuesError.
func Load(dir string) (Config, error) {
	c := Defaults()
	path := filepath.Join(dir, configFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	var f fileConfig
	if err := yaml.Unmarshal(data, &f); err != nil {
		return Defaults(), fmt.Errorf("%s: %w", path, err)
	}
	var problems []string
	if f.DefaultFlavor != "" {
		fl := engine.Flavor(f.DefaultFlavor)
		known := false
		for _, k := range engine.Flavors {
			known = known || k == fl
		}
		if known {
			c.DefaultFlavor = fl
		} else {
			problems = append(problems, fmt.Sprintf("default_flavor %q is unknown, using python", f.DefaultFlavor))
		}
	}
	if f.Mouse != nil {
		c.Mouse = *f.Mouse
	}
	if f.Timeout.Kind != 0 {
		d, err := time.ParseDuration(f.Timeout.Value)
		switch {
		case f.Timeout.Kind != yaml.ScalarNode || err != nil:
			problems = append(problems, fmt.Sprintf("line %d: timeout is not a duration like 2s, using 2s", f.Timeout.Line))
		case d <= 0:
			problems = append(problems, fmt.Sprintf("line %d: timeout must be positive, using 2s", f.Timeout.Line))
		default:
			c.Timeout = d
		}
	}
	c.Theme = f.Theme
	if len(problems) > 0 {
		return c, &InvalidValuesError{File: path, Problems: problems}
	}
	return c, nil
}

// Save writes c to config.yaml in dir atomically.
func Save(dir string, c Config) error {
	f := struct {
		DefaultFlavor string            `yaml:"default_flavor"`
		Mouse         bool              `yaml:"mouse"`
		Timeout       string            `yaml:"timeout"`
		Theme         map[string]string `yaml:"theme,omitempty"`
	}{string(c.DefaultFlavor), c.Mouse, c.Timeout.String(), c.Theme}
	data, err := yaml.Marshal(f)
	if err != nil {
		return err
	}
	return writeAtomic(dir, configFile, data)
}

// writeAtomic writes data to dir/name via a synced temp file and rename.
func writeAtomic(dir, name string, data []byte) (err error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+name+".tmp-*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()
	if err = tmp.Chmod(0o600); err != nil {
		return err
	}
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(dir, name))
}
