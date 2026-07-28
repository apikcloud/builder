package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apikcloud/odoo-builder/internal/buildkit"
	"github.com/apikcloud/odoo-builder/internal/workspace"
)

func TestEngine_Validate(t *testing.T) {
	t.Run("testdata/simple has no errors", func(t *testing.T) {
		repoDir := t.TempDir()
		require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))

		e := &Engine{}
		errs := e.Validate(BuildRequest{RepoRoot: repoDir})
		assert.Empty(t, errs)
	})

	t.Run("empty repo has errors", func(t *testing.T) {
		e := &Engine{}
		errs := e.Validate(BuildRequest{RepoRoot: t.TempDir()})
		assert.NotEmpty(t, errs)
	})
}

func TestEngine_Prepare(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))

	e := &Engine{}
	result, err := e.Prepare(BuildRequest{RepoRoot: repoDir})
	require.NoError(t, err)

	assert.Equal(t, 1, result.AddonCount)
	assert.Equal(t, filepath.Join(repoDir, ".build"), result.BuildDir)
	_, err = os.Stat(filepath.Join(result.BuildDir, "Dockerfile"))
	assert.NoError(t, err)
}

func TestEngine_Inspect(t *testing.T) {
	t.Run("OCI default, no odoo-builder.yaml", func(t *testing.T) {
		repoDir := t.TempDir()
		require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))

		e := &Engine{}
		req, err := e.Inspect(BuildRequest{RepoRoot: repoDir})
		require.NoError(t, err)

		buildDir := filepath.Join(repoDir, ".build")
		assert.Equal(t, "oci", req.Output.Type)
		assert.Equal(t, filepath.Join(buildDir, "image.oci.tar"), req.Output.Path)

		_, err = os.Stat(buildDir)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("registry convention via odoo-builder.yaml image.name", func(t *testing.T) {
		repoDir := t.TempDir()
		require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, "odoo-builder.yaml"),
			[]byte("image:\n  name: registry.example.com/customer/odoo\n"), 0o644))

		e := &Engine{}
		req, err := e.Inspect(BuildRequest{RepoRoot: repoDir})
		require.NoError(t, err)

		assert.Equal(t, "registry", req.Output.Type)
		assert.Equal(t, "registry.example.com/customer/odoo:latest", req.Output.Image)
	})

	t.Run("malformed image.name surfaces the same error as Build", func(t *testing.T) {
		repoDir := t.TempDir()
		require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, "odoo-builder.yaml"),
			[]byte("image:\n  name: \"app:v1\"\n"), 0o644))

		e := &Engine{}
		_, err := e.Inspect(BuildRequest{RepoRoot: repoDir})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "registry:")
	})
}

type fakeRunner struct {
	calls int
	opts  buildkit.BuildOptions
	out   buildkit.BuildOutput
	err   error
}

func (f *fakeRunner) Build(ctx context.Context, opts buildkit.BuildOptions) (buildkit.BuildOutput, error) {
	f.calls++
	f.opts = opts
	return f.out, f.err
}

