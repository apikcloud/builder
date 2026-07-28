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

	"github.com/apikcloud/odoo-builder/internal/buildkit"
	"github.com/apikcloud/odoo-builder/internal/engine"
	"github.com/apikcloud/odoo-builder/internal/launcher"
	"github.com/apikcloud/odoo-builder/internal/workspace"
)

func stubLauncherNeededFalse(t *testing.T) {
	t.Helper()
	old := launcherNeeded
	launcherNeeded = func() bool { return false }
	t.Cleanup(func() { launcherNeeded = old })
}

type fakeRunner struct{}

func (fakeRunner) Build(ctx context.Context, opts buildkit.BuildOptions) (buildkit.BuildOutput, error) {
	return buildkit.BuildOutput{OutputPath: "/fake/image.oci.tar"}, nil
}

func TestBuildCmd_Success(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	oldNewEngine := newEngine
	newEngine = func() *engine.Engine { return &engine.Engine{Runner: fakeRunner{}} }
	t.Cleanup(func() { newEngine = oldNewEngine })
	stubLauncherNeededFalse(t)

	cmd := newBuildCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)

	require.NoError(t, cmd.RunE(cmd, nil))
	assert.Contains(t, out.String(), "built image at /fake/image.oci.tar")
	assert.Contains(t, out.String(), "(1 addon(s))")
}

type fakeRegistryRunner struct{}

func (fakeRegistryRunner) Build(ctx context.Context, opts buildkit.BuildOptions) (buildkit.BuildOutput, error) {
	return buildkit.BuildOutput{ImageRef: "registry.example.com/customer/odoo:latest"}, nil
}

func TestBuildCmd_Success_RegistryPush(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "odoo-builder.yaml"),
		[]byte("image:\n  name: registry.example.com/customer/odoo\n"), 0o644))

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	oldNewEngine := newEngine
	newEngine = func() *engine.Engine { return &engine.Engine{Runner: fakeRegistryRunner{}} }
	t.Cleanup(func() { newEngine = oldNewEngine })
	stubLauncherNeededFalse(t)

	cmd := newBuildCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)

	require.NoError(t, cmd.RunE(cmd, nil))
	assert.Contains(t, out.String(), "pushed image registry.example.com/customer/odoo:latest")
	assert.Contains(t, out.String(), "(1 addon(s))")
}

