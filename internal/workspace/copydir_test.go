package workspace_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apikcloud/odoo-builder/internal/workspace"
)

func TestCopyDir_NestedFilesAndSubdirs(t *testing.T) {
	src := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(src, "top.txt"), []byte("top\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(src, "nested"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "nested", "inner.txt"), []byte("inner\n"), 0o755))

	dst := filepath.Join(t.TempDir(), "out")
	require.NoError(t, workspace.CopyDir(src, dst))

	top, err := os.ReadFile(filepath.Join(dst, "top.txt"))
	require.NoError(t, err)
	assert.Equal(t, "top\n", string(top))

	inner, err := os.ReadFile(filepath.Join(dst, "nested", "inner.txt"))
	require.NoError(t, err)
	assert.Equal(t, "inner\n", string(inner))

	innerInfo, err := os.Stat(filepath.Join(dst, "nested", "inner.txt"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), innerInfo.Mode().Perm())
}

func TestCopyDir_SkipsGitMetadata(t *testing.T) {
	src := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(src, "__manifest__.py"), []byte("{}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(src, ".git"), []byte("gitdir: ../.git/modules/addon\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(src, "sub", ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "sub", ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(src, ".gitmodules"), []byte("[submodule]\n"), 0o644))

	dst := filepath.Join(t.TempDir(), "out")
	require.NoError(t, workspace.CopyDir(src, dst))

	_, err := os.Stat(filepath.Join(dst, "__manifest__.py"))
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(dst, ".git"))
	assert.True(t, os.IsNotExist(err), ".git must not be copied")

	_, err = os.Stat(filepath.Join(dst, ".gitmodules"))
	assert.True(t, os.IsNotExist(err), ".gitmodules must not be copied")

	_, err = os.Stat(filepath.Join(dst, "sub", ".git"))
	assert.True(t, os.IsNotExist(err), "nested .git must not be copied")
}

func TestCopyDir_SymlinkPassthrough(t *testing.T) {
	src := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(src, "sub"), 0o755))
	require.NoError(t, os.Symlink("some/unresolved/target", filepath.Join(src, "sub", "dangling_link")))

	dst := filepath.Join(t.TempDir(), "out")
	require.NoError(t, workspace.CopyDir(src, dst))

	linkPath := filepath.Join(dst, "sub", "dangling_link")
	info, err := os.Lstat(linkPath)
	require.NoError(t, err)
	assert.True(t, info.Mode()&os.ModeSymlink != 0)

	target, err := os.Readlink(linkPath)
	require.NoError(t, err)
	assert.Equal(t, "some/unresolved/target", target)
}

func TestCopyDir_Deterministic(t *testing.T) {
	src := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(src, "a", "b"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "a", "b", "file.txt"), []byte("content\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(src, "root.txt"), []byte("root\n"), 0o644))

	root1, err := workspace.New(filepath.Join(t.TempDir(), ".build"))
	require.NoError(t, err)
	root2, err := workspace.New(filepath.Join(t.TempDir(), ".build"))
	require.NoError(t, err)

	require.NoError(t, workspace.CopyDir(src, root1.Root))
	require.NoError(t, workspace.CopyDir(src, root2.Root))

	assert.Equal(t, treeContents(t, root1.Root), treeContents(t, root2.Root))
}

func treeContents(t *testing.T, root string) map[string]string {
	t.Helper()
	contents := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		contents[rel] = string(data)
		return nil
	})
	require.NoError(t, err)
	return contents
}
