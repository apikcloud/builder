// SPDX-License-Identifier: MIT
package enterprise

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultCacheDir returns the on-disk directory Fetch's result cache reads
// from and writes to: a single directory shared across every repository
// this builder processes, content-addressed by ref (an immutable commit
// SHA — see Fetch's doc comment), never evicted. Mirrors
// buildkit.DefaultCacheDir's sibling-directory convention under the same
// OS user-cache root.
func DefaultCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("enterprise: resolving user cache directory: %w", err)
	}
	return filepath.Join(base, "odoo-builder", "enterprise-cache"), nil
}

// cachedFetch returns the cached tree for ref under cacheRoot if present (a
// cache hit — fetchInto is never called). On a miss, it creates a fresh
// temporary sibling directory under cacheRoot, calls fetchInto to populate
// it completely, and atomically publishes it as cacheRoot/ref via
// os.Rename — so a directory at cacheRoot/ref only ever exists once fully
// populated. If a concurrent cachedFetch for the same ref publishes first,
// this call discards its own (redundant but equally valid, since content
// is keyed by the same immutable ref) copy and reuses the concurrent one.
func cachedFetch(cacheRoot, ref string, fetchInto func(dir string) error) (string, error) {
	target := filepath.Join(cacheRoot, ref)
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		return target, nil
	}

	if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
		return "", fmt.Errorf("enterprise: creating cache directory: %w", err)
	}

	tmp, err := os.MkdirTemp(cacheRoot, ref+".tmp-*")
	if err != nil {
		return "", fmt.Errorf("enterprise: creating cache staging directory: %w", err)
	}

	if err := fetchInto(tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return "", err
	}

	if err := os.Rename(tmp, target); err != nil {
		if info, statErr := os.Stat(target); statErr == nil && info.IsDir() {
			_ = os.RemoveAll(tmp)
			return target, nil
		}
		_ = os.RemoveAll(tmp)
		return "", fmt.Errorf("enterprise: publishing cached tree: %w", err)
	}

	return target, nil
}
