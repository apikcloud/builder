package enterprise_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apikcloud/odoo-builder/internal/enterprise"
)

func TestClone_EmptyVersion_ReturnsError(t *testing.T) {
	_, _, err := enterprise.Clone("https://unreachable.invalid/enterprise.git", "", "token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "base.version")
}

func TestClone_EmptyToken_ReturnsError(t *testing.T) {
	_, _, err := enterprise.Clone("https://unreachable.invalid/enterprise.git", "18.0", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), enterprise.TokenEnvVar)
}

func TestClone_ValidToken_ChecksOutBranch(t *testing.T) {
	repoURL := startAuthedGitServer(t, "18.0", "MARKER.txt", "enterprise content\n", "correct-token")

	dir, cleanup, err := enterprise.Clone(repoURL, "18.0", "correct-token")
	require.NoError(t, err)
	defer cleanup()

	content, err := os.ReadFile(filepath.Join(dir, "MARKER.txt"))
	require.NoError(t, err)
	assert.Equal(t, "enterprise content\n", string(content))
}

func TestClone_WrongToken_ReturnsError(t *testing.T) {
	repoURL := startAuthedGitServer(t, "18.0", "MARKER.txt", "enterprise content\n", "correct-token")

	_, _, err := enterprise.Clone(repoURL, "18.0", "wrong-token")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "correct-token", "the real credential must never appear in an error message")
}

func TestClone_CleanupRemovesClone(t *testing.T) {
	repoURL := startAuthedGitServer(t, "18.0", "MARKER.txt", "content\n", "tok")

	dir, cleanup, err := enterprise.Clone(repoURL, "18.0", "tok")
	require.NoError(t, err)
	cleanup()

	_, err = os.Stat(dir)
	assert.True(t, os.IsNotExist(err))
}
