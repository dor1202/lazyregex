package version

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

// The engine versions shown by --version and --licenses must be the ones the
// WASM engines are built from.
func TestEngineVersionsMatchVersionsEnv(t *testing.T) {
	f, err := os.Open(filepath.Join("..", "..", "wasm", "versions.env"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	env := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if k, v, ok := strings.Cut(line, "="); ok && !strings.HasPrefix(line, "#") {
			env[k] = v
		}
	}
	for key, got := range map[string]string{
		"CPYTHON_VERSION":    CPython,
		"PCRE2_VERSION":      PCRE2,
		"QUICKJS_NG_VERSION": QuickJSNG,
	} {
		if env[key] != got {
			t.Errorf("%s = %q in wasm/versions.env, but the constant is %q", key, env[key], got)
		}
	}
}

func TestResolve(t *testing.T) {
	info := func(v string) *debug.BuildInfo { return &debug.BuildInfo{Main: debug.Module{Version: v}} }
	for _, c := range []struct {
		linked string
		info   *debug.BuildInfo
		want   string
	}{
		{"1.0.0", info("v1.0.0"), "1.0.0"}, // set by -ldflags: wins
		{"dev", info("v1.0.0"), "v1.0.0"},  // go install ...@v1.0.0
		{"dev", info("(devel)"), "dev"},    // go build in a checkout
		{"dev", info(""), "dev"},
		{"dev", nil, "dev"}, // no build info
	} {
		if got := resolve(c.linked, c.info); got != c.want {
			t.Errorf("resolve(%q, %v) = %q, want %q", c.linked, c.info, got, c.want)
		}
	}
}
