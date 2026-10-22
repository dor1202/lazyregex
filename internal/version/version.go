// Package version holds the build version, overridden at link time, and the
// pinned versions of the bundled engines.
package version

import (
	"runtime/debug"
	"sync"
)

// Version is set via -ldflags "-X github.com/dor1202/lazyregex/internal/version.Version=...".
var Version = "dev"

// Versions of the bundled WASM engines; they must equal wasm/versions.env
// (checked by TestEngineVersionsMatchVersionsEnv).
const (
	CPython   = "3.14.9"
	PCRE2     = "10.44"
	QuickJSNG = "0.17.0"
)

// String is Version, or, for a build without -ldflags (go install
// ...@v1.0.0), the module version recorded in the binary.
func String() string { return resolved() }

var resolved = sync.OnceValue(func() string {
	info, _ := debug.ReadBuildInfo()
	return resolve(Version, info)
})

func resolve(linked string, info *debug.BuildInfo) string {
	if linked != "dev" || info == nil {
		return linked
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	return linked
}