func TestEngine_Build(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		repoDir := t.TempDir()
		require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))

		runner := &fakeRunner{out: buildkit.BuildOutput{OutputPath: "/fake/image.oci.tar"}}
		e := &Engine{Runner: runner}

		result, err := e.Build(context.Background(), BuildRequest{RepoRoot: repoDir})
		require.NoError(t, err)

		buildDir := filepath.Join(repoDir, ".build")
		assert.Equal(t, 1, runner.calls)
		assert.Equal(t, buildDir, runner.opts.ContextDir)
		assert.Equal(t, filepath.Join(buildDir, "Dockerfile"), runner.opts.DockerfilePath)
		assert.Equal(t, "oci", runner.opts.OutputType)
		assert.Equal(t, filepath.Join(buildDir, "image.oci.tar"), runner.opts.OutputPath)
		assert.Equal(t, "/fake/image.oci.tar", result.ImagePath)
		assert.Equal(t, 1, result.AddonCount)
		assert.Empty(t, result.ImageRef)
	})

	t.Run("registry push via odoo-builder.yaml convention", func(t *testing.T) {
		repoDir := t.TempDir()
		require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, "odoo-builder.yaml"),
			[]byte("image:\n  name: registry.example.com/customer/odoo\n  tag: v1\n"), 0o644))

		runner := &fakeRunner{out: buildkit.BuildOutput{ImageRef: "registry.example.com/customer/odoo:v1"}}
		e := &Engine{Runner: runner}

		result, err := e.Build(context.Background(), BuildRequest{RepoRoot: repoDir})
		require.NoError(t, err)

		assert.Equal(t, "registry", runner.opts.OutputType)
		assert.Equal(t, "registry.example.com/customer/odoo:v1", runner.opts.Image)
		assert.Equal(t, "registry.example.com/customer/odoo:v1", result.ImageRef)
		assert.Empty(t, result.ImagePath)
	})

	t.Run("registry push defaults tag to latest when omitted", func(t *testing.T) {
		repoDir := t.TempDir()
		require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, "odoo-builder.yaml"),
			[]byte("image:\n  name: registry.example.com/customer/odoo\n"), 0o644))

		runner := &fakeRunner{out: buildkit.BuildOutput{ImageRef: "registry.example.com/customer/odoo:latest"}}
		e := &Engine{Runner: runner}

		_, err := e.Build(context.Background(), BuildRequest{RepoRoot: repoDir})
		require.NoError(t, err)

		assert.Equal(t, "registry.example.com/customer/odoo:latest", runner.opts.Image)
	})

	t.Run("explicit Output overrides image.name convention", func(t *testing.T) {
		repoDir := t.TempDir()
		require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, "odoo-builder.yaml"),
			[]byte("image:\n  name: registry.example.com/customer/odoo\n"), 0o644))

		runner := &fakeRunner{out: buildkit.BuildOutput{OutputPath: "/custom.tar"}}
		e := &Engine{Runner: runner}

		result, err := e.Build(context.Background(), BuildRequest{
			RepoRoot: repoDir,
			Output:   OutputSpec{Type: "oci", Path: "/custom.tar"},
		})
		require.NoError(t, err)

		assert.Equal(t, "oci", runner.opts.OutputType)
		assert.Equal(t, "/custom.tar", runner.opts.OutputPath)
		assert.Equal(t, "/custom.tar", result.ImagePath)
	})

	t.Run("docker load output via explicit Output.Type", func(t *testing.T) {
		repoDir := t.TempDir()
		require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, "odoo-builder.yaml"),
			[]byte("image:\n  name: registry.example.com/customer/odoo\n  tag: v1\n"), 0o644))

		runner := &fakeRunner{out: buildkit.BuildOutput{
			OutputPath: filepath.Join(repoDir, ".build", "image.docker.tar"),
			ImageRef:   "registry.example.com/customer/odoo:v1",
		}}
		e := &Engine{Runner: runner}

		result, err := e.Build(context.Background(), BuildRequest{
			RepoRoot: repoDir,
			Output:   OutputSpec{Type: "docker"},
		})
		require.NoError(t, err)

		buildDir := filepath.Join(repoDir, ".build")
		assert.Equal(t, "docker", runner.opts.OutputType)
		assert.Equal(t, filepath.Join(buildDir, "image.docker.tar"), runner.opts.OutputPath)
		assert.Equal(t, "registry.example.com/customer/odoo:v1", runner.opts.Image)
		assert.Equal(t, "registry.example.com/customer/odoo:v1", result.ImageRef)
		assert.Equal(t, filepath.Join(buildDir, "image.docker.tar"), result.ImagePath)
	})

	t.Run("docker load requires image.name", func(t *testing.T) {
		repoDir := t.TempDir()
		require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))

		runner := &fakeRunner{}
		e := &Engine{Runner: runner}

		_, err := e.Build(context.Background(), BuildRequest{
			RepoRoot: repoDir,
			Output:   OutputSpec{Type: "docker"},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires odoo-builder.yaml's image.name")
		assert.Equal(t, 0, runner.calls)
	})

	t.Run("malformed image.name surfaces as error before Runner is invoked", func(t *testing.T) {
		repoDir := t.TempDir()
		require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, "odoo-builder.yaml"),
			[]byte("image:\n  name: \"app:v1\"\n"), 0o644))

		runner := &fakeRunner{}
		e := &Engine{Runner: runner}

		_, err := e.Build(context.Background(), BuildRequest{RepoRoot: repoDir})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "registry:")
		assert.Equal(t, 0, runner.calls)
	})

	t.Run("validation failure short-circuits before Runner is invoked", func(t *testing.T) {
		runner := &fakeRunner{}
		e := &Engine{Runner: runner}

		_, err := e.Build(context.Background(), BuildRequest{RepoRoot: t.TempDir()})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
		assert.Equal(t, 0, runner.calls)
	})
}