func TestBuildCmd_ValidationFailure_ReturnsError(t *testing.T) {
	repoDir := t.TempDir()

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })
	stubLauncherNeededFalse(t)

	cmd := newBuildCmd()
	cmd.SetContext(context.Background())

	err = cmd.RunE(cmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

type recordingLauncherRun struct {
	called    bool
	runtime   launcher.Runtime
	workspace string
	args      []string
	err       error
}

func (r *recordingLauncherRun) run(ctx context.Context, runtime launcher.Runtime, workspace string, args []string, stdout, stderr io.Writer) error {
	r.called = true
	r.runtime = runtime
	r.workspace = workspace
	r.args = args
	return r.err
}

func TestBuildCmd_UsesLauncher_WhenBuildKitMissing(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	engineCalled := false
	oldNewEngine := newEngine
	newEngine = func() *engine.Engine {
		engineCalled = true
		return &engine.Engine{Runner: fakeRunner{}}
	}
	t.Cleanup(func() { newEngine = oldNewEngine })

	oldLauncherNeeded := launcherNeeded
	launcherNeeded = func() bool { return true }
	t.Cleanup(func() { launcherNeeded = oldLauncherNeeded })

	oldDetectRuntime := launcherDetectRuntime
	launcherDetectRuntime = func() (launcher.Runtime, error) { return launcher.Docker, nil }
	t.Cleanup(func() { launcherDetectRuntime = oldDetectRuntime })

	rec := &recordingLauncherRun{}
	oldLauncherRun := launcherRun
	launcherRun = rec.run
	t.Cleanup(func() { launcherRun = oldLauncherRun })

	cmd := newBuildCmd()
	require.NoError(t, cmd.RunE(cmd, nil))

	assert.True(t, rec.called)
	assert.Equal(t, launcher.Docker, rec.runtime)
	assert.Equal(t, []string{"build"}, rec.args)
	assert.False(t, engineCalled)
}

func TestBuildCmd_LauncherDetectRuntimeError_ReturnsError(t *testing.T) {
	repoDir := t.TempDir()

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	oldLauncherNeeded := launcherNeeded
	launcherNeeded = func() bool { return true }
	t.Cleanup(func() { launcherNeeded = oldLauncherNeeded })

	wantErr := fmt.Errorf("launcher: neither docker nor podman found on PATH")
	oldDetectRuntime := launcherDetectRuntime
	launcherDetectRuntime = func() (launcher.Runtime, error) { return "", wantErr }
	t.Cleanup(func() { launcherDetectRuntime = oldDetectRuntime })

	rec := &recordingLauncherRun{}
	oldLauncherRun := launcherRun
	launcherRun = rec.run
	t.Cleanup(func() { launcherRun = oldLauncherRun })

	cmd := newBuildCmd()
	err = cmd.RunE(cmd, nil)
	require.Error(t, err)
	assert.Equal(t, wantErr, err)
	assert.False(t, rec.called)
}

func TestBuildCmd_LauncherRunError_ReturnsError(t *testing.T) {
	repoDir := t.TempDir()

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	oldLauncherNeeded := launcherNeeded
	launcherNeeded = func() bool { return true }
	t.Cleanup(func() { launcherNeeded = oldLauncherNeeded })

	oldDetectRuntime := launcherDetectRuntime
	launcherDetectRuntime = func() (launcher.Runtime, error) { return launcher.Docker, nil }
	t.Cleanup(func() { launcherDetectRuntime = oldDetectRuntime })

	rec := &recordingLauncherRun{err: fmt.Errorf("container exited 1")}
	oldLauncherRun := launcherRun
	launcherRun = rec.run
	t.Cleanup(func() { launcherRun = oldLauncherRun })

	cmd := newBuildCmd()
	err = cmd.RunE(cmd, nil)
	require.Error(t, err)
	assert.Equal(t, rec.err, err)
}

func TestBuildCmd_ModeEngine_SkipsLauncher_EvenWhenBuildKitMissing(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	engineCalled := false
	oldNewEngine := newEngine
	newEngine = func() *engine.Engine {
		engineCalled = true
		return &engine.Engine{Runner: fakeRunner{}}
	}
	t.Cleanup(func() { newEngine = oldNewEngine })

	oldLauncherNeeded := launcherNeeded
	launcherNeeded = func() bool { return true }
	t.Cleanup(func() { launcherNeeded = oldLauncherNeeded })

	rec := &recordingLauncherRun{}
	oldLauncherRun := launcherRun
	launcherRun = rec.run
	t.Cleanup(func() { launcherRun = oldLauncherRun })

	cmd := newBuildCmd()
	require.NoError(t, cmd.Flags().Set("mode", "engine"))

	require.NoError(t, cmd.RunE(cmd, nil))
	assert.True(t, engineCalled)
	assert.False(t, rec.called)
}

func TestBuildCmd_ModeEngine_RootlessError_DoesNotFallBack(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	oldNewEngine := newEngine
	newEngine = func() *engine.Engine { return &engine.Engine{Runner: rootlessFailingRunner{}} }
	t.Cleanup(func() { newEngine = oldNewEngine })

	rec := &recordingLauncherRun{}
	oldLauncherRun := launcherRun
	launcherRun = rec.run
	t.Cleanup(func() { launcherRun = oldLauncherRun })

	cmd := newBuildCmd()
	require.NoError(t, cmd.Flags().Set("mode", "engine"))

	err = cmd.RunE(cmd, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, buildkit.ErrRootlessRequired)
	assert.False(t, rec.called)
}

func TestBuildCmd_ModeLauncher_ForcesContainer_EvenWhenBuildKitPresent(t *testing.T) {
	repoDir := t.TempDir()

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	engineCalled := false
	oldNewEngine := newEngine
	newEngine = func() *engine.Engine {
		engineCalled = true
		return &engine.Engine{Runner: fakeRunner{}}
	}
	t.Cleanup(func() { newEngine = oldNewEngine })

	oldLauncherNeeded := launcherNeeded
	launcherNeeded = func() bool { return false }
	t.Cleanup(func() { launcherNeeded = oldLauncherNeeded })

	oldDetectRuntime := launcherDetectRuntime
	launcherDetectRuntime = func() (launcher.Runtime, error) { return launcher.Docker, nil }
	t.Cleanup(func() { launcherDetectRuntime = oldDetectRuntime })

	rec := &recordingLauncherRun{}
	oldLauncherRun := launcherRun
	launcherRun = rec.run
	t.Cleanup(func() { launcherRun = oldLauncherRun })

	cmd := newBuildCmd()
	require.NoError(t, cmd.Flags().Set("mode", "launcher"))

	require.NoError(t, cmd.RunE(cmd, nil))
	assert.True(t, rec.called)
	assert.False(t, engineCalled)
}

func TestBuildCmd_InvalidMode_ReturnsError(t *testing.T) {
	repoDir := t.TempDir()

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	engineCalled := false
	oldNewEngine := newEngine
	newEngine = func() *engine.Engine {
		engineCalled = true
		return &engine.Engine{Runner: fakeRunner{}}
	}
	t.Cleanup(func() { newEngine = oldNewEngine })

	rec := &recordingLauncherRun{}
	oldLauncherRun := launcherRun
	launcherRun = rec.run
	t.Cleanup(func() { launcherRun = oldLauncherRun })

	cmd := newBuildCmd()
	require.NoError(t, cmd.Flags().Set("mode", "bogus"))

	err = cmd.RunE(cmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bogus")
	assert.False(t, engineCalled)
	assert.False(t, rec.called)
}

type rootlessFailingRunner struct{}

func (rootlessFailingRunner) Build(ctx context.Context, opts buildkit.BuildOptions) (buildkit.BuildOutput, error) {
	return buildkit.BuildOutput{}, fmt.Errorf("wrapped: %w", buildkit.ErrRootlessRequired)
}

func TestBuildCmd_FallsBackToLauncher_OnRootlessError(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	oldNewEngine := newEngine
	newEngine = func() *engine.Engine { return &engine.Engine{Runner: rootlessFailingRunner{}} }
	t.Cleanup(func() { newEngine = oldNewEngine })
	stubLauncherNeededFalse(t)

	oldDetectRuntime := launcherDetectRuntime
	launcherDetectRuntime = func() (launcher.Runtime, error) { return launcher.Docker, nil }
	t.Cleanup(func() { launcherDetectRuntime = oldDetectRuntime })

	rec := &recordingLauncherRun{}
	oldLauncherRun := launcherRun
	launcherRun = rec.run
	t.Cleanup(func() { launcherRun = oldLauncherRun })

	cmd := newBuildCmd()
	var errOut bytes.Buffer
	cmd.SetErr(&errOut)

	require.NoError(t, cmd.RunE(cmd, nil))
	assert.True(t, rec.called)
	assert.Equal(t, []string{"build"}, rec.args)
	assert.Contains(t, errOut.String(), "retrying inside the odoo-builder container image")
}

// fakeDockerRunner writes a real file at opts.OutputPath, mirroring the real
// execRunner — the archive must exist on disk after Build returns for
// prepare.Prepare's buildDir recreation (which runs before Build) to not
// wipe it out from under the load/removal logic under test.
type fakeDockerRunner struct{}

func (f fakeDockerRunner) Build(ctx context.Context, opts buildkit.BuildOptions) (buildkit.BuildOutput, error) {
	if err := os.WriteFile(opts.OutputPath, []byte("fake"), 0o644); err != nil {
		return buildkit.BuildOutput{}, err
	}
	return buildkit.BuildOutput{OutputPath: opts.OutputPath, ImageRef: opts.Image}, nil
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

func TestBuildCmd_Load_Success(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "odoo-builder.yaml"),
		[]byte("image:\n  name: registry.example.com/customer/odoo\n"), 0o644))

	archivePath := filepath.Join(repoDir, ".build", "image.docker.tar")

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	oldNewEngine := newEngine
	newEngine = func() *engine.Engine { return &engine.Engine{Runner: fakeDockerRunner{}} }
	t.Cleanup(func() { newEngine = oldNewEngine })
	stubLauncherNeededFalse(t)

	oldDetectRuntime := launcherDetectRuntime
	launcherDetectRuntime = func() (launcher.Runtime, error) { return launcher.Docker, nil }
	t.Cleanup(func() { launcherDetectRuntime = oldDetectRuntime })

	rec := &recordingLauncherLoad{}
	oldLauncherLoad := launcherLoad
	launcherLoad = rec.load
	t.Cleanup(func() { launcherLoad = oldLauncherLoad })

	cmd := newBuildCmd()
	require.NoError(t, cmd.Flags().Set("load", "true"))
	var out bytes.Buffer
	cmd.SetOut(&out)

	require.NoError(t, cmd.RunE(cmd, nil))
	assert.Contains(t, out.String(), "loaded image registry.example.com/customer/odoo:latest into docker")
	assert.True(t, rec.called)
	assert.Equal(t, archivePath, rec.archivePath)

	_, statErr := os.Stat(archivePath)
	assert.True(t, os.IsNotExist(statErr))
}

