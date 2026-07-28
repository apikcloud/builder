// SPDX-License-Identifier: MIT
package workspace

// DefaultIgnore lists paths excluded when copying a repository into a
// build context. Addon-aware ignore rules (e.g. per-module excludes)
// arrive with Milestone 2's flattening logic.
func DefaultIgnore() []string {
	return []string{".git", ".build"}
}
