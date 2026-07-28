// Package version exposes build-time metadata injected via -ldflags.
package version

import "fmt"

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func String() string {
	return fmt.Sprintf("%s (commit=%s, built=%s)", Version, Commit, Date)
}
