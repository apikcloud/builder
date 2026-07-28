// SPDX-License-Identifier: MIT
package launcher

import (
	"bytes"
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHostIDEnv(t *testing.T) {
	require.Equal(t, []string{"HOST_UID=1000", "HOST_GID=1001"}, hostIDEnv(1000, 1001))
	require.Equal(t, []string{"HOST_GID=1001"}, hostIDEnv(-1, 1001))
	require.Equal(t, []string{"HOST_UID=1000"}, hostIDEnv(1000, -1))
	require.Empty(t, hostIDEnv(-1, -1))
}

func TestRun_Integration(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not found in PATH, skipping launcher integration test")
	}

	t.Setenv(ImageEnvVar, "busybox")

	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), Docker, t.TempDir(), []string{"true"}, &stdout, &stderr)
	require.NoError(t, err)
}
