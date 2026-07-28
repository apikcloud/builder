package buildkit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureDaemon_UsesExistingHost(t *testing.T) {
	t.Setenv("BUILDKIT_HOST", "unix:///tmp/existing.sock")

	addr, cleanup, err := ensureDaemon(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "unix:///tmp/existing.sock", addr)
	require.NotNil(t, cleanup)
	cleanup()
}

// installFakeBuildkitd writes a fake buildkitd shell script that echoes
// stderrMsg to stderr and exits 1, then prepends its directory to PATH so
// exec.CommandContext("buildkitd", ...) resolves to it instead of any real
// buildkitd binary.
func installFakeBuildkitd(t *testing.T, stderrMsg string) {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "buildkitd")
	content := "#!/bin/sh\necho '" + stderrMsg + "' >&2\nexit 1\n"
	require.NoError(t, os.WriteFile(script, []byte(content), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestEnsureDaemon_RootlessError_ReturnsErrRootlessRequired(t *testing.T) {
	installFakeBuildkitd(t, "buildkitd: rootless mode requires to be executed as the mapped root in a user namespace; you may use RootlessKit for setting up the namespace")

	start := time.Now()
	_, _, err := ensureDaemon(context.Background())
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRootlessRequired))
	require.Less(t, elapsed, 2*time.Second)
}

func TestEnsureDaemon_OtherStartupFailure_ReturnsGenericError(t *testing.T) {
	installFakeBuildkitd(t, "boom")

	_, _, err := ensureDaemon(context.Background())

	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrRootlessRequired))
	assert.Contains(t, err.Error(), "boom")
}
