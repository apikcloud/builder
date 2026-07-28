// SPDX-License-Identifier: MIT
package buildkit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildkitdRunDirBase(t *testing.T) {
	t.Run("env var set", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("ODOO_BUILDER_BUILDKITD_ROOT", dir)

		got, err := buildkitdRunDirBase()
		require.NoError(t, err)
		assert.Equal(t, dir, got)
	})

	t.Run("env var unset falls back to MkdirTemp", func(t *testing.T) {
		t.Setenv("ODOO_BUILDER_BUILDKITD_ROOT", "")

		first, err := buildkitdRunDirBase()
		require.NoError(t, err)
		_, statErr := os.Stat(first)
		require.NoError(t, statErr)

		second, err := buildkitdRunDirBase()
		require.NoError(t, err)
		assert.NotEqual(t, first, second)

		os.RemoveAll(first)
		os.RemoveAll(second)
	})
}

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

// installFakeBuildkitdCapturingArgv writes a fake buildkitd shell script
// that appends its own argv to argvFile (one arg per line) and exits 1,
// then prepends its directory to PATH — same technique as
// installFakeBuildkitd, but capturing argv instead of a fixed stderr
// message.
func installFakeBuildkitdCapturingArgv(t *testing.T, argvFile string) {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "buildkitd")
	content := "#!/bin/sh\nfor a in \"$@\"; do echo \"$a\" >> '" + argvFile + "'; done\nexit 1\n"
	require.NoError(t, os.WriteFile(script, []byte(content), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestEnsureDaemon_UsesBuildkitdRootEnvVar(t *testing.T) {
	envDir := t.TempDir()
	t.Setenv("ODOO_BUILDER_BUILDKITD_ROOT", envDir)

	argvFile := filepath.Join(t.TempDir(), "argv.txt")
	installFakeBuildkitdCapturingArgv(t, argvFile)

	_, _, err := ensureDaemon(context.Background())
	require.Error(t, err)

	argvBytes, readErr := os.ReadFile(argvFile)
	require.NoError(t, readErr)
	argv := strings.Split(strings.TrimRight(string(argvBytes), "\n"), "\n")

	assert.Contains(t, argv, "--root")
	wantRoot := filepath.Join(envDir, "root")
	found := false
	for i, a := range argv {
		if a == "--root" && i+1 < len(argv) && argv[i+1] == wantRoot {
			found = true
		}
	}
	assert.True(t, found, "expected --root %s in argv %v", wantRoot, argv)
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
