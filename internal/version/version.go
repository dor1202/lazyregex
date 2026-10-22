// Package version holds the build version, overridden at link time.
package version

// Version is set via -ldflags "-X github.com/dor1202/lazyregex/internal/version.Version=...".
var Version = "dev"
