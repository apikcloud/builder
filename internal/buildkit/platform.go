// SPDX-License-Identifier: MIT
// Package buildkit (see runner.go).
package buildkit

import (
	"fmt"
	"regexp"
)

// platformPattern matches BuildKit/OCI platform strings: "os/arch" with an
// optional "/variant" component, e.g. "linux/amd64", "linux/arm/v7". It is
// intentionally permissive about which os/arch/variant names it accepts —
// buildctl/buildkitd are the final authority at build time — the point is
// catching obviously malformed entries (missing slash, embedded whitespace,
// empty segments) in odoo-builder.yaml before a build even starts.
var platformPattern = regexp.MustCompile(`^[a-z0-9]+/[a-z0-9]+(/[a-z0-9]+)?$`)

// ValidatePlatform reports whether s looks like a well-formed "os/arch" (or
// "os/arch/variant") platform string.
func ValidatePlatform(s string) error {
	if !platformPattern.MatchString(s) {
		return fmt.Errorf("buildkit: invalid platform %q (expected \"os/arch\", e.g. \"linux/amd64\")", s)
	}
	return nil
}
