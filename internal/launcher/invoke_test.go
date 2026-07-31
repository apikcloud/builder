// SPDX-License-Identifier: MIT
package launcher

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apikcloud/builder/internal/engine"
)

// fakeEngineDir holds the directory containing a compiled testdata/fakeengine
// binary named odoo-builder-engine, built once in TestMain and shared (via
// PATH prepending) across every test in this package that needs a fake
// engine invokeLocal can exec.
var fakeEngineDir string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "odoo-builder-fakeengine-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "invoke_test: creating temp dir:", err)
		os.Exit(1)
	}

	cmd := exec.Command("go", "build", "-o", filepath.Join(dir, "odoo-builder-engine"), "./testdata/fakeengine")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "invoke_test: building fakeengine:", err, out.String())
		os.RemoveAll(dir)
		os.Exit(1)
	}

	fakeEngineDir = dir
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func withFakeEngineOnPath(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", fakeEngineDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestInvokeLocal_Success(t *testing.T) {
	withFakeEngineOnPath(t)

	resp, err := invokeLocal(context.Background(), engine.BuildRequest{RepoRoot: "ok"}, os.Stderr)
	require.NoError(t, err)
	assert.Equal(t, "/fake/build", resp.BuildDir)
	assert.Equal(t, 3, resp.AddonCount)
}

func TestInvokeLocal_NonJSONStdout_SurfacesRunErr(t *testing.T) {
	withFakeEngineOnPath(t)

	_, err := invokeLocal(context.Background(), engine.BuildRequest{RepoRoot: "BADOUTPUT"}, os.Stderr)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "decoding engine response")
}

func TestInvokeLocal_NonJSONStdout_ExitZero_IncludesSnippet(t *testing.T) {
	withFakeEngineOnPath(t)

	_, err := invokeLocal(context.Background(), engine.BuildRequest{RepoRoot: "GARBAGE_EXIT0"}, os.Stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decoding engine response")
	assert.Contains(t, err.Error(), "pulling image progress line")
}

func TestInvokeLocal_RespError_SurfacesAsGoError(t *testing.T) {
	withFakeEngineOnPath(t)

	resp, err := invokeLocal(context.Background(), engine.BuildRequest{RepoRoot: "FAIL"}, os.Stderr)
	require.Error(t, err)
	assert.Equal(t, "boom", err.Error())
	assert.Equal(t, engine.ErrorCodeBuildFailed, resp.ErrorCode)
}

func TestInvoke_AutoMode_RetriesContainer_OnRootlessRequired(t *testing.T) {
	withFakeEngineOnPath(t)
	// Force DetectRuntime to fail deterministically, offline, so the retry
	// attempt itself never spawns a real container — this test only cares
	// that a retry was attempted at all, keyed off resp.ErrorCode.
	stubLookPath(t, map[string]bool{EngineBinary: true, "docker": false, "podman": false})

	var stderr bytes.Buffer
	_, err := Invoke(context.Background(), ModeAuto, engine.BuildRequest{RepoRoot: "ROOTLESS"}, &stderr)

	require.Error(t, err)
	assert.Contains(t, stderr.String(), "retrying inside the odoo-builder container image")
}

func TestInvoke_AutoMode_Load_DoesNotRetryContainer_OnRootlessRequired(t *testing.T) {
	withFakeEngineOnPath(t)
	// DetectRuntime is left free to fail/succeed for real — the point of
	// this test is that it's never even called, since --load must not
	// retry into the container.
	stubLookPath(t, map[string]bool{EngineBinary: true, "docker": false, "podman": false})

	var stderr bytes.Buffer
	_, err := Invoke(context.Background(), ModeAuto, engine.BuildRequest{RepoRoot: "ROOTLESS", Load: true}, &stderr)

	require.Error(t, err)
	assert.Equal(t, "rootless needed", err.Error())
	assert.NotContains(t, stderr.String(), "retrying inside")
}

func TestInvoke_ModeEngine_RequiresEngineBinaryOnPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := Invoke(context.Background(), ModeEngine, engine.BuildRequest{RepoRoot: "ok"}, os.Stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), EngineBinary)
}

func TestHostBuildkitdRootDir(t *testing.T) {
	dir, cleanup := hostBuildkitdRootDir()
	require.NotEmpty(t, dir)
	_, err := os.Stat(dir)
	require.NoError(t, err)

	cleanup()

	_, err = os.Stat(dir)
	assert.True(t, os.IsNotExist(err))
}

func TestInvoke_ModeEngine_DoesNotRetryOnRootless(t *testing.T) {
	withFakeEngineOnPath(t)

	var stderr bytes.Buffer
	_, err := Invoke(context.Background(), ModeEngine, engine.BuildRequest{RepoRoot: "ROOTLESS"}, &stderr)

	require.Error(t, err)
	assert.NotContains(t, stderr.String(), "retrying inside")
}
