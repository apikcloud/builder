// SPDX-License-Identifier: MIT
package addons_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apikcloud/odoo-builder/internal/addons"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH")
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
	return string(out)
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "config", "commit.gpgsign", "false")
}

func TestInitSubmodules_NoGitmodules_NoOp(t *testing.T) {
	requireGit(t)
	root := t.TempDir()

	err := addons.InitSubmodules(root, false)
	require.NoError(t, err)
}

func TestInitSubmodules_ChecksOutSubmoduleContent(t *testing.T) {
	requireGit(t)
	t.Setenv("GIT_ALLOW_PROTOCOL", "file")

	sourceRepo := t.TempDir()
	initGitRepo(t, sourceRepo)
	require.NoError(t, os.WriteFile(filepath.Join(sourceRepo, "MARKER.txt"), []byte("addon content\n"), 0o644))
	runGit(t, sourceRepo, "add", "-A")
	runGit(t, sourceRepo, "commit", "-q", "-m", "add marker")

	superRepo := t.TempDir()
	initGitRepo(t, superRepo)
	runGit(t, superRepo, "submodule", "add", "-q", sourceRepo, "addons/sub_addon")
	runGit(t, superRepo, "commit", "-q", "-m", "add submodule")

	clonedRepo := filepath.Join(t.TempDir(), "clone")
	runGit(t, "", "clone", "-q", superRepo, clonedRepo)

	markerPath := filepath.Join(clonedRepo, "addons", "sub_addon", "MARKER.txt")
	_, err := os.Stat(markerPath)
	require.True(t, os.IsNotExist(err), "submodule content must not be checked out before Init")

	require.NoError(t, addons.InitSubmodules(clonedRepo, false))

	content, err := os.ReadFile(markerPath)
	require.NoError(t, err)
	assert.Equal(t, "addon content\n", string(content))
}

func TestInitSubmodules_Recursive(t *testing.T) {
	requireGit(t)
	t.Setenv("GIT_ALLOW_PROTOCOL", "file")

	grandchildRepo := t.TempDir()
	initGitRepo(t, grandchildRepo)
	require.NoError(t, os.WriteFile(filepath.Join(grandchildRepo, "GRANDCHILD.txt"), []byte("grandchild\n"), 0o644))
	runGit(t, grandchildRepo, "add", "-A")
	runGit(t, grandchildRepo, "commit", "-q", "-m", "add grandchild marker")

	middleRepo := t.TempDir()
	initGitRepo(t, middleRepo)
	require.NoError(t, os.WriteFile(filepath.Join(middleRepo, "MIDDLE.txt"), []byte("middle\n"), 0o644))
	runGit(t, middleRepo, "add", "-A")
	runGit(t, middleRepo, "commit", "-q", "-m", "add middle marker")
	runGit(t, middleRepo, "submodule", "add", "-q", grandchildRepo, "nested_sub")
	runGit(t, middleRepo, "commit", "-q", "-m", "add nested submodule")

	superRepo := t.TempDir()
	initGitRepo(t, superRepo)
	runGit(t, superRepo, "submodule", "add", "-q", middleRepo, "addons/middle_submodule")
	runGit(t, superRepo, "commit", "-q", "-m", "add middle submodule")

	t.Run("non-recursive leaves nested submodule uninitialized", func(t *testing.T) {
		clonedRepo := filepath.Join(t.TempDir(), "clone")
		runGit(t, "", "clone", "-q", superRepo, clonedRepo)

		require.NoError(t, addons.InitSubmodules(clonedRepo, false))

		_, err := os.ReadFile(filepath.Join(clonedRepo, "addons", "middle_submodule", "MIDDLE.txt"))
		require.NoError(t, err)

		_, err = os.Stat(filepath.Join(clonedRepo, "addons", "middle_submodule", "nested_sub", "GRANDCHILD.txt"))
		assert.True(t, os.IsNotExist(err), "nested submodule must stay uninitialized without --recursive")
	})

	t.Run("recursive initializes nested submodule", func(t *testing.T) {
		clonedRepo := filepath.Join(t.TempDir(), "clone")
		runGit(t, "", "clone", "-q", superRepo, clonedRepo)

		require.NoError(t, addons.InitSubmodules(clonedRepo, true))

		content, err := os.ReadFile(filepath.Join(clonedRepo, "addons", "middle_submodule", "nested_sub", "GRANDCHILD.txt"))
		require.NoError(t, err)
		assert.Equal(t, "grandchild\n", string(content))
	})
}
