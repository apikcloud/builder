// Package buildkit (see runner.go).
package buildkit

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultCacheDir returns the on-disk directory BuildKit's local cache
// importer/exporter read from and write to when odoo-builder.yaml's
// cache.enabled is true and no image.name is configured. It is a single
// directory shared across every repository this builder processes:
// BuildKit's cache entries are content-addressed, so sharing one directory
// across unrelated builds is safe and lets a layer built for one repo speed
// up another's identical layer — mirroring BuildKit/Buildx's own default
// local-cache behavior.
func DefaultCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("buildkit: resolving user cache directory: %w", err)
	}
	return filepath.Join(base, "odoo-builder", "buildkit-cache"), nil
}
