// SPDX-License-Identifier: MIT
package buildkit_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apikcloud/builder/internal/buildkit"
	"github.com/apikcloud/builder/internal/config"
	"github.com/apikcloud/builder/internal/prepare"
	"github.com/apikcloud/builder/internal/workspace"
)

func TestExecRunner_Build_Integration(t *testing.T) {
	if _, err := exec.LookPath("buildctl"); err != nil {
		t.Skip("buildctl/buildkitd not found in PATH, skipping BuildKit integration test")
	}
	if _, err := exec.LookPath("buildkitd"); err != nil {
		t.Skip("buildctl/buildkitd not found in PATH, skipping BuildKit integration test")
	}

	repoDir := t.TempDir()
	require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))

	buildDir := filepath.Join(repoDir, ".build")
	_, err := prepare.Prepare(repoDir, buildDir, config.Default())
	require.NoError(t, err)

	outputPath := filepath.Join(buildDir, "image.oci.tar")
	out, err := buildkit.NewRunner().Build(context.Background(), buildkit.BuildOptions{
		ContextDir:     buildDir,
		DockerfilePath: filepath.Join(buildDir, "Dockerfile"),
		OutputType:     "oci",
		OutputPath:     outputPath,
	})
	require.NoError(t, err)

	info, err := os.Stat(out.OutputPath)
	require.NoError(t, err)
	require.NotZero(t, info.Size())
}

func TestExecRunner_Build_Integration_LocalCache(t *testing.T) {
	if _, err := exec.LookPath("buildctl"); err != nil {
		t.Skip("buildctl/buildkitd not found in PATH, skipping BuildKit integration test")
	}
	if _, err := exec.LookPath("buildkitd"); err != nil {
		t.Skip("buildctl/buildkitd not found in PATH, skipping BuildKit integration test")
	}

	repoDir := t.TempDir()
	require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))
	buildDir := filepath.Join(repoDir, ".build")
	_, err := prepare.Prepare(repoDir, buildDir, config.Default())
	require.NoError(t, err)

	cacheDir := filepath.Join(t.TempDir(), "cache")
	_, err = buildkit.NewRunner().Build(context.Background(), buildkit.BuildOptions{
		ContextDir:     buildDir,
		DockerfilePath: filepath.Join(buildDir, "Dockerfile"),
		OutputType:     "oci",
		OutputPath:     filepath.Join(buildDir, "image.oci.tar"),
		CacheDir:       cacheDir,
	})
	require.NoError(t, err)

	entries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)
	assert.NotEmpty(t, entries, "local cache export should have written at least one entry")
}

func TestExecRunner_Build_Integration_HostPlatform(t *testing.T) {
	if _, err := exec.LookPath("buildctl"); err != nil {
		t.Skip("buildctl/buildkitd not found in PATH, skipping BuildKit integration test")
	}
	if _, err := exec.LookPath("buildkitd"); err != nil {
		t.Skip("buildctl/buildkitd not found in PATH, skipping BuildKit integration test")
	}

	repoDir := t.TempDir()
	require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))
	buildDir := filepath.Join(repoDir, ".build")
	_, err := prepare.Prepare(repoDir, buildDir, config.Default())
	require.NoError(t, err)

	// Only the host's own platform is used here — proving the --opt
	// platform= flag is plumbed through and accepted by a real buildctl/
	// buildkitd, without requiring QEMU/binfmt emulation for a foreign
	// architecture (out of scope for this milestone; see the plan's "What
	// We're NOT Doing").
	out, err := buildkit.NewRunner().Build(context.Background(), buildkit.BuildOptions{
		ContextDir:     buildDir,
		DockerfilePath: filepath.Join(buildDir, "Dockerfile"),
		OutputType:     "oci",
		OutputPath:     filepath.Join(buildDir, "image.oci.tar"),
		Platforms:      []string{runtime.GOOS + "/" + runtime.GOARCH},
	})
	require.NoError(t, err)

	info, err := os.Stat(out.OutputPath)
	require.NoError(t, err)
	require.NotZero(t, info.Size())
}

func TestExecRunner_Build_Integration_Docker(t *testing.T) {
	if _, err := exec.LookPath("buildctl"); err != nil {
		t.Skip("buildctl/buildkitd not found in PATH, skipping BuildKit integration test")
	}
	if _, err := exec.LookPath("buildkitd"); err != nil {
		t.Skip("buildctl/buildkitd not found in PATH, skipping BuildKit integration test")
	}
	loadRuntime := "docker"
	if _, err := exec.LookPath("docker"); err != nil {
		loadRuntime = "podman"
		if _, err := exec.LookPath("podman"); err != nil {
			t.Skip("neither docker nor podman found in PATH, skipping load integration test")
		}
	}

	repoDir := t.TempDir()
	require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))
	buildDir := filepath.Join(repoDir, ".build")
	_, err := prepare.Prepare(repoDir, buildDir, config.Default())
	require.NoError(t, err)

	ref := "odoo-builder-plan-test:latest"
	outputPath := filepath.Join(buildDir, "image.docker.tar")
	out, err := buildkit.NewRunner().Build(context.Background(), buildkit.BuildOptions{
		ContextDir:     buildDir,
		DockerfilePath: filepath.Join(buildDir, "Dockerfile"),
		OutputType:     "docker",
		OutputPath:     outputPath,
		Image:          ref,
	})
	require.NoError(t, err)
	assert.Equal(t, ref, out.ImageRef)

	info, err := os.Stat(out.OutputPath)
	require.NoError(t, err)
	require.NotZero(t, info.Size())

	// Prove the archive is actually loadable, and clean up afterward.
	require.NoError(t, exec.Command(loadRuntime, "load", "-i", out.OutputPath).Run())
	t.Cleanup(func() { _ = exec.Command(loadRuntime, "rmi", "-f", ref).Run() })
}
