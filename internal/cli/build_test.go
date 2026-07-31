// SPDX-License-Identifier: MIT
package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apikcloud/builder/internal/engine"
	"github.com/apikcloud/builder/internal/launcher"
)

type recordingInvoke struct {
	calls []engine.BuildRequest
	mode  launcher.Mode
	resp  engine.BuildResponse
	err   error
}

func (r *recordingInvoke) invoke(ctx context.Context, mode launcher.Mode, req engine.BuildRequest, stderr io.Writer) (engine.BuildResponse, error) {
	r.calls = append(r.calls, req)
	r.mode = mode
	return r.resp, r.err
}

func stubInvokeEngine(t *testing.T, rec *recordingInvoke) {
	t.Helper()
	old := invokeEngine
	invokeEngine = rec.invoke
	t.Cleanup(func() { invokeEngine = old })
}

func TestBuildCmd_Success_BuildsRequest(t *testing.T) {
	repoDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	rec := &recordingInvoke{resp: engine.BuildResponse{ImagePath: "/fake/image.oci.tar", AddonCount: 1}}
	stubInvokeEngine(t, rec)

	cmd := newBuildCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)

	require.NoError(t, cmd.RunE(cmd, nil))
	assert.Contains(t, out.String(), "built image at /fake/image.oci.tar")
	assert.Contains(t, out.String(), "(1 addon(s))")

	require.Len(t, rec.calls, 1)
	assert.Equal(t, engine.APIVersion, rec.calls[0].APIVersion)
	assert.Equal(t, engine.CommandBuild, rec.calls[0].Command)
	assert.Equal(t, repoDir, rec.calls[0].RepoRoot)
	assert.Equal(t, launcher.ModeAuto, rec.mode)
}

func TestBuildCmd_Success_RegistryPush(t *testing.T) {
	repoDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	rec := &recordingInvoke{resp: engine.BuildResponse{ImageRef: "registry.example.com/customer/odoo:latest", AddonCount: 1}}
	stubInvokeEngine(t, rec)

	cmd := newBuildCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)

	require.NoError(t, cmd.RunE(cmd, nil))
	assert.Contains(t, out.String(), "pushed image registry.example.com/customer/odoo:latest")
	assert.Contains(t, out.String(), "(1 addon(s))")
}

func TestBuildCmd_InvokeError_ReturnsError(t *testing.T) {
	repoDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	rec := &recordingInvoke{err: fmt.Errorf("engine: validation failed: boom")}
	stubInvokeEngine(t, rec)

	cmd := newBuildCmd()
	err = cmd.RunE(cmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestBuildCmd_InvalidMode_ReturnsError(t *testing.T) {
	repoDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	rec := &recordingInvoke{}
	stubInvokeEngine(t, rec)

	cmd := newBuildCmd()
	require.NoError(t, cmd.Flags().Set("mode", "bogus"))

	err = cmd.RunE(cmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bogus")
	assert.Empty(t, rec.calls)
}

func TestBuildCmd_Load_Success(t *testing.T) {
	repoDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	archivePath := filepath.Join(repoDir, "image.docker.tar")
	require.NoError(t, os.WriteFile(archivePath, []byte("fake"), 0o644))

	rec := &recordingInvoke{resp: engine.BuildResponse{
		ImagePath:  archivePath,
		ImageRef:   "registry.example.com/customer/odoo:latest",
		AddonCount: 1,
	}}
	stubInvokeEngine(t, rec)

	oldDetectRuntime := launcherDetectRuntime
	launcherDetectRuntime = func() (launcher.Runtime, error) { return launcher.Docker, nil }
	t.Cleanup(func() { launcherDetectRuntime = oldDetectRuntime })

	loadRec := &recordingLauncherLoad{}
	oldLoad := launcherLoad
	launcherLoad = loadRec.load
	t.Cleanup(func() { launcherLoad = oldLoad })

	cmd := newBuildCmd()
	require.NoError(t, cmd.Flags().Set("mode", "engine")) // skip the auto-mode Needed(CommandBuild) guard, irrelevant here
	require.NoError(t, cmd.Flags().Set("load", "true"))
	var out bytes.Buffer
	cmd.SetOut(&out)

	require.NoError(t, cmd.RunE(cmd, nil))
	assert.Contains(t, out.String(), "loaded image registry.example.com/customer/odoo:latest into docker")
	assert.True(t, loadRec.called)
	assert.Equal(t, archivePath, loadRec.archivePath)

	require.Len(t, rec.calls, 1)
	assert.True(t, rec.calls[0].Load)
	assert.Equal(t, "docker", rec.calls[0].Output.Type)

	_, statErr := os.Stat(archivePath)
	assert.True(t, os.IsNotExist(statErr))
}

func TestBuildCmd_Load_LauncherModeExplicit_ReturnsError(t *testing.T) {
	repoDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	rec := &recordingInvoke{}
	stubInvokeEngine(t, rec)

	cmd := newBuildCmd()
	require.NoError(t, cmd.Flags().Set("mode", "launcher"))
	require.NoError(t, cmd.Flags().Set("load", "true"))

	err = cmd.RunE(cmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Engine Mode")
	assert.Empty(t, rec.calls)
}

func TestBuildCmd_Load_AutoWithNothingOnPath_ReturnsError(t *testing.T) {
	repoDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	// Empty PATH forces launcher.Needed(engine.CommandBuild) to true
	// deterministically (no engine binary/buildctl/buildkitd found),
	// regardless of what's actually installed on the host running this test.
	t.Setenv("PATH", t.TempDir())

	rec := &recordingInvoke{}
	stubInvokeEngine(t, rec)

	cmd := newBuildCmd()
	require.NoError(t, cmd.Flags().Set("load", "true"))

	err = cmd.RunE(cmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Engine Mode")
	assert.Empty(t, rec.calls)
}

func TestBuildCmd_Load_RunnerLoadFails_KeepsArchiveAndWrapsError(t *testing.T) {
	repoDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	archivePath := filepath.Join(repoDir, "image.docker.tar")
	require.NoError(t, os.WriteFile(archivePath, []byte("fake"), 0o644))

	rec := &recordingInvoke{resp: engine.BuildResponse{ImagePath: archivePath, ImageRef: "registry.example.com/customer/odoo:latest"}}
	stubInvokeEngine(t, rec)

	oldDetectRuntime := launcherDetectRuntime
	launcherDetectRuntime = func() (launcher.Runtime, error) { return launcher.Docker, nil }
	t.Cleanup(func() { launcherDetectRuntime = oldDetectRuntime })

	loadRec := &recordingLauncherLoad{err: fmt.Errorf("load exited 1")}
	oldLoad := launcherLoad
	launcherLoad = loadRec.load
	t.Cleanup(func() { launcherLoad = oldLoad })

	cmd := newBuildCmd()
	require.NoError(t, cmd.Flags().Set("mode", "engine")) // skip the auto-mode Needed(CommandBuild) guard, irrelevant here
	require.NoError(t, cmd.Flags().Set("load", "true"))

	err = cmd.RunE(cmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load exited 1")
	assert.Contains(t, err.Error(), archivePath)

	_, statErr := os.Stat(archivePath)
	assert.NoError(t, statErr)
}

type recordingLauncherLoad struct {
	called      bool
	runtime     launcher.Runtime
	archivePath string
	err         error
}

func (r *recordingLauncherLoad) load(ctx context.Context, runtime launcher.Runtime, archivePath string, stdout, stderr io.Writer) error {
	r.called = true
	r.runtime = runtime
	r.archivePath = archivePath
	return r.err
}
