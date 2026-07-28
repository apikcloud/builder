// SPDX-License-Identifier: MIT
package buildkit

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultCacheDir(t *testing.T) {
	t.Run("resolves under XDG_CACHE_HOME", func(t *testing.T) {
		tmp := t.TempDir()
		t.Setenv("XDG_CACHE_HOME", tmp)

		dir, err := DefaultCacheDir()
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(tmp, "odoo-builder", "buildkit-cache"), dir)
	})

	t.Run("errors when no cache directory is resolvable", func(t *testing.T) {
		t.Setenv("XDG_CACHE_HOME", "")
		t.Setenv("HOME", "")

		_, err := DefaultCacheDir()
		require.Error(t, err)
	})
}
