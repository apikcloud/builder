// SPDX-License-Identifier: MIT
package buildkit

import (
	"context"
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
