// SPDX-License-Identifier: MIT
// Package buildkit (see runner.go) — this file holds the pure, side-effect-
// free helpers that turn BuildOptions' cache/platform settings into
// buildctl flags, kept separate from execRunner.Build so they are testable
// without a buildctl/buildkitd binary.
package buildkit

import "strings"

// cacheArgs returns the --export-cache/--import-cache flags for one
// buildctl invocation. cacheRef (registry-typed) takes precedence over
// cacheDir (local-typed) when both are non-empty; both empty returns nil
// (no cache flags at all).
func cacheArgs(cacheRef, cacheDir string) []string {
	switch {
	case cacheRef != "":
		return []string{
			"--export-cache", "type=registry,ref=" + cacheRef + ",mode=max",
			"--import-cache", "type=registry,ref=" + cacheRef,
		}
	case cacheDir != "":
		return []string{
			"--export-cache", "type=local,dest=" + cacheDir + ",mode=max",
			"--import-cache", "type=local,src=" + cacheDir,
		}
	default:
		return nil
	}
}

// platformArgs returns the --opt platform=... flag for one buildctl
// invocation, or nil when platforms is empty.
func platformArgs(platforms []string) []string {
	if len(platforms) == 0 {
		return nil
	}
	return []string{"--opt", "platform=" + strings.Join(platforms, ",")}
}
