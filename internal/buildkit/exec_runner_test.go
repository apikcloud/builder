// SPDX-License-Identifier: MIT
package buildkit

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecRunner_Build_RejectsUnknownOutputType(t *testing.T) {
	t.Setenv("BUILDKIT_HOST", "unix:///tmp/nonexistent.sock")

	_, err := execRunner{}.Build(context.Background(), BuildOptions{OutputType: "bogus"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "oci")
	assert.Contains(t, err.Error(), "docker")
	assert.Contains(t, err.Error(), "registry")
}

func TestExecRunner_Build_RegistryRequiresImage(t *testing.T) {
	t.Setenv("BUILDKIT_HOST", "unix:///tmp/nonexistent.sock")

	_, err := execRunner{}.Build(context.Background(), BuildOptions{OutputType: "registry", Image: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires Image")
}

func TestExecRunner_Build_DockerRequiresImage(t *testing.T) {
	t.Setenv("BUILDKIT_HOST", "unix:///tmp/nonexistent.sock")

	_, err := execRunner{}.Build(context.Background(), BuildOptions{OutputType: "docker", Image: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires Image")
}

func TestExecRunner_Build_BuildctlStdoutRoutedToStderr(t *testing.T) {
	dir := t.TempDir()
	buildctl := filepath.Join(dir, "buildctl")
	require.NoError(t, os.WriteFile(buildctl, []byte("#!/bin/sh\necho on-stdout\n"), 0o755))

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("BUILDKIT_HOST", "unix:///tmp/nonexistent.sock")

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })

	_, buildErr := execRunner{}.Build(context.Background(), BuildOptions{OutputType: "oci", OutputPath: filepath.Join(dir, "out.tar")})

	require.NoError(t, w.Close())
	captured, readErr := io.ReadAll(r)
	require.NoError(t, readErr)

	require.NoError(t, buildErr)
	assert.NotContains(t, string(captured), "on-stdout")
}
