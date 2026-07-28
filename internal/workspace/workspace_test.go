// SPDX-License-Identifier: MIT
package workspace_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apikcloud/odoo-builder/internal/workspace"
)

func TestNew_CreatesEmptyRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "build")

	ws, err := workspace.New(root)
	require.NoError(t, err)
	assert.Equal(t, root, ws.Root)

	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestNew_WipesExistingContents(t *testing.T) {
	root := filepath.Join(t.TempDir(), "build")
	require.NoError(t, os.MkdirAll(root, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "stale.txt"), []byte("old"), 0o644))

	_, err := workspace.New(root)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(root, "stale.txt"))
	assert.True(t, os.IsNotExist(err), "stale content must be wiped")
}

func TestCopyFile_PreservesContentAndMode(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	src := filepath.Join(srcDir, "requirements.txt")
	require.NoError(t, os.WriteFile(src, []byte("odoo-stubs==1.0\n"), 0o600))

	dst := filepath.Join(dstDir, "nested", "requirements.txt")
	require.NoError(t, workspace.CopyFile(src, dst))

	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "odoo-stubs==1.0\n", string(got))

	srcInfo, err := os.Stat(src)
	require.NoError(t, err)
	dstInfo, err := os.Stat(dst)
	require.NoError(t, err)
	assert.Equal(t, srcInfo.Mode().Perm(), dstInfo.Mode().Perm())
}

func TestEnsureDir_CreatesNestedPath(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "addons", "nested")

	require.NoError(t, workspace.EnsureDir(path))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}