func TestEngine_Build_Cache(t *testing.T) {
	t.Run("cache disabled by default", func(t *testing.T) {
		repoDir := t.TempDir()
		require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))

		runner := &fakeRunner{}
		e := &Engine{Runner: runner}

		_, err := e.Build(context.Background(), BuildRequest{RepoRoot: repoDir})
		require.NoError(t, err)

		assert.Empty(t, runner.opts.CacheDir)
		assert.Empty(t, runner.opts.CacheRef)
	})

	t.Run("cache enabled without image.name uses local cache dir", func(t *testing.T) {
		repoDir := t.TempDir()
		require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, "odoo-builder.yaml"),
			[]byte("cache:\n  enabled: true\n"), 0o644))

		runner := &fakeRunner{}
		e := &Engine{Runner: runner}

		_, err := e.Build(context.Background(), BuildRequest{RepoRoot: repoDir})
		require.NoError(t, err)

		assert.True(t, strings.HasSuffix(runner.opts.CacheDir, filepath.Join("odoo-builder", "buildkit-cache")), runner.opts.CacheDir)
		assert.Empty(t, runner.opts.CacheRef)
	})

	t.Run("cache enabled with image.name uses registry cache ref", func(t *testing.T) {
		repoDir := t.TempDir()
		require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, "odoo-builder.yaml"),
			[]byte("cache:\n  enabled: true\nimage:\n  name: registry.example.com/customer/odoo\n"), 0o644))

		runner := &fakeRunner{}
		e := &Engine{Runner: runner}

		_, err := e.Build(context.Background(), BuildRequest{RepoRoot: repoDir})
		require.NoError(t, err)

		assert.Equal(t, "registry.example.com/customer/odoo:buildcache", runner.opts.CacheRef)
		assert.Empty(t, runner.opts.CacheDir)
	})
}

func TestEngine_Build_Platforms(t *testing.T) {
	t.Run("platforms forwarded from odoo-builder.yaml", func(t *testing.T) {
		repoDir := t.TempDir()
		require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, "odoo-builder.yaml"),
			[]byte("build:\n  platform:\n    - linux/amd64\n    - linux/arm64\n"), 0o644))

		runner := &fakeRunner{}
		e := &Engine{Runner: runner}

		_, err := e.Build(context.Background(), BuildRequest{RepoRoot: repoDir})
		require.NoError(t, err)

		assert.Equal(t, []string{"linux/amd64", "linux/arm64"}, runner.opts.Platforms)
	})

	t.Run("platforms empty when unset", func(t *testing.T) {
		repoDir := t.TempDir()
		require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))

		runner := &fakeRunner{}
		e := &Engine{Runner: runner}

		_, err := e.Build(context.Background(), BuildRequest{RepoRoot: repoDir})
		require.NoError(t, err)

		assert.Empty(t, runner.opts.Platforms)
	})
}