func TestBuildCmd_Load_LauncherModeExplicit_ReturnsError(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	rec := &recordingLauncherRun{}
	oldLauncherRun := launcherRun
	launcherRun = rec.run
	t.Cleanup(func() { launcherRun = oldLauncherRun })

	recLoad := &recordingLauncherLoad{}
	oldLauncherLoad := launcherLoad
	launcherLoad = recLoad.load
	t.Cleanup(func() { launcherLoad = oldLauncherLoad })

	cmd := newBuildCmd()
	require.NoError(t, cmd.Flags().Set("mode", "launcher"))
	require.NoError(t, cmd.Flags().Set("load", "true"))

	err = cmd.RunE(cmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Engine Mode")
	assert.False(t, rec.called)
	assert.False(t, recLoad.called)
}

func TestBuildCmd_Load_AutoWithBuildKitMissing_ReturnsError(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	oldLauncherNeeded := launcherNeeded
	launcherNeeded = func() bool { return true }
	t.Cleanup(func() { launcherNeeded = oldLauncherNeeded })

	rec := &recordingLauncherRun{}
	oldLauncherRun := launcherRun
	launcherRun = rec.run
	t.Cleanup(func() { launcherRun = oldLauncherRun })

	recLoad := &recordingLauncherLoad{}
	oldLauncherLoad := launcherLoad
	launcherLoad = recLoad.load
	t.Cleanup(func() { launcherLoad = oldLauncherLoad })

	cmd := newBuildCmd()
	require.NoError(t, cmd.Flags().Set("load", "true"))

	err = cmd.RunE(cmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Engine Mode")
	assert.False(t, rec.called)
	assert.False(t, recLoad.called)
}

func TestBuildCmd_Load_MissingImageName_ReturnsError(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	oldNewEngine := newEngine
	newEngine = func() *engine.Engine { return &engine.Engine{Runner: fakeRunner{}} }
	t.Cleanup(func() { newEngine = oldNewEngine })
	stubLauncherNeededFalse(t)

	detectCalled := false
	oldDetectRuntime := launcherDetectRuntime
	launcherDetectRuntime = func() (launcher.Runtime, error) {
		detectCalled = true
		return launcher.Docker, nil
	}
	t.Cleanup(func() { launcherDetectRuntime = oldDetectRuntime })

	recLoad := &recordingLauncherLoad{}
	oldLauncherLoad := launcherLoad
	launcherLoad = recLoad.load
	t.Cleanup(func() { launcherLoad = oldLauncherLoad })

	cmd := newBuildCmd()
	require.NoError(t, cmd.Flags().Set("load", "true"))

	err = cmd.RunE(cmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires odoo-builder.yaml's image.name")
	assert.False(t, detectCalled)
	assert.False(t, recLoad.called)
}

func TestBuildCmd_Load_RunnerLoadFails_KeepsArchiveAndWrapsError(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "odoo-builder.yaml"),
		[]byte("image:\n  name: registry.example.com/customer/odoo\n"), 0o644))

	archivePath := filepath.Join(repoDir, ".build", "image.docker.tar")

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	oldNewEngine := newEngine
	newEngine = func() *engine.Engine { return &engine.Engine{Runner: fakeDockerRunner{}} }
	t.Cleanup(func() { newEngine = oldNewEngine })
	stubLauncherNeededFalse(t)

	oldDetectRuntime := launcherDetectRuntime
	launcherDetectRuntime = func() (launcher.Runtime, error) { return launcher.Docker, nil }
	t.Cleanup(func() { launcherDetectRuntime = oldDetectRuntime })

	rec := &recordingLauncherLoad{err: fmt.Errorf("load exited 1")}
	oldLauncherLoad := launcherLoad
	launcherLoad = rec.load
	t.Cleanup(func() { launcherLoad = oldLauncherLoad })

	cmd := newBuildCmd()
	require.NoError(t, cmd.Flags().Set("load", "true"))

	err = cmd.RunE(cmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load exited 1")
	assert.Contains(t, err.Error(), archivePath)

	_, statErr := os.Stat(archivePath)
	assert.NoError(t, statErr)
}

func TestBuildCmd_Load_RootlessError_DoesNotFallBackToLauncher(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "odoo-builder.yaml"),
		[]byte("image:\n  name: registry.example.com/customer/odoo\n"), 0o644))

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	oldNewEngine := newEngine
	newEngine = func() *engine.Engine { return &engine.Engine{Runner: rootlessFailingRunner{}} }
	t.Cleanup(func() { newEngine = oldNewEngine })
	stubLauncherNeededFalse(t)

	rec := &recordingLauncherRun{}
	oldLauncherRun := launcherRun
	launcherRun = rec.run
	t.Cleanup(func() { launcherRun = oldLauncherRun })

	cmd := newBuildCmd()
	require.NoError(t, cmd.Flags().Set("load", "true"))

	err = cmd.RunE(cmd, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, buildkit.ErrRootlessRequired)
	assert.False(t, rec.called)
}

type otherFailingRunner struct{}

func (otherFailingRunner) Build(ctx context.Context, opts buildkit.BuildOptions) (buildkit.BuildOutput, error) {
	return buildkit.BuildOutput{}, fmt.Errorf("some unrelated build error")
}

func TestBuildCmd_OtherEngineError_DoesNotFallBack(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	oldNewEngine := newEngine
	newEngine = func() *engine.Engine { return &engine.Engine{Runner: otherFailingRunner{}} }
	t.Cleanup(func() { newEngine = oldNewEngine })
	stubLauncherNeededFalse(t)

	detectCalled := false
	oldDetectRuntime := launcherDetectRuntime
	launcherDetectRuntime = func() (launcher.Runtime, error) {
		detectCalled = true
		return launcher.Docker, nil
	}
	t.Cleanup(func() { launcherDetectRuntime = oldDetectRuntime })

	rec := &recordingLauncherRun{}
	oldLauncherRun := launcherRun
	launcherRun = rec.run
	t.Cleanup(func() { launcherRun = oldLauncherRun })

	cmd := newBuildCmd()
	err = cmd.RunE(cmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "some unrelated build error")
	assert.False(t, detectCalled)
	assert.False(t, rec.called)
}
